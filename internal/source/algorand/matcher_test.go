package algorand

import (
	"encoding/base64"
	"testing"

	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/devblac/watch-tower/internal/config"
)

func TestMatcher_AppCall(t *testing.T) {
	rule := config.Rule{
		ID:     "app",
		Source: "algo",
		Match: config.MatchSpec{
			Type:  "app_call",
			AppID: 123,
		},
	}
	m, err := NewRuleMatcher(rule)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	tx := sdk.Transaction{
		Type: sdk.ApplicationCallTx,
		Header: sdk.Header{
			Sender: addr("SENDER0000000000000000000000000000000000000000000000000000"),
		},
		ApplicationFields: sdk.ApplicationFields{
			ApplicationCallTxnFields: sdk.ApplicationCallTxnFields{
				ApplicationID:   123,
				OnCompletion:    sdk.NoOpOC,
				ApplicationArgs: [][]byte{[]byte("hello")},
				Accounts:        []sdk.Address{addr("ACCOUNT000000000000000000000000000000000000000000000000")},
			},
		},
	}

	ev, ok, err := m.MatchTxn(tx, sdk.ApplyData{})
	if err != nil {
		t.Fatalf("match txn: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if ev.AppID != 0 && ev.AppID != 123 {
		t.Fatalf("unexpected app_id %d", ev.AppID)
	}
	args, ok := ev.Args["application_args"].([]string)
	if !ok || len(args) != 1 || args[0] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("args not encoded")
	}
}

func TestMatcher_AssetTransfer(t *testing.T) {
	rule := config.Rule{
		ID:     "asa",
		Source: "algo",
		Match: config.MatchSpec{
			Type: "asset_transfer",
		},
	}
	m, err := NewRuleMatcher(rule)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	tx := sdk.Transaction{
		Type: sdk.AssetTransferTx,
		Header: sdk.Header{
			Sender: addr("SENDER0000000000000000000000000000000000000000000000000000"),
		},
		AssetTransferTxnFields: sdk.AssetTransferTxnFields{
			XferAsset:     999,
			AssetAmount:   42,
			AssetSender:   addr("SENDER0000000000000000000000000000000000000000000000000000"),
			AssetReceiver: addr("RECEIVER000000000000000000000000000000000000000000000000"),
		},
	}

	ev, ok, err := m.MatchTxn(tx, sdk.ApplyData{})
	if err != nil {
		t.Fatalf("match txn: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if ev.Args["asset_id"] != uint64(999) {
		t.Fatalf("asset_id mismatch")
	}
}

func addr(bech string) sdk.Address {
	var a sdk.Address
	copy(a[:], []byte(bech)[:])
	return a
}
