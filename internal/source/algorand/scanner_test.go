package algorand

import (
	"context"
	"testing"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/common"
	"github.com/algorand/go-algorand-sdk/v2/client/v2/common/models"
	sdk "github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/devblac/watch-tower/internal/config"
	"github.com/devblac/watch-tower/internal/storage"
)

type fakeStatus struct {
	resp models.NodeStatus
	err  error
}

func (f fakeStatus) Do(ctx context.Context, headers ...*common.Header) (models.NodeStatus, error) {
	return f.resp, f.err
}

type fakeBlock struct {
	block sdk.Block
	err   error
}

func (f fakeBlock) Do(ctx context.Context, headers ...*common.Header) (sdk.Block, error) {
	return f.block, f.err
}

type fakeAlgod struct {
	status      fakeStatus
	blocks      map[uint64]sdk.Block
	blockHashes map[uint64]string
}

func (f *fakeAlgod) Status() statusGetter {
	return f.status
}

func (f *fakeAlgod) Block(round uint64) blockGetter {
	return fakeBlock{block: f.blocks[round]}
}

func (f *fakeAlgod) GetBlockHash(round uint64) blockHashGetter {
	h := f.blockHashes[round]
	if h == "" {
		h = "hash"
	}
	return fakeBlockHash{resp: models.BlockHashResponse{Blockhash: h}}
}

type fakeBlockHash struct {
	resp models.BlockHashResponse
	err  error
}

func (f fakeBlockHash) Do(ctx context.Context, headers ...*common.Header) (models.BlockHashResponse, error) {
	return f.resp, f.err
}

func TestScannerProcessesRound(t *testing.T) {
	store := newTestStore(t)

	rule := config.Rule{
		ID:     "app",
		Source: "algo",
		Match: config.MatchSpec{
			Type:  "app_call",
			AppID: 123,
		},
	}

	block := sdk.Block{
		BlockHeader: sdk.BlockHeader{
			Round:  1,
			Branch: sdk.BlockHash{},
		},
		Payset: []sdk.SignedTxnInBlock{
			{
				SignedTxnWithAD: sdk.SignedTxnWithAD{
					SignedTxn: sdk.SignedTxn{
						Txn: sdk.Transaction{
							Type: sdk.ApplicationCallTx,
							Header: sdk.Header{
								Sender: mustAddress(),
							},
							ApplicationFields: sdk.ApplicationFields{
								ApplicationCallTxnFields: sdk.ApplicationCallTxnFields{
									ApplicationID: 123,
									OnCompletion:  sdk.NoOpOC,
								},
							},
						},
					},
				},
			},
		},
	}

	client := &fakeAlgod{
		status:      fakeStatus{resp: models.NodeStatus{LastRound: 1}},
		blocks:      map[uint64]sdk.Block{1: block},
		blockHashes: map[uint64]string{1: "hash1"},
	}

	scanner, err := NewScanner(client, store, config.Source{ID: "algo", Type: "algorand", StartRound: "1"}, 0, []config.Rule{rule})
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
	if evs[0].Hash != "hash1" {
		t.Fatalf("hash mismatch")
	}
	h, _, ok, err := store.GetCursor(context.Background(), "algo")
	if err != nil || !ok || h != 1 {
		t.Fatalf("cursor not advanced: h=%d ok=%v err=%v", h, ok, err)
	}
}

func TestScannerReorgDetection(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.UpsertCursor(ctx, "algo", 1, "prevhash"); err != nil {
		t.Fatalf("seed cursor: %v", err)
	}

	block := sdk.Block{
		BlockHeader: sdk.BlockHeader{
			Round:  2,
			Branch: sdk.BlockHash{}, // does not match prevhash
		},
	}
	client := &fakeAlgod{
		status: fakeStatus{resp: models.NodeStatus{LastRound: 2}},
		blocks: map[uint64]sdk.Block{2: block},
	}

	scanner, err := NewScanner(client, store, config.Source{ID: "algo", Type: "algorand", StartRound: "1"}, 0, nil)
	if err != nil {
		t.Fatalf("new scanner: %v", err)
	}
	_, err = scanner.ProcessNext(ctx)
	if err == nil || err != ErrReorgDetected {
		t.Fatalf("expected reorg err, got %v", err)
	}
}

func mustAddress() sdk.Address {
	var a sdk.Address
	copy(a[:], []byte("SENDER0000000000000000000000000000000000000000000000000000")[:])
	return a
}

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(t.TempDir() + "/db.sqlite")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
