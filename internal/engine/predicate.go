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
//
//	"value > 10"
//	"sender in a,b,c"
//	"memo contains alert"
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

	numRHS, rhsIsNum := evaluateNumber(rhsRaw)

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

// evaluateNumber evaluates a numeric expression, supporting:
// - Simple numbers: "100", "1e6", "1_000_000"
// - Helper functions: "wei(1e18)", "microAlgos(1e6)"
// - Multiplication: "1_000_000 * 1e6"
func evaluateNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "_", "")

	// Handle multiplication
	if strings.Contains(s, "*") {
		parts := strings.Split(s, "*")
		if len(parts) != 2 {
			return 0, false
		}
		a, ok1 := evaluateNumber(strings.TrimSpace(parts[0]))
		b, ok2 := evaluateNumber(strings.TrimSpace(parts[1]))
		if !ok1 || !ok2 {
			return 0, false
		}
		return a * b, true
	}

	// Check for helper functions: wei(value) or microAlgos(value)
	if strings.HasPrefix(s, "wei(") && strings.HasSuffix(s, ")") {
		inner := strings.TrimSpace(s[4 : len(s)-1])
		v, ok := evaluateNumber(inner)
		if !ok {
			return 0, false
		}
		return v, true // wei is already the base unit, no conversion needed
	}
	if strings.HasPrefix(s, "microAlgos(") && strings.HasSuffix(s, ")") {
		inner := strings.TrimSpace(s[11 : len(s)-1])
		v, ok := evaluateNumber(inner)
		if !ok {
			return 0, false
		}
		return v, true // microAlgos is already the base unit, no conversion needed
	}

	// Parse as a simple number
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// parseNumber is a simple wrapper for backward compatibility.
func parseNumber(s string) (float64, bool) {
	return evaluateNumber(s)
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
