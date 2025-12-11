package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInterpolatesEnvAndValidates(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	cfgYAML := `
version: 1
sources:
  - id: evm_main
    type: evm
    rpc_url: ${RPC_URL}
rules:
  - id: r1
    source: evm_main
    match:
      type: log
      contract: "0x0"
      event: "E()"
    sinks: ["sink1"]
sinks:
  - id: sink1
    type: slack
    webhook_url: ${SLACK_HOOK}
`
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("RPC_URL", "http://example-rpc")
	t.Setenv("SLACK_HOOK", "https://hooks.slack.test")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("expected load to succeed: %v", err)
	}

	if got := cfg.Sources[0].RPCURL; got != "http://example-rpc" {
		t.Fatalf("rpc_url not interpolated, got %q", got)
	}
}

func TestLoadFailsOnMissingEnv(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	cfgYAML := `
version: 1
sources:
  - id: evm_main
    type: evm
    rpc_url: ${RPC_URL}
rules:
  - id: r1
    source: evm_main
    match:
      type: log
      contract: "0x0"
      event: "E()"
    sinks: ["sink1"]
sinks:
  - id: sink1
    type: slack
    webhook_url: ${SLACK_HOOK}
`
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatalf("expected missing env to fail")
	}
}
