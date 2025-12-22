package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite-backed persistence for cursors, alerts, sends, and dedupe.
type Store struct {
	db *sql.DB
}

// Open initializes a SQLite database and runs minimal schema setup.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ping checks database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialized")
	}
	return s.db.PingContext(ctx)
}

func configure(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("set pragma %q: %w", p, err)
		}
	}
	return nil
}

func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schema := `
CREATE TABLE IF NOT EXISTS cursors (
  source_id   TEXT PRIMARY KEY,
  height      INTEGER NOT NULL,
  hash        TEXT NOT NULL,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alerts (
  id            TEXT PRIMARY KEY,
  rule_id       TEXT NOT NULL,
  fingerprint   TEXT,
  txhash        TEXT,
  payload_json  TEXT,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sends (
  alert_id      TEXT NOT NULL,
  sink_id       TEXT NOT NULL,
  status        TEXT NOT NULL,
  response_code INTEGER,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(alert_id, sink_id)
);

CREATE TABLE IF NOT EXISTS dedupe (
  key         TEXT PRIMARY KEY,
  expires_at  TIMESTAMP NOT NULL
);
`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// UpsertCursor records the latest processed height/hash for a source.
func (s *Store) UpsertCursor(ctx context.Context, sourceID string, height uint64, hash string) error {
	if sourceID == "" {
		return errors.New("sourceID required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO cursors (source_id, height, hash, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(source_id) DO UPDATE SET
  height=excluded.height,
  hash=excluded.hash,
  updated_at=CURRENT_TIMESTAMP;
`, sourceID, height, hash)
	if err != nil {
		return fmt.Errorf("upsert cursor: %w", err)
	}
	return nil
}

// GetCursor retrieves the cursor for a source.
func (s *Store) GetCursor(ctx context.Context, sourceID string) (height uint64, hash string, ok bool, err error) {
	row := s.db.QueryRowContext(ctx, `
SELECT height, hash FROM cursors WHERE source_id = ?;
`, sourceID)
	switch err = row.Scan(&height, &hash); err {
	case nil:
		return height, hash, true, nil
	case sql.ErrNoRows:
		return 0, "", false, nil
	default:
		return 0, "", false, fmt.Errorf("get cursor: %w", err)
	}
}

// MarkDedupe sets or refreshes a dedupe key until expiresAt.
func (s *Store) MarkDedupe(ctx context.Context, key string, expiresAt time.Time) error {
	if key == "" {
		return errors.New("key required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO dedupe (key, expires_at)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET expires_at=excluded.expires_at;
`, key, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("mark dedupe: %w", err)
	}
	return nil
}

// IsDuplicate returns true if the key exists and is not expired; expired entries are pruned.
func (s *Store) IsDuplicate(ctx context.Context, key string, now time.Time) (bool, error) {
	if key == "" {
		return false, errors.New("key required")
	}

	var expires time.Time
	err := s.db.QueryRowContext(ctx, `
SELECT expires_at FROM dedupe WHERE key = ?;
`, key).Scan(&expires)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check dedupe: %w", err)
	}

	if expires.After(now.UTC()) {
		return true, nil
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM dedupe WHERE key = ?;`, key); err != nil {
		return false, fmt.Errorf("prune dedupe: %w", err)
	}
	return false, nil
}

// Alert represents an emitted alert record.
type Alert struct {
	ID          string
	RuleID      string
	Fingerprint string
	TxHash      string
	PayloadJSON string
	CreatedAt   time.Time
}

// InsertAlert stores an alert; primary key enforces exactly-once insertion.
func (s *Store) InsertAlert(ctx context.Context, a Alert) error {
	if a.ID == "" || a.RuleID == "" {
		return errors.New("alert id and rule_id required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO alerts (id, rule_id, fingerprint, txhash, payload_json, created_at)
VALUES (?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP));
`, a.ID, a.RuleID, a.Fingerprint, a.TxHash, a.PayloadJSON, nullTime(a.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}
	return nil
}

// Send represents a sink delivery record.
type Send struct {
	AlertID      string
	SinkID       string
	Status       string
	ResponseCode int
	CreatedAt    time.Time
}

// InsertSend records a sink delivery attempt; primary key enforces exactly-once per alert/sink.
func (s *Store) InsertSend(ctx context.Context, srec Send) error {
	if srec.AlertID == "" || srec.SinkID == "" || srec.Status == "" {
		return errors.New("alert_id, sink_id, and status are required")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sends (alert_id, sink_id, status, response_code, created_at)
VALUES (?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP));
`, srec.AlertID, srec.SinkID, srec.Status, srec.ResponseCode, nullTime(srec.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert send: %w", err)
	}
	return nil
}

// WithTx executes a callback inside a transaction for callers needing atomicity.
func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}
