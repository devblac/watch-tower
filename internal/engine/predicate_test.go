package engine

import (
	"testing"
	"time"
)

func TestCompilePredicates_NumericComparisons(t *testing.T) {
	preds, err := CompilePredicates([]string{"value > 10", "value < 20"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	args := map[string]any{"value": 15}
	for _, p := range preds {
		ok, err := p(args)
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if !ok {
			t.Fatalf("expected predicate to pass")
		}
	}
}

func TestCompilePredicates_InAndContains(t *testing.T) {
	preds, err := CompilePredicates([]string{"sender in a,b,c", "memo contains alert"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	args := map[string]any{"sender": "b", "memo": "critical alert raised"}
	for _, p := range preds {
		ok, err := p(args)
		if err != nil {
			t.Fatalf("eval: %v", err)
		}
		if !ok {
			t.Fatalf("expected predicate to pass")
		}
	}
}

func TestCompilePredicates_StringEquality(t *testing.T) {
	preds, err := CompilePredicates([]string{"status == ok"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	args := map[string]any{"status": "ok"}
	ok, err := preds[0](args)
	if err != nil || !ok {
		t.Fatalf("expected true, got %v err=%v", ok, err)
	}
}

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(2, 1) // capacity=2, 1 token/sec
	now := time.Now()

	if !tb.Allow(now) || !tb.Allow(now) {
		t.Fatalf("expected initial tokens available")
	}
	if tb.Allow(now) {
		t.Fatalf("expected third to be rate-limited")
	}

	// Refill after 1.5s -> should allow one
	now = now.Add(1500 * time.Millisecond)
	if !tb.Allow(now) {
		t.Fatalf("expected token after refill")
	}
}
