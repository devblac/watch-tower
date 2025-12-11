package algorand

import (
	"context"
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-codec/codec"
	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/storage"
)

// statusGetter models the algod Status() fluent call.
type statusGetter interface {
	Do(ctx context.Context, headers ...*common.Header) (models.NodeStatus, error)
}

// blockGetter models the algod BlockRaw() fluent call.
type blockGetter interface {
	Do(ctx context.Context, headers ...*common.Header) ([]byte, error)
}

type blockHashGetter interface {
	Do(ctx context.Context, headers ...*common.Header) (models.BlockHashResponse, error)
}

// AlgodClient is the minimal subset of the algod client we need.
type AlgodClient interface {
	Status() statusGetter
	BlockRaw(round uint64) blockGetter
	GetBlockHash(round uint64) blockHashGetter
}

// NewAlgodClient constructs a real algod client.
func NewAlgodClient(url string) (AlgodClient, error) {
	cli, err := algod.MakeClient(url, "")
	if err != nil {
		return nil, err
	}
	return &clientAdapter{c: cli}, nil
}

type clientAdapter struct {
	c *algod.Client
}

func (a *clientAdapter) Status() statusGetter { return a.c.Status() }
func (a *clientAdapter) BlockRaw(round uint64) blockGetter {
	return a.c.BlockRaw(round)
}
func (a *clientAdapter) GetBlockHash(round uint64) blockHashGetter {
	return a.c.GetBlockHash(round)
}

// Scanner processes Algorand rounds with confirmation safety.
type Scanner struct {
	client        AlgodClient
	store         *storage.Store
	source        config.Source
	confirmations uint64
	matchers      []*RuleMatcher
}

// NewScanner builds a scanner for an Algorand source and its rules.
func NewScanner(client AlgodClient, store *storage.Store, source config.Source, confirmations uint64, rules []config.Rule) (*Scanner, error) {
	matchers := []*RuleMatcher{}
	for _, r := range rules {
		if r.Source != source.ID {
			continue
		}
		m, err := NewRuleMatcher(r)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, m)
	}

	return &Scanner{
		client:        client,
		store:         store,
		source:        source,
		confirmations: confirmations,
		matchers:      matchers,
	}, nil
}

// ProcessNext handles the next eligible round (respecting confirmations) and returns matched events.
// On success advances the cursor. On reorg returns ErrReorgDetected after rewinding.
func (s *Scanner) ProcessNext(ctx context.Context) ([]NormalizedEvent, error) {
	curRound, curHash, hasCursor, err := s.store.GetCursor(ctx, s.source.ID)
	if err != nil {
		return nil, err
	}

	status, err := s.client.Status().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("latest status: %w", err)
	}
	latest := status.LastRound
	safe := latest
	if s.confirmations > 0 {
		if safe < s.confirmations {
			return nil, nil
		}
		safe -= s.confirmations
	}

	target := curRound + 1
	if !hasCursor {
		start, err := resolveStartRound(s.source.StartRound, safe)
		if err != nil {
			return nil, err
		}
		target = start
	}

	if target > safe {
		return nil, nil
	}

	raw, err := s.client.BlockRaw(target).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("block %d: %w", target, err)
	}
	var block sdk.Block
	if err := decodeBlock(raw, &block); err != nil {
		return nil, fmt.Errorf("decode block: %w", err)
	}

	if hasCursor {
		prev := digestToString(block.BlockHeader.Branch[:])
		if prev != curHash {
			rewindTo := uint64(0)
			if target > 0 {
				rewindTo = target - 1
			}
			_ = s.store.UpsertCursor(ctx, s.source.ID, rewindTo, prev)
			return nil, ErrReorgDetected
		}
	}

	hashResp, err := s.client.GetBlockHash(target).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("block hash %d: %w", target, err)
	}
	blockHash := hashResp.Blockhash
	events, err := s.extractEvents(block)
	if err != nil {
		return nil, err
	}
	for i := range events {
		events[i].Chain = Chain
		events[i].SourceID = s.source.ID
		events[i].Height = target
		events[i].Hash = blockHash
	}

	if err := s.store.UpsertCursor(ctx, s.source.ID, target, blockHash); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Scanner) extractEvents(block sdk.Block) ([]NormalizedEvent, error) {
	var out []NormalizedEvent
	for _, stib := range block.Payset {
		tx := stib.SignedTxnWithAD.SignedTxn.Txn
		apply := stib.SignedTxnWithAD.ApplyData
		txid := crypto.TransactionIDString(tx)
		for _, m := range s.matchers {
			ev, ok, err := m.MatchTxn(tx, apply)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			ev.TxHash = txid
			ev.AppID = uint64(tx.ApplicationID)
			out = append(out, *ev)
		}
	}
	return out, nil
}

func resolveStartRound(start string, safe uint64) (uint64, error) {
	if start == "" || start == "0" {
		return 0, nil
	}
	if strings.HasPrefix(start, "latest-") {
		offsetStr := strings.TrimPrefix(start, "latest-")
		n, err := strconv.ParseUint(offsetStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse start_round %q: %w", start, err)
		}
		if n > safe {
			return 0, nil
		}
		return safe - n, nil
	}

	n, err := strconv.ParseUint(start, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse start_round %q: %w", start, err)
	}
	return n, nil
}

func digestToString(b []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

func decodeBlock(raw []byte, dest *sdk.Block) error {
	h := &codec.MsgpackHandle{}
	dec := codec.NewDecoderBytes(raw, h)
	return dec.Decode(dest)
}
