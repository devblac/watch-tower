package evm

import (
	"fmt"
	"strings"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// RuleMatcher filters and decodes logs for a given rule.
type RuleMatcher struct {
	rule    config.Rule
	address common.Address
	topic0  common.Hash
	event   *abi.Event
	// synthetic is set when no ABI was found and the event was built from the
	// signature alone; indexed args and names are then resolved per log.
	synthetic bool
}

// NewRuleMatcher builds a matcher for a log rule using available ABIs. Supports only log rules.
func NewRuleMatcher(rule config.Rule, abis map[string]*abi.ABI) (*RuleMatcher, error) {
	if strings.ToLower(rule.Match.Type) != "log" {
		return nil, fmt.Errorf("rule %s: match.type %s unsupported in evm matcher", rule.ID, rule.Match.Type)
	}
	if rule.Match.Contract == "" || rule.Match.Event == "" {
		return nil, fmt.Errorf("rule %s: contract and event are required", rule.ID)
	}

	signature := canonicalSignature(rule.Match.Event)
	evName := eventName(signature)
	var ev *abi.Event
	isSynthetic := false
	if found, ok := FindEvent(abis, evName); ok {
		ev = found
	} else {
		synthetic, err := syntheticEvent(signature)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rule.ID, err)
		}
		ev = synthetic
		isSynthetic = true
	}

	topic := crypto.Keccak256Hash([]byte(signature))

	return &RuleMatcher{
		rule:      rule,
		address:   common.HexToAddress(rule.Match.Contract),
		topic0:    topic,
		event:     ev,
		synthetic: isSynthetic,
	}, nil
}

// Match checks the log against the matcher; returns a normalized event on success.
func (m *RuleMatcher) Match(log types.Log) (*NormalizedEvent, bool, error) {
	if log.Address != m.address {
		return nil, false, nil
	}
	if len(log.Topics) == 0 || log.Topics[0] != m.topic0 {
		return nil, false, nil
	}

	args := map[string]any{}
	if m.event != nil {
		inputs := m.event.Inputs
		if m.synthetic {
			resolved, err := resolveSyntheticInputs(m.event, len(log.Topics)-1)
			if err != nil {
				return nil, false, err
			}
			inputs = resolved
		}
		indexed, nonIndexed := splitIndexed(inputs)
		if err := abi.ParseTopicsIntoMap(args, indexed, log.Topics[1:]); err != nil {
			return nil, false, fmt.Errorf("parse topics: %w", err)
		}
		if err := nonIndexed.UnpackIntoMap(args, log.Data); err != nil {
			return nil, false, fmt.Errorf("unpack data: %w", err)
		}
	}

	idx := uint(log.Index)
	return &NormalizedEvent{
		RuleID:   m.rule.ID,
		Contract: log.Address.Hex(),
		Name:     eventName(m.rule.Match.Event),
		TxHash:   log.TxHash.Hex(),
		LogIndex: &idx,
		Args:     args,
	}, true, nil
}

func eventName(signature string) string {
	if i := strings.Index(signature, "("); i > 0 {
		return signature[:i]
	}
	return signature
}

// canonicalSignature strips whitespace so "Transfer(address, address, uint256)"
// hashes to the same topic as "Transfer(address,address,uint256)".
func canonicalSignature(signature string) string {
	return strings.Join(strings.Fields(signature), "")
}

// syntheticEvent builds a minimal ABI Event from a signature like Transfer(address,address,uint256).
// Which arguments are indexed is not known from the signature; resolveSyntheticInputs
// decides that per log from the number of topics.
func syntheticEvent(signature string) (*abi.Event, error) {
	l := strings.Index(signature, "(")
	r := strings.LastIndex(signature, ")")
	if l <= 0 || r <= l {
		return nil, fmt.Errorf("invalid event signature: %s", signature)
	}
	name := signature[:l]
	rawArgs := strings.Split(signature[l+1:r], ",")
	args := make(abi.Arguments, 0, len(rawArgs))
	for _, a := range rawArgs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		t, err := abi.NewType(a, "", nil)
		if err != nil {
			return nil, fmt.Errorf("parse type %s: %w", a, err)
		}
		args = append(args, abi.Argument{Type: t})
	}
	return &abi.Event{
		Name:      name,
		RawName:   name,
		Inputs:    args,
		Anonymous: false,
		Sig:       signature,
	}, nil
}

// wellKnownArgNames gives argument names for common token events, keyed by
// canonical signature and then by number of indexed arguments. ERC-20 and
// ERC-721 share the Transfer/Approval signatures and differ only in indexing.
var wellKnownArgNames = map[string]map[int][]string{
	"Transfer(address,address,uint256)": {
		2: {"from", "to", "value"},   // ERC-20
		3: {"from", "to", "tokenId"}, // ERC-721
	},
	"Approval(address,address,uint256)": {
		2: {"owner", "spender", "value"},
		3: {"owner", "approved", "tokenId"},
	},
	"ApprovalForAll(address,address,bool)": {
		2: {"owner", "operator", "approved"},
	},
	"TransferSingle(address,address,address,uint256,uint256)": {
		3: {"operator", "from", "to", "id", "value"},
	},
}

// resolveSyntheticInputs marks the first indexedCount arguments as indexed
// (the usual Solidity layout) and names every argument, using well-known names
// when available and arg0, arg1, ... otherwise. Events whose indexed
// arguments are not leading need an ABI file in abi_dirs.
func resolveSyntheticInputs(ev *abi.Event, indexedCount int) (abi.Arguments, error) {
	if indexedCount < 0 || indexedCount > len(ev.Inputs) {
		return nil, fmt.Errorf("event %s: log has %d indexed topics but signature has %d arguments", ev.Sig, indexedCount, len(ev.Inputs))
	}
	names := wellKnownArgNames[ev.Sig][indexedCount]
	out := make(abi.Arguments, len(ev.Inputs))
	for i, in := range ev.Inputs {
		in.Indexed = i < indexedCount
		if len(names) == len(ev.Inputs) {
			in.Name = names[i]
		} else {
			in.Name = fmt.Sprintf("arg%d", i)
		}
		out[i] = in
	}
	return out, nil
}

func splitIndexed(args abi.Arguments) (indexed abi.Arguments, nonIndexed abi.Arguments) {
	for _, a := range args {
		if a.Indexed {
			indexed = append(indexed, a)
		} else {
			nonIndexed = append(nonIndexed, a)
		}
	}
	return indexed, nonIndexed
}
