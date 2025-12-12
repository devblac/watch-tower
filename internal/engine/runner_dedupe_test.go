package engine

import (
	"testing"
)

func TestBuildDedupeKey(t *testing.T) {
	idx := uint(5)
	ev := Event{
		TxHash:   "0xabc",
		LogIndex: &idx,
		AppID:    42,
	}

	key := buildDedupeKey("txhash:logIndex:app_id", ev)
	if key != "0xabc:5:42" {
		t.Fatalf("unexpected key: %s", key)
	}

	key = buildDedupeKey("", ev)
	if key != "0xabc" {
		t.Fatalf("default key mismatch: %s", key)
	}
}
