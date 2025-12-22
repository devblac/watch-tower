package health

import (
	"context"
	"fmt"
	"math/big"

	"github.com/devblac/watch-tower/internal/source/algorand"
	"github.com/devblac/watch-tower/internal/source/evm"
)

// RPCChecker combines multiple RPC health checks.
type RPCChecker struct {
	evmClients      map[string]evm.BlockClient
	algorandClients map[string]algorand.AlgodClient
}

// NewRPCChecker creates a checker for multiple RPC sources.
func NewRPCChecker(evmClients map[string]evm.BlockClient, algorandClients map[string]algorand.AlgodClient) *RPCChecker {
	return &RPCChecker{
		evmClients:      evmClients,
		algorandClients: algorandClients,
	}
}

// Ping checks all configured RPC endpoints.
func (c *RPCChecker) Ping(ctx context.Context) error {
	var lastErr error
	for id, cli := range c.evmClients {
		if _, err := cli.HeaderByNumber(ctx, big.NewInt(0)); err != nil {
			lastErr = fmt.Errorf("evm source %s: %w", id, err)
			continue
		}
	}
	for id, cli := range c.algorandClients {
		if _, err := cli.Status().Do(ctx); err != nil {
			lastErr = fmt.Errorf("algorand source %s: %w", id, err)
			continue
		}
	}
	return lastErr
}
