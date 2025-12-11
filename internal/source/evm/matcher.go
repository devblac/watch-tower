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
}

// NewRuleMatcher builds a matcher for a log rule using available ABIs. Supports only log rules.
func NewRuleMatcher(rule config.Rule, abis map[string]*abi.ABI) (*RuleMatcher, error) {
	if strings.ToLower(rule.Match.Type) != "log" {
		return nil, fmt.Errorf("rule %s: match.type %s unsupported in evm matcher", rule.ID, rule.Match.Type)
	}
	if rule.Match.Contract == "" || rule.Match.Event == "" {
		return nil, fmt.Errorf("rule %s: contract and event are required", rule.ID)
	}

	evName := eventName(rule.Match.Event)
	var ev *abi.Event
	if found, ok := FindEvent(abis, evName); ok {
		ev = found
	} else if synthetic, err := syntheticEvent(rule.Match.Event); err == nil {
		ev = synthetic
	}

	topic := crypto.Keccak256Hash([]byte(rule.Match.Event))

	return &RuleMatcher{
		rule:    rule,
		address: common.HexToAddress(rule.Match.Contract),
		topic0:  topic,
		event:   ev,
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
		indexed, nonIndexed := splitIndexed(m.event.Inputs)
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

// syntheticEvent builds a minimal ABI Event from a signature like Transfer(address,address,uint256).
// Indexed fields are not inferred; all arguments are treated as non-indexed.
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
		Inputs:    args,
		Anonymous: false,
	}, nil
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
