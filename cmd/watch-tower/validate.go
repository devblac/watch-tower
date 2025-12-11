package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devblac/watch-tower/internal/config"
	"github.com/spf13/cobra"
)

const defaultHTTPTimeout = 8 * time.Second

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config and ping RPC endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("config invalid: %w", err)
		}
		fmt.Fprintf(out, "config OK (version %d)\n", cfg.Version)

		client := &http.Client{Timeout: defaultHTTPTimeout}
		failures := 0

		for _, src := range cfg.Sources {
			switch strings.ToLower(src.Type) {
			case "evm":
				chainID, err := pingEVM(cmd.Context(), client, src.RPCURL)
				if err != nil {
					failures++
					fmt.Fprintf(out, "- source %s (evm): ERROR %v\n", src.ID, err)
					continue
				}
				fmt.Fprintf(out, "- source %s (evm): chainId %s OK\n", src.ID, chainID)
			case "algorand":
				algodVer, algodErr := pingAlgod(cmd.Context(), client, src.AlgodURL)
				indexerVer, indexerErr := pingAlgod(cmd.Context(), client, src.IndexerURL)

				if algodErr != nil || indexerErr != nil {
					failures++
					fmt.Fprintf(out, "- source %s (algorand): algod error=%v indexer error=%v\n", src.ID, algodErr, indexerErr)
					continue
				}
				fmt.Fprintf(out, "- source %s (algorand): algod %s, indexer %s OK\n", src.ID, algodVer, indexerVer)
			default:
				failures++
				fmt.Fprintf(out, "- source %s: unsupported type %s\n", src.ID, src.Type)
			}
		}

		if failures > 0 {
			return fmt.Errorf("validate: %d source(s) failed connectivity", failures)
		}

		fmt.Fprintln(out, "validate: success")
		return nil
	},
}

func pingEVM(ctx context.Context, client *http.Client, url string) (string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_chainId",
		"params":  []any{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call eth_chainId: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("rpc status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("decode rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
	}
	if rpcResp.Result == "" {
		return "", fmt.Errorf("empty chainId result")
	}

	return rpcResp.Result, nil
}

func pingAlgod(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	url := strings.TrimRight(baseURL, "/") + "/versions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var body struct {
		Versions []string `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(body.Versions) == 0 {
		return "unknown", nil
	}
	return body.Versions[0], nil
}
