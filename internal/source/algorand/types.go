package algorand

import "errors"

// Chain identifier for Algorand.
const Chain = "algorand"

// ErrReorgDetected signals that the chain rewound; caller should restart from the updated cursor.
var ErrReorgDetected = errors.New("reorg detected")

// NormalizedEvent represents a decoded on-chain event in a uniform shape.
type NormalizedEvent struct {
	Chain    string
	SourceID string
	RuleID   string
	Height   uint64
	Hash     string
	TxHash   string
	AppID    uint64
	Name     string
	Args     map[string]any
}
