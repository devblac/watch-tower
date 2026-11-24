package engine

import (
	"math/big"
	"testing"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/source/evm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func bigFromString(t *testing.T, s string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad big int %q", s)
	}
	return v
}

func evalOne(t *testing.T, expr string, args map[string]any) bool {
	t.Helper()
	preds, err := CompilePredicates([]string{expr})
	if err != nil {
		t.Fatalf("compile %q: %v", expr, err)
	}
	ok, err := preds[0](args)
	if err != nil {
		t.Fatalf("eval %q: %v", expr, err)
	}
	return ok
}

func TestPredicates_BigIntValues(t *testing.T) {
	oneMillionUSDC := big.NewInt(1_000_000_000_000)
	tests := []struct {
		name string
		expr string
		val  any
		want bool
	}{
		{"usdc_at_threshold", "value >= 1_000_000 * 1e6", oneMillionUSDC, true},
		{"usdc_below_threshold", "value >= 1_000_000 * 1e6", big.NewInt(999_999_999_999), false},
		{"wei_helper", "value >= wei(1e18)", bigFromString(t, "1000000000000000000"), true},
		// float64 cannot tell these apart; exact comparison must.
		{"exact_beyond_float_precision", "value > 1e24", bigFromString(t, "1000000000000000000000001"), true},
		{"exact_eq_beyond_float_precision", "value == 1e24", bigFromString(t, "1000000000000000000000001"), false},
		{"uint256_max", "value > 1e77", bigFromString(t, "115792089237316195423570985008687907853269984665640564039457584007913129639935"), true},
		{"negative_int256", "value < 0", big.NewInt(-5), true},
		{"uint8", "decimals == 6", uint8(6), true},
		{"decimal_rhs", "value > 0.5", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalOne(t, tt.expr, map[string]any{"value": tt.val, "decimals": tt.val}); got != tt.want {
				t.Errorf("%q with %v = %v, want %v", tt.expr, tt.val, got, tt.want)
			}
		})
	}
}

func TestPredicates_Addresses(t *testing.T) {
	addr := common.HexToAddress("0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
	zero := common.Address{}
	tests := []struct {
		name string
		expr string
		val  any
		want bool
	}{
		{"quoted_lowercase_matches_checksummed", `sender == "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"`, addr, true},
		{"unquoted_matches", "sender == 0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", addr, true},
		{"zero_address_not_numeric", "sender != 0x0000000000000000000000000000000000000000", addr, true},
		{"zero_address_excluded", "sender != 0x0000000000000000000000000000000000000000", zero, false},
		{"in_list_mixed_case", "sender in 0x01,0xA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48", addr, true},
		{"quoted_contains", `memo contains "alert"`, "critical alert", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalOne(t, tt.expr, map[string]any{"sender": tt.val, "memo": tt.val}); got != tt.want {
				t.Errorf("%q with %v = %v, want %v", tt.expr, tt.val, got, tt.want)
			}
		})
	}
}

// TestReadmeQuickstartRule runs the README's USDC whale rule (no abi_dirs)
// through the EVM matcher and the predicates, end to end.
func TestReadmeQuickstartRule(t *testing.T) {
	const sig = "Transfer(address,address,uint256)"
	rule := config.Rule{
		ID:     "large_transfer",
		Source: "mainnet",
		Match: config.MatchSpec{
			Type:     "log",
			Contract: "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			Event:    sig,
			Where: []string{
				"value >= 1_000_000 * 1e6",
				"to != 0x0000000000000000000000000000000000000000",
			},
		},
	}
	m, err := evm.NewRuleMatcher(rule, nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	preds, err := CompilePredicates(rule.Match.Where)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	topic := func(a common.Address) common.Hash { return common.BytesToHash(common.LeftPadBytes(a.Bytes(), 32)) }
	transfer := func(to common.Address, amount int64) types.Log {
		return types.Log{
			Address: common.HexToAddress(rule.Match.Contract),
			Topics:  []common.Hash{crypto.Keccak256Hash([]byte(sig)), topic(common.HexToAddress("0x01")), topic(to)},
			Data:    common.LeftPadBytes(big.NewInt(amount).Bytes(), 32),
		}
	}

	recipient := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	tests := []struct {
		name string
		log  types.Log
		want bool
	}{
		{"whale_transfer_alerts", transfer(recipient, 2_000_000_000_000), true},
		{"small_transfer_ignored", transfer(recipient, 500_000_000), false},
		{"burn_ignored", transfer(common.Address{}, 2_000_000_000_000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, ok, err := m.Match(tt.log)
			if err != nil || !ok {
				t.Fatalf("expected log to match, ok=%v err=%v", ok, err)
			}
			got, err := allPredicates(preds, ev.Args)
			if err != nil {
				t.Fatalf("predicates: %v", err)
			}
			if got != tt.want {
				t.Errorf("alert = %v, want %v (args %v)", got, tt.want, ev.Args)
			}
		})
	}
}
