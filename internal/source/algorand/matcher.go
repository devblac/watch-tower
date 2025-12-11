package algorand

import (
	"encoding/base64"
	"fmt"
	"strings"

	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/devblac/watch-tower/internal/config"
)

// RuleMatcher filters Algorand transactions for a given rule.
type RuleMatcher struct {
	rule  config.Rule
	appID uint64
	kind  string
}

// NewRuleMatcher builds a matcher for Algorand rules.
func NewRuleMatcher(rule config.Rule) (*RuleMatcher, error) {
	mt := strings.ToLower(rule.Match.Type)
	switch mt {
	case "app_call":
		if rule.Match.AppID == 0 {
			return nil, fmt.Errorf("rule %s: match.app_id required for app_call", rule.ID)
		}
		return &RuleMatcher{rule: rule, appID: rule.Match.AppID, kind: "app_call"}, nil
	case "asset_transfer":
		return &RuleMatcher{rule: rule, kind: "asset_transfer"}, nil
	default:
		return nil, fmt.Errorf("rule %s: unsupported match.type %s for algorand", rule.ID, rule.Match.Type)
	}
}

// MatchTxn inspects a transaction and returns a normalized event when matched.
func (m *RuleMatcher) MatchTxn(tx sdk.Transaction, apply sdk.ApplyData) (*NormalizedEvent, bool, error) {
	switch m.kind {
	case "app_call":
		if tx.Type != sdk.ApplicationCallTx {
			return nil, false, nil
		}
		if uint64(tx.ApplicationID) != m.appID {
			return nil, false, nil
		}
		args := map[string]any{
			"sender":           tx.Sender.String(),
			"on_completion":    tx.OnCompletion,
			"app_id":           uint64(tx.ApplicationID),
			"foreign_apps":     toAppUint64s(tx.ForeignApps),
			"foreign_assets":   toAssetUint64s(tx.ForeignAssets),
			"accounts":         toStrings(tx.Accounts),
			"application_args": encodeArgs(tx.ApplicationArgs),
		}
		if apply.ApplicationID != 0 {
			args["inner_app_id"] = apply.ApplicationID
		}
		return &NormalizedEvent{
			RuleID: m.rule.ID,
			Name:   "app_call",
			AppID:  uint64(tx.ApplicationID),
			Args:   args,
		}, true, nil

	case "asset_transfer":
		if tx.Type != sdk.AssetTransferTx {
			return nil, false, nil
		}
		args := map[string]any{
			"asset_id":       uint64(tx.XferAsset),
			"amount":         tx.AssetAmount,
			"sender":         tx.Sender.String(),
			"asset_sender":   tx.AssetSender.String(),
			"receiver":       tx.AssetReceiver.String(),
			"close_to":       tx.AssetCloseTo.String(),
			"close_amount":   apply.AssetClosingAmount,
			"closing_reward": apply.CloseRewards,
		}
		return &NormalizedEvent{
			RuleID: m.rule.ID,
			Name:   "asset_transfer",
			Args:   args,
		}, true, nil
	default:
		return nil, false, nil
	}
}

func toAssetUint64s(in []sdk.AssetIndex) []uint64 {
	out := make([]uint64, 0, len(in))
	for _, v := range in {
		out = append(out, uint64(v))
	}
	return out
}

func toAppUint64s(in []sdk.AppIndex) []uint64 {
	out := make([]uint64, 0, len(in))
	for _, v := range in {
		out = append(out, uint64(v))
	}
	return out
}

func toStrings(addrs []sdk.Address) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}

func encodeArgs(args [][]byte) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, base64.StdEncoding.EncodeToString(a))
	}
	return out
}
