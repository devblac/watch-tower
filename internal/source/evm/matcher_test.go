package evm

import (
	"math/big"
	"strings"
	"testing"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestRuleMatcher_DecodesTransfer(t *testing.T) {
	erc20ABIJSON := `[
		{"type":"event","name":"Transfer","inputs":[
			{"name":"from","type":"address","indexed":true},
			{"name":"to","type":"address","indexed":true},
			{"name":"value","type":"uint256","indexed":false}
		]}
	]`
	a, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		t.Fatalf("parse abi: %v", err)
	}
	abis := map[string]*abi.ABI{"erc20": &a}

	rule := config.Rule{
		ID:     "usdc_whale",
		Source: "evm_main",
		Match: config.MatchSpec{
			Type:     "log",
			Contract: "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
			Event:    "Transfer(address,address,uint256)",
		},
	}

	m, err := NewRuleMatcher(rule, abis)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	value := big.NewInt(0).Mul(big.NewInt(1_000_000), big.NewInt(1_000_000))
	data := common.LeftPadBytes(value.Bytes(), 32)
	from := common.HexToAddress("0x0000000000000000000000000000000000000001")
	to := common.HexToAddress("0x0000000000000000000000000000000000000002")

	log := types.Log{
		Address:     common.HexToAddress(rule.Match.Contract),
		Topics:      []common.Hash{crypto.Keccak256Hash([]byte(rule.Match.Event)), addrTopic(from), addrTopic(to)},
		Data:        data,
		TxHash:      common.HexToHash("0xabc"),
		BlockNumber: 100,
		Index:       3,
	}

	ev, ok, err := m.Match(log)
	if err != nil {
		t.Fatalf("match error: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if ev.Name != "Transfer" {
		t.Fatalf("unexpected name: %s", ev.Name)
	}
	if got := ev.Args["value"].(*big.Int); got.Cmp(value) != 0 {
		t.Fatalf("unexpected value %s", got)
	}
}
