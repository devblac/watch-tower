package engine

import (
	"fmt"
	"math/big"
	"regexp"
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
			v = normalizeString(unquote(strings.TrimSpace(v)))
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
			_, hit := values[normalizeString(fmt.Sprint(arg))]
			return hit, nil
		}, nil
	}

	if strings.Contains(expr, " contains ") {
		parts := strings.SplitN(expr, " contains ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid contains expression: %s", expr)
		}
		field := strings.TrimSpace(parts[0])
		needle := unquote(strings.TrimSpace(parts[1]))
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
	strRHS := normalizeString(unquote(rhsRaw))

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
			c := lhs.Cmp(numRHS)
			switch op {
			case "==":
				return c == 0, nil
			case "!=":
				return c != 0, nil
			case ">":
				return c > 0, nil
			case "<":
				return c < 0, nil
			case ">=":
				return c >= 0, nil
			case "<=":
				return c <= 0, nil
			}
		}

		// String comparisons
		lhs := normalizeString(fmt.Sprint(val))
		switch op {
		case "==":
			return lhs == strRHS, nil
		case "!=":
			return lhs != strRHS, nil
		default:
			return false, nil
		}
	}, nil
}

// decimalRe matches plain decimal literals (optionally with an exponent).
// Hex literals such as addresses are deliberately not numbers.
var decimalRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// evaluateNumber evaluates a numeric expression exactly, supporting:
// - Simple numbers: "100", "1e6", "1_000_000", "0.5"
// - Helper functions: "wei(1e18)", "microAlgos(1e6)"
// - Multiplication: "1_000_000 * 1e6"
func evaluateNumber(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "_", "")

	// Handle multiplication
	if strings.Contains(s, "*") {
		result := big.NewRat(1, 1)
		for _, part := range strings.Split(s, "*") {
			v, ok := evaluateNumber(part)
			if !ok {
				return nil, false
			}
			result.Mul(result, v)
		}
		return result, true
	}

	// Helper functions: wei(value) and microAlgos(value) are already base units.
	for _, helper := range []string{"wei(", "microAlgos("} {
		if strings.HasPrefix(s, helper) && strings.HasSuffix(s, ")") {
			return evaluateNumber(s[len(helper) : len(s)-1])
		}
	}

	if !decimalRe.MatchString(s) {
		return nil, false
	}
	v, ok := new(big.Rat).SetString(s)
	return v, ok
}

// toNumber converts a decoded event argument into an exact rational number.
// EVM uint256/int256 values arrive as *big.Int; Algorand amounts as uint64.
func toNumber(v any) (*big.Rat, bool) {
	switch n := v.(type) {
	case *big.Int:
		if n == nil {
			return nil, false
		}
		return new(big.Rat).SetInt(n), true
	case big.Int:
		return new(big.Rat).SetInt(&n), true
	case int:
		return new(big.Rat).SetInt64(int64(n)), true
	case int8:
		return new(big.Rat).SetInt64(int64(n)), true
	case int16:
		return new(big.Rat).SetInt64(int64(n)), true
	case int32:
		return new(big.Rat).SetInt64(int64(n)), true
	case int64:
		return new(big.Rat).SetInt64(n), true
	case uint:
		return new(big.Rat).SetUint64(uint64(n)), true
	case uint8:
		return new(big.Rat).SetUint64(uint64(n)), true
	case uint16:
		return new(big.Rat).SetUint64(uint64(n)), true
	case uint32:
		return new(big.Rat).SetUint64(uint64(n)), true
	case uint64:
		return new(big.Rat).SetUint64(n), true
	case float64:
		r := new(big.Rat).SetFloat64(n)
		return r, r != nil
	case float32:
		r := new(big.Rat).SetFloat64(float64(n))
		return r, r != nil
	case string:
		return evaluateNumber(n)
	default:
		return nil, false
	}
}

// unquote strips one pair of matching surrounding quotes, so that
// `sender == "0xabc"` compares against 0xabc.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// normalizeString lowercases hex strings (addresses, hashes) so that
// checksummed and lowercase forms compare equal. Other strings are unchanged.
func normalizeString(s string) string {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strings.ToLower(s)
	}
	return s
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
