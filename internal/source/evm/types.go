package evm

import (
	"errors"
)

// Chain is the identifier for EVM chains.
const Chain = "evm"

// ErrReorgDetected signals that the chain rewound; caller should restart from the updated cursor.
var ErrReorgDetected = errors.New("reorg detected")

// NormalizedEvent represents a decoded on-chain event in a uniform shape.
type NormalizedEvent struct {
	Chain    string
	SourceID string
	Height   uint64
	Hash     string
	TxHash   string
	LogIndex *uint
	Contract string
	Name     string
	Args     map[string]any
}
