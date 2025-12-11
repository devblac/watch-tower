package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/storage"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

type fakeClient struct {
	headers map[uint64]*types.Header
	logs    map[uint64][]types.Log
}

func (f *fakeClient) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if number == nil {
		var max uint64
		for n := range f.headers {
			if n > max {
				max = n
			}
		}
		if h, ok := f.headers[max]; ok {
			return h, nil
		}
		return nil, fmt.Errorf("no headers")
	}
	if h, ok := f.headers[number.Uint64()]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("header %d not found", number.Uint64())
}

func (f *fakeClient) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	from := q.FromBlock.Uint64()
	return f.logs[from], nil
}

func TestScannerProcessesBlock(t *testing.T) {
	store := newTestStore(t)
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

	parent := &types.Header{Number: big.NewInt(0)}
	h1 := &types.Header{Number: big.NewInt(1), ParentHash: parent.Hash()}

	fc := &fakeClient{
		headers: map[uint64]*types.Header{
			0: parent,
			1: h1,
		},
		logs: map[uint64][]types.Log{
			1: {
				{
					Address: common.HexToAddress(rule.Match.Contract),
					Topics: []common.Hash{
						transferTopic(rule.Match.Event),
						addrTopic(common.HexToAddress("0x0000000000000000000000000000000000000001")),
						addrTopic(common.HexToAddress("0x0000000000000000000000000000000000000002")),
					},
					Data:        common.LeftPadBytes(big.NewInt(1000).Bytes(), 32),
					TxHash:      common.HexToHash("0xabc"),
					BlockNumber: 1,
					Index:       0,
				},
			},
		},
	}

	source := config.Source{ID: "evm_main", Type: "evm", RPCURL: "stub", StartBlock: "1"}
	scanner, err := NewScanner(fc, store, source, 0, abis, []config.Rule{rule})
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	evs, err := scanner.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process next: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	h, _, ok, _ := store.GetCursor(context.Background(), source.ID)
	if !ok || h != 1 {
		t.Fatalf("cursor not advanced, h=%d ok=%v", h, ok)
	}
}

func TestScannerReorgDetection(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.UpsertCursor(ctx, "evm_main", 1, "0xparent"); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}

	h2 := &types.Header{Number: big.NewInt(2), ParentHash: common.HexToHash("0xother")}
	fc := &fakeClient{
		headers: map[uint64]*types.Header{
			2: h2,
		},
	}

	scanner, err := NewScanner(fc, store, config.Source{ID: "evm_main", Type: "evm", RPCURL: "stub"}, 0, nil, nil)
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}

	_, err = scanner.ProcessNext(ctx)
	if !errors.Is(err, ErrReorgDetected) {
		t.Fatalf("expected reorg error, got %v", err)
	}
}

func transferTopic(signature string) common.Hash {
	return crypto.Keccak256Hash([]byte(signature))
}

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_ = os.RemoveAll(dir)
	})
	return store
}

func addrTopic(addr common.Address) common.Hash {
	return common.BytesToHash(common.LeftPadBytes(addr.Bytes(), 32))
}
