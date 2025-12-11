package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Predicate evaluates whether an event args map satisfies a condition.
type Predicate func(args map[string]any) (bool, error)

// CompilePredicates parses simple expressions into executable predicates.
// Supported operators: ==, !=, >, <, in, contains.
// Examples:
//   "value > 10"
//   "sender in a,b,c"
//   "memo contains alert"
func CompilePredicates(exprs []string) ([]Predicate, error) {
	var preds []Predicate
	for _, raw := range exprs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := compile(raw)
		if err != nil {
			return nil, err
		}
		preds = append(preds, p)
	}
	return preds, nil
}

func compile(expr string) (Predicate, error) {
	if strings.Contains(expr, " in ") {
		parts := strings.SplitN(expr, " in ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid in expression: %s", expr)
		}
		field := strings.TrimSpace(parts[0])
		rawList := strings.Split(parts[1], ",")
		values := make(map[string]struct{}, len(rawList))
		for _, v := range rawList {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			values[v] = struct{}{}
		}
		return func(args map[string]any) (bool, error) {
			arg, ok := args[field]
			if !ok {
				return false, nil
			}
			s := fmt.Sprint(arg)
			_, hit := values[s]
			return hit, nil
		}, nil
	}

	if strings.Contains(expr, " contains ") {
		parts := strings.SplitN(expr, " contains ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid contains expression: %s", expr)
		}
		field := strings.TrimSpace(parts[0])
		needle := strings.TrimSpace(parts[1])
		return func(args map[string]any) (bool, error) {
			val, ok := args[field]
			if !ok {
				return false, nil
			}
			return strings.Contains(fmt.Sprint(val), needle), nil
		}, nil
	}

	var op string
	switch {
	case strings.Contains(expr, "=="):
		op = "=="
	case strings.Contains(expr, "!="):
		op = "!="
	case strings.Contains(expr, ">="):
		op = ">="
	case strings.Contains(expr, "<="):
		op = "<="
	case strings.Contains(expr, ">"):
		op = ">"
	case strings.Contains(expr, "<"):
		op = "<"
	default:
		return nil, fmt.Errorf("unsupported expression: %s", expr)
	}

	parts := strings.SplitN(expr, op, 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid expression: %s", expr)
	}
	field := strings.TrimSpace(parts[0])
	rhsRaw := strings.TrimSpace(parts[1])

	numRHS, rhsIsNum := parseNumber(rhsRaw)

	return func(args map[string]any) (bool, error) {
		val, ok := args[field]
		if !ok {
			return false, nil
		}

		if rhsIsNum {
			lhs, ok := toNumber(val)
			if !ok {
				return false, nil
			}
			switch op {
			case "==":
				return lhs == numRHS, nil
			case "!=":
				return lhs != numRHS, nil
			case ">":
				return lhs > numRHS, nil
			case "<":
				return lhs < numRHS, nil
			case ">=":
				return lhs >= numRHS, nil
			case "<=":
				return lhs <= numRHS, nil
			}
		}

		// String comparisons
		lhs := fmt.Sprint(val)
		switch op {
		case "==":
			return lhs == rhsRaw, nil
		case "!=":
			return lhs != rhsRaw, nil
		default:
			return false, nil
		}
	}, nil
}

func parseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(s, "_", "")
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func toNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case string:
		return parseNumber(n)
	default:
		return 0, false
	}
}

// TokenBucket is a simple per-rule rate limiter.
type TokenBucket struct {
	capacity float64
	rate     float64 // tokens per second

	tokens     float64
	lastUpdate time.Time
}

// NewTokenBucket creates a token bucket with capacity and refill rate.
func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
	}
}

// Allow consumes one token if available, refilling based on elapsed time.
func (b *TokenBucket) Allow(now time.Time) bool {
	if b.lastUpdate.IsZero() {
		b.lastUpdate = now
	}
	elapsed := now.Sub(b.lastUpdate).Seconds()
	if elapsed > 0 {
		b.tokens = min(b.capacity, b.tokens+elapsed*b.rate)
		b.lastUpdate = now
	}
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
