package evm

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/storage"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// BlockClient captures the subset of ethclient used by the scanner.
type BlockClient interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
}

// RPCClient is a thin wrapper over ethclient.Client that satisfies BlockClient.
type RPCClient struct {
	*ethclient.Client
}

// NewRPCClient builds an RPC client to an EVM node.
func NewRPCClient(rpcURL string) (*RPCClient, error) {
	c, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial evm rpc: %w", err)
	}
	return &RPCClient{Client: c}, nil
}

// Scanner processes blocks sequentially with confirmation safety.
type Scanner struct {
	client        BlockClient
	store         *storage.Store
	source        config.Source
	confirmations uint64
	matchers      []*RuleMatcher
	addresses     []common.Address
}

// NewScanner builds a scanner for a given source and its log rules.
func NewScanner(client BlockClient, store *storage.Store, source config.Source, confirmations uint64, abis map[string]*abi.ABI, rules []config.Rule) (*Scanner, error) {
	matchers := []*RuleMatcher{}
	addrSet := map[common.Address]struct{}{}
	for _, r := range rules {
		if r.Source != source.ID || strings.ToLower(r.Match.Type) != "log" {
			continue
		}
		m, err := NewRuleMatcher(r, abis)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, m)
		addrSet[m.address] = struct{}{}
	}

	addresses := make([]common.Address, 0, len(addrSet))
	for a := range addrSet {
		addresses = append(addresses, a)
	}

	return &Scanner{
		client:        client,
		store:         store,
		source:        source,
		confirmations: confirmations,
		matchers:      matchers,
		addresses:     addresses,
	}, nil
}

// ProcessNext handles the next eligible block (respecting confirmations) and returns matched events.
// It advances the cursor on success. If a reorg is detected, ErrReorgDetected is returned after rewinding.
func (s *Scanner) ProcessNext(ctx context.Context) ([]NormalizedEvent, error) {
	curHeight, curHash, hasCursor, err := s.store.GetCursor(ctx, s.source.ID)
	if err != nil {
		return nil, err
	}

	latest, err := s.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("latest header: %w", err)
	}
	latestHeight := latest.Number.Uint64()

	safeHeight := latestHeight
	if s.confirmations > 0 {
		if s.confirmations > safeHeight {
			return nil, nil
		}
		safeHeight -= s.confirmations
	}

	target := curHeight + 1
	if !hasCursor {
		start, err := resolveStartHeight(s.source.StartBlock, safeHeight)
		if err != nil {
			return nil, err
		}
		target = start
	}

	if target > safeHeight {
		return nil, nil
	}

	header, err := s.client.HeaderByNumber(ctx, big.NewInt(int64(target)))
	if err != nil {
		return nil, fmt.Errorf("header %d: %w", target, err)
	}

	if hasCursor && header.ParentHash.Hex() != curHash {
		rewindTo := uint64(0)
		if target > 0 {
			rewindTo = target - 1
		}
		_ = s.store.UpsertCursor(ctx, s.source.ID, rewindTo, header.ParentHash.Hex())
		return nil, ErrReorgDetected
	}

	logs, err := s.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(target)),
		ToBlock:   big.NewInt(int64(target)),
		Addresses: s.addresses,
	})
	if err != nil {
		return nil, fmt.Errorf("filter logs: %w", err)
	}

	events := []NormalizedEvent{}
	for _, lg := range logs {
		for _, m := range s.matchers {
			ev, ok, err := m.Match(lg)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			ev.Chain = Chain
			ev.SourceID = s.source.ID
			ev.Height = target
			ev.Hash = header.Hash().Hex()
			events = append(events, *ev)
		}
	}

	if err := s.store.UpsertCursor(ctx, s.source.ID, target, header.Hash().Hex()); err != nil {
		return nil, err
	}

	return events, nil
}

func resolveStartHeight(start string, safeHeight uint64) (uint64, error) {
	if start == "" || start == "0" {
		return 0, nil
	}
	if strings.HasPrefix(start, "latest-") {
		offsetStr := strings.TrimPrefix(start, "latest-")
		n, err := strconv.ParseUint(offsetStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse start_block %q: %w", start, err)
		}
		if n > safeHeight {
			return 0, nil
		}
		return safeHeight - n, nil
	}

	n, err := strconv.ParseUint(start, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse start_block %q: %w", start, err)
	}
	return n, nil
}
