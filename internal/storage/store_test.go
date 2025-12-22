package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestCursorUpsertAndGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertCursor(ctx, "src1", 10, "hashA"); err != nil {
		t.Fatalf("upsert cursor: %v", err)
	}
	h, hash, ok, err := store.GetCursor(ctx, "src1")
	if err != nil || !ok {
		t.Fatalf("get cursor failed err=%v ok=%v", err, ok)
	}
	if h != 10 || hash != "hashA" {
		t.Fatalf("unexpected cursor: %d %s", h, hash)
	}

	if err := store.UpsertCursor(ctx, "src1", 20, "hashB"); err != nil {
		t.Fatalf("upsert cursor update: %v", err)
	}
	h, hash, ok, err = store.GetCursor(ctx, "src1")
	if err != nil || !ok || h != 20 || hash != "hashB" {
		t.Fatalf("cursor not updated: %d %s err=%v ok=%v", h, hash, err, ok)
	}
}

func TestDedupeTTL(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := store.MarkDedupe(ctx, "k1", now.Add(1*time.Second)); err != nil {
		t.Fatalf("mark dedupe: %v", err)
	}
	dup, err := store.IsDuplicate(ctx, "k1", now)
	if err != nil {
		t.Fatalf("is duplicate: %v", err)
	}
	if !dup {
		t.Fatalf("expected duplicate before expiry")
	}

	later := now.Add(2 * time.Second)
	dup, err = store.IsDuplicate(ctx, "k1", later)
	if err != nil {
		t.Fatalf("is duplicate later: %v", err)
	}
	if dup {
		t.Fatalf("expected non-duplicate after expiry")
	}
}

func TestExactlyOnceAlert(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	alert := Alert{
		ID:          "a1",
		RuleID:      "r1",
		Fingerprint: "fp",
		TxHash:      "0xabc",
		PayloadJSON: `{"x":1}`,
		CreatedAt:   time.Now(),
	}

	if err := store.InsertAlert(ctx, alert); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	if err := store.InsertAlert(ctx, alert); err == nil {
		t.Fatalf("expected duplicate alert insert to fail")
	}
}

func TestPing(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	store.Close()
	if err := store.Ping(ctx); err == nil {
		t.Fatalf("expected ping to fail after close")
	}
}
