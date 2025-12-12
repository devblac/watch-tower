package engine

import (
	"context"
	"testing"
	"time"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/sink"
	"github.com/devblac/watch-tower/internal/storage"
)

type fakeSink struct {
	count int
}

func (f *fakeSink) Send(ctx context.Context, payload sink.EventPayload) error {
	f.count++
	return nil
}

// Simple integration: ensure predicates + dedupe + dry-run behave.
func TestRunnerPredicatesAndDryRun(t *testing.T) {
	store := newTestStore(t)
	rule := config.Rule{
		ID:    "r1",
		Match: config.MatchSpec{Where: []string{"value > 10"}},
		Sinks: []string{"s1"},
		Dedupe: &config.Dedupe{
			Key: "txhash",
			TTL: "1h",
		},
	}
	cfg := &config.Config{Rules: []config.Rule{rule}}
	s := &fakeSink{}
	runner, err := NewRunner(store, cfg, nil, nil, map[string]sink.Sender{"s1": s}, true, 0, 0)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	runner.nowFunc = func() time.Time { return time.Now() }

	evs := []Event{{
		RuleID: "r1",
		TxHash: "0x1",
		Args:   map[string]any{"value": 20},
	}}
	if err := runner.handleEvents(context.Background(), evs); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if s.count != 0 { // dry-run should skip sends
		t.Fatalf("expected no sends in dry-run, got %d", s.count)
	}

	// now run non-dry and ensure dedupe prevents duplicate
	runner.dryRun = false
	if err := runner.handleEvents(context.Background(), evs); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if s.count != 1 {
		t.Fatalf("expected 1 send, got %d", s.count)
	}
	if err := runner.handleEvents(context.Background(), evs); err != nil {
		t.Fatalf("handle dup: %v", err)
	}
	if s.count != 1 {
		t.Fatalf("expected dedupe to skip duplicate send")
	}
}

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(t.TempDir() + "/db.sqlite")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
