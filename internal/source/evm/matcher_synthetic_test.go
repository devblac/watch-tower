package evm

import (
	"math/big"
	"testing"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const usdc = "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"

func logRule(event string) config.Rule {
	return config.Rule{
		ID:     "r",
		Source: "evm_main",
		Match:  config.MatchSpec{Type: "log", Contract: usdc, Event: event},
	}
}

func TestRuleMatcher_NoABI_ERC20Transfer(t *testing.T) {
	m, err := NewRuleMatcher(logRule("Transfer(address,address,uint256)"), nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	from := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	to := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	value := new(big.Int).Mul(big.NewInt(2_000_000), big.NewInt(1_000_000))
	log := types.Log{
		Address: common.HexToAddress(usdc),
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), addrTopic(from), addrTopic(to)},
		Data:    common.LeftPadBytes(value.Bytes(), 32),
	}

	ev, ok, err := m.Match(log)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	if got := ev.Args["from"]; got != from {
		t.Errorf("from = %v, want %v", got, from)
	}
	if got := ev.Args["to"]; got != to {
		t.Errorf("to = %v, want %v", got, to)
	}
	if got, _ := ev.Args["value"].(*big.Int); got == nil || got.Cmp(value) != 0 {
		t.Errorf("value = %v, want %v", ev.Args["value"], value)
	}
}

func TestRuleMatcher_NoABI_ERC721Transfer(t *testing.T) {
	m, err := NewRuleMatcher(logRule("Transfer(address,address,uint256)"), nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	tokenID := common.BigToHash(big.NewInt(42))
	log := types.Log{
		Address: common.HexToAddress(usdc),
		Topics: []common.Hash{
			crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")),
			addrTopic(common.HexToAddress("0x01")),
			addrTopic(common.HexToAddress("0x02")),
			tokenID,
		},
	}
	ev, ok, err := m.Match(log)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	if got, _ := ev.Args["tokenId"].(*big.Int); got == nil || got.Int64() != 42 {
		t.Errorf("tokenId = %v, want 42", ev.Args["tokenId"])
	}
}

func TestRuleMatcher_NoABI_UnknownEventUsesPositionalNames(t *testing.T) {
	sig := "Swap(address,uint256,uint256)"
	m, err := NewRuleMatcher(logRule(sig), nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	data := append(common.LeftPadBytes(big.NewInt(7).Bytes(), 32), common.LeftPadBytes(big.NewInt(9).Bytes(), 32)...)
	log := types.Log{
		Address: common.HexToAddress(usdc),
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte(sig)), addrTopic(common.HexToAddress("0x03"))},
		Data:    data,
	}
	ev, ok, err := m.Match(log)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	if got := ev.Args["arg0"]; got != common.HexToAddress("0x03") {
		t.Errorf("arg0 = %v", got)
	}
	if got, _ := ev.Args["arg2"].(*big.Int); got == nil || got.Int64() != 9 {
		t.Errorf("arg2 = %v, want 9", ev.Args["arg2"])
	}
}

func TestRuleMatcher_SignatureWhitespaceIgnored(t *testing.T) {
	m, err := NewRuleMatcher(logRule("Transfer(address, address, uint256)"), nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	if want := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")); m.topic0 != want {
		t.Fatalf("topic0 = %s, want %s", m.topic0, want)
	}
}

func TestRuleMatcher_NoABI_TooManyTopics(t *testing.T) {
	sig := "Ping(uint256)"
	m, err := NewRuleMatcher(logRule(sig), nil)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	log := types.Log{
		Address: common.HexToAddress(usdc),
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte(sig)), {}, {}},
	}
	if _, _, err := m.Match(log); err == nil {
		t.Fatalf("expected error for more indexed topics than arguments")
	}
}

func TestNewRuleMatcher_InvalidSignature(t *testing.T) {
	if _, err := NewRuleMatcher(logRule("Transfer"), nil); err == nil {
		t.Fatalf("expected error for signature without parentheses")
	}
}
