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

func TestCompilePredicates_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		args      map[string]any
		want      bool
		wantError bool
	}{
		// Numeric comparisons
		{"numeric_eq", "value == 10", map[string]any{"value": 10}, true, false},
		{"numeric_eq_fail", "value == 10", map[string]any{"value": 20}, false, false},
		{"numeric_ne", "value != 10", map[string]any{"value": 20}, true, false},
		{"numeric_ne_fail", "value != 10", map[string]any{"value": 10}, false, false},
		{"numeric_gt", "value > 10", map[string]any{"value": 15}, true, false},
		{"numeric_gt_fail", "value > 10", map[string]any{"value": 5}, false, false},
		{"numeric_lt", "value < 10", map[string]any{"value": 5}, true, false},
		{"numeric_lt_fail", "value < 10", map[string]any{"value": 15}, false, false},
		{"numeric_gte", "value >= 10", map[string]any{"value": 10}, true, false},
		{"numeric_gte_above", "value >= 10", map[string]any{"value": 15}, true, false},
		{"numeric_gte_fail", "value >= 10", map[string]any{"value": 5}, false, false},
		{"numeric_lte", "value <= 10", map[string]any{"value": 10}, true, false},
		{"numeric_lte_below", "value <= 10", map[string]any{"value": 5}, true, false},
		{"numeric_lte_fail", "value <= 10", map[string]any{"value": 15}, false, false},

		// String comparisons
		{"string_eq", "status == ok", map[string]any{"status": "ok"}, true, false},
		{"string_eq_fail", "status == ok", map[string]any{"status": "fail"}, false, false},
		{"string_ne", "status != ok", map[string]any{"status": "fail"}, true, false},
		{"string_ne_fail", "status != ok", map[string]any{"status": "ok"}, false, false},

		// In operator
		{"in_match", "sender in a,b,c", map[string]any{"sender": "b"}, true, false},
		{"in_match_first", "sender in a,b,c", map[string]any{"sender": "a"}, true, false},
		{"in_match_last", "sender in a,b,c", map[string]any{"sender": "c"}, true, false},
		{"in_no_match", "sender in a,b,c", map[string]any{"sender": "d"}, false, false},
		{"in_missing_field", "sender in a,b,c", map[string]any{"other": "a"}, false, false},

		// Contains operator
		{"contains_match", "memo contains alert", map[string]any{"memo": "critical alert raised"}, true, false},
		{"contains_no_match", "memo contains alert", map[string]any{"memo": "normal message"}, false, false},
		{"contains_missing_field", "memo contains alert", map[string]any{"other": "alert"}, false, false},

		// Numeric helpers and expressions
		{"wei_helper", "value >= wei(1000)", map[string]any{"value": 1000}, true, false},
		{"wei_helper_fail", "value >= wei(1000)", map[string]any{"value": 500}, false, false},
		{"microAlgos_helper", "amount >= microAlgos(1000000)", map[string]any{"amount": 1000000}, true, false},
		{"multiplication", "value >= 1_000_000 * 1e6", map[string]any{"value": 1e12}, true, false},
		{"multiplication_fail", "value >= 1_000_000 * 1e6", map[string]any{"value": 1e11}, false, false},
		{"scientific_notation", "value >= 1e6", map[string]any{"value": 1e6}, true, false},
		{"underscore_separators", "value >= 1_000_000", map[string]any{"value": 1000000}, true, false},

		// Type conversions
		{"int64_value", "value > 10", map[string]any{"value": int64(15)}, true, false},
		{"uint64_value", "value > 10", map[string]any{"value": uint64(15)}, true, false},
		{"float64_value", "value > 10", map[string]any{"value": 15.5}, true, false},
		{"string_number", "value > 10", map[string]any{"value": "15"}, true, false},

		// Missing fields
		{"missing_field_numeric", "value > 10", map[string]any{"other": 15}, false, false},
		{"missing_field_string", "status == ok", map[string]any{"other": "ok"}, false, false},

		// Invalid operators (should error on compile)
		{"invalid_op", "value ** 2", map[string]any{"value": 4}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preds, err := CompilePredicates([]string{tt.expr})
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected compile error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			if len(preds) != 1 {
				t.Fatalf("expected 1 predicate, got %d", len(preds))
			}

			got, err := preds[0](tt.args)
			if err != nil {
				t.Fatalf("unexpected eval error: %v", err)
			}
			if got != tt.want {
				t.Errorf("predicate(%q) with args %v = %v, want %v", tt.expr, tt.args, got, tt.want)
			}
		})
	}
}

func TestCompilePredicates_MultiplePredicates(t *testing.T) {
	tests := []struct {
		name  string
		exprs []string
		args  map[string]any
		want  bool
	}{
		{"all_pass", []string{"value > 10", "value < 20", "status == ok"}, map[string]any{"value": 15, "status": "ok"}, true},
		{"first_fails", []string{"value > 10", "value < 20"}, map[string]any{"value": 5}, false},
		{"second_fails", []string{"value > 10", "value < 20"}, map[string]any{"value": 25}, false},
		{"mixed_types", []string{"value > 10", "sender in a,b,c", "memo contains test"}, map[string]any{"value": 15, "sender": "b", "memo": "test message"}, true},
		{"empty_exprs", []string{}, map[string]any{"value": 15}, true},
		{"whitespace_exprs", []string{"  ", "value > 10", ""}, map[string]any{"value": 15}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preds, err := CompilePredicates(tt.exprs)
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}

			got := true
			for _, p := range preds {
				ok, err := p(tt.args)
				if err != nil {
					t.Fatalf("unexpected eval error: %v", err)
				}
				if !ok {
					got = false
					break
				}
			}

			if got != tt.want {
				t.Errorf("predicates %v with args %v = %v, want %v", tt.exprs, tt.args, got, tt.want)
			}
		})
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

func TestTokenBucket_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		capacity float64
		rate     float64
		actions  []struct {
			elapsed time.Duration
			want    bool
		}
	}{
		{
			name:     "basic_rate_limit",
			capacity: 2,
			rate:     1,
			actions: []struct {
				elapsed time.Duration
				want    bool
			}{
				{0, true},                       // first token
				{0, true},                       // second token
				{0, false},                      // rate limited
				{1500 * time.Millisecond, true}, // refilled ~1.5 tokens, consume 1 (0.5 remaining)
				{500 * time.Millisecond, true},  // refilled 0.5 tokens (now 1.0 total), can consume
				{0, false},                      // rate limited again
				{1000 * time.Millisecond, true}, // refilled 1 token
			},
		},
		{
			name:     "high_capacity",
			capacity: 10,
			rate:     2,
			actions: []struct {
				elapsed time.Duration
				want    bool
			}{
				{0, true},               // consume 1
				{0, true},               // consume 2
				{0, true},               // consume 3
				{1 * time.Second, true}, // refilled 2 tokens (now at 9)
				{1 * time.Second, true}, // refilled 2 more (now at 10, capped)
			},
		},
		{
			name:     "slow_refill",
			capacity: 1,
			rate:     0.5, // 1 token per 2 seconds
			actions: []struct {
				elapsed time.Duration
				want    bool
			}{
				{0, true},                // consume initial token
				{0, false},               // rate limited
				{1 * time.Second, false}, // only 0.5 tokens refilled
				{2 * time.Second, true},  // 1 token refilled
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := NewTokenBucket(tt.capacity, tt.rate)
			now := time.Now()

			for i, action := range tt.actions {
				if i > 0 {
					now = now.Add(action.elapsed)
				}
				got := tb.Allow(now)
				if got != action.want {
					t.Errorf("action %d: Allow() = %v, want %v (elapsed: %v)", i, got, action.want, action.elapsed)
				}
			}
		})
	}
}
