# watch-tower — Build Plan & Spec (Go, single binary)

## 0) Vision & Constraints
- Goal: deterministic, reorg-safe, cross-chain monitoring & alerting CLI (EVM + Algorand), CI-first.
- Language: Go; single static-ish binary; lean deps (stdlib + go-ethereum, Algorand Go SDK).
- Principles: YAGNI, minimal surface; deterministic correctness; operational simplicity; reproducible runs.
- Must-ship v1: declarative YAML rules; confirmations + rewind + exactly-once alerts via tiny SQLite ledger; replay/time-travel; CI-friendly flags; simple sinks (Slack/Teams/Webhook/Email), metrics/health.
- Non-goals v1: AI, SaaS control plane, plugins, complex DSL; only basic predicates.

## 1) Architecture (minimal)
- Pipeline per rule: Source (chain cursor) → Decoder (ABI/Algorand app) → Predicates → Sink(s) → Ack.
- State: embedded SQLite (default `./watch_tower.db`) with tables `cursors`, `alerts`, `sends`, `dedupe`.
- Finality: per-chain confirmations; store `{height, hash}`; detect reorgs → rewind & reprocess.
- Config: YAML with env interpolation; rule graphs are acyclic and small.
- Metrics/health: optional `/metrics`; `/healthz` reflects DB + RPC reachability.

## 2) Example config.yaml
```yaml
version: 1
global:
  db_path: "./watch_tower.db"
  confirmations:
    evm: 12
    algorand: 10
sources:
  - id: evm_main
    type: evm
    rpc_url: ${EVM_RPC_URL}
    start_block: "latest-5000"
    abi_dirs: ["./abis"]
  - id: algo_main
    type: algorand
    algod_url: ${ALGOD_URL}
    indexer_url: ${ALGO_INDEXER_URL}
    start_round: "latest-10000"
rules:
  - id: usdc_whale
    source: evm_main
    match:
      type: log
      contract: "0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
      event: "Transfer(address,address,uint256)"
      where:
        - "value >= 1_000_000 * 1e6"
    sinks: ["slack_ops","risk_webhook"]
    dedupe:
      key: "txhash:logIndex"
      ttl: "24h"
  - id: algo_app_watch
    source: algo_main
    match:
      type: app_call
      app_id: 12345678
      where:
        - "sender in env(ALLOWED_SENDERS)"
    sinks: ["slack_ops"]
sinks:
  - id: slack_ops
    type: slack
    webhook_url: ${SLACK_WEBHOOK_URL}
    template: "ALERT {{rule_id}} {{chain}} {{txhash}} {{pretty_json}}"
  - id: risk_webhook
    type: webhook
    url: ${RISK_WEBHOOK}
    method: POST
```

## 3) CLI Spec (minimal, CI-ready)
- `watch-tower init` → scaffold config, sample ABIs, CI workflow, goreleaser stub.
- `watch-tower validate -c config.yaml` → schema + env + RPC ping + secret checks.
- `watch-tower run -c config.yaml [--once] [--dry-run] [--from H --to H]`
- `watch-tower state` → show cursors, lag, latest hashes.
- `watch-tower export alerts|cursors --format csv|json`
- `watch-tower version`
- Alias `wt` only if trivial.

## 4) Repo Layout (small)
- `/cmd/watch-tower`
- `/internal/config`
- `/internal/storage`       # sqlite, migrations
- `/internal/source/evm`    # ethclient, ABI decode
- `/internal/source/algorand`
- `/internal/engine`        # predicates, dedupe, pipeline
- `/internal/sink/slack`
- `/internal/sink/teams`
- `/internal/sink/webhook`
- `/internal/metrics`       # optional minimal Prometheus
- `/abis` `/examples` `/scripts`

## 5) Tasks & Acceptance Criteria (checklists)
- Phase A — Bootstrap
  - [x] Go module `github.com/devblac/watch-tower`; Cobra CLI; Makefile; basic main.
  - [x] CI: lint, test, build (linux/darwin/windows); minimal `.goreleaser.yaml`.
  - AC: `go build ./cmd/watch-tower` passes in CI; `watch-tower version` runs.
- Phase B — Config (YAML)
  - [x] Load/validate via `yaml.v3`; env interpolation; schema checks.
  - AC: `watch-tower validate -c config.yaml` prints pass; RPC endpoints reachable; secrets only via `${...}`.
- Phase C — Storage (SQLite, minimal)
  - [x] Tables: cursors(source_id, height, hash, updated_at); alerts(id, rule_id, fingerprint, txhash, payload_json, created_at); sends(alert_id, sink_id, status, response_code, created_at); dedupe(key, expires_at).
  - [x] Reusable migrations; transactions for exactly-once.
  - AC: Unit tests for cursor upsert, dedupe TTL, exactly-once semantics.
- Phase D — EVM Source (MVP)
  - [x] ethclient HTTP; block pull with confirmations; parent hash verify; rewind N on mismatch.
  - [x] ABI loader; filter logs by address/topic; map to `NormalizedEvent`.
  - AC: USDC Transfer rule matches on testnet; event decoded; alert enqueued once.
- Phase E — Algorand Source (MVP)
  - [x] algod + indexer; round cursor; basic app call & ASA transfer decode.
  - AC: Sample app call triggers alert; rewind works across rounds.
- Phase F — Engine & Predicates (tiny)
  - [x] Predicates: `== != > < in contains`; numeric helpers (wei, microAlgos). 
  - [x] Simple token-bucket rate limit per rule.
  - AC: Table-driven tests for predicate eval; rate-limit respected.
- Phase G — Sinks (minimal)
  - [x] Slack webhook; generic HTTP webhook; Teams webhook (shared impl).
  - [x] Templates via `text/template`; helpers `pretty_json`, `short_addr`.
  - AC: Duplicate events do not resend (dedupe key honored); sink errors retried with backoff.
- Phase H — Replay & Dry-Run
  - [x] `--from/--to` historical scan; progress output; backpressure.
  - [x] `--dry-run` writes alerts only; no network sends.
  - AC: 5k-block replay yields expected count; zero sink calls in dry-run; deterministic reruns.
- Phase I — Ops (only needed)
  - [x] `/healthz` HTTP (db, RPCs); optional `/metrics` Prometheus counters.
  - [x] Structured logs; secrets redacted; minimal log levels.
  - AC: Health reflects failures correctly; secrets never logged in tests.
- Phase J — Packaging
  - [ ] goreleaser for win/linux/darwin (amd64/arm64); checksums; SBOM.
  - [ ] Dockerfile (distroless, small image).
  - AC: v0.1.0 GitHub Release artifacts publish; images run with sample config.
- Phase K — Docs & Examples
  - [x] README (quickstart <15 min), config reference, reorg model, replay usage, CI example.
  - [x] `examples/evm_usdc_whale`, `examples/algo_app_watch` with scripts.
  - AC: Fresh clone → alert on testnet in <15 minutes via documented steps.

## 6) Data Types (sketch)
```go
type NormalizedEvent struct {
  Chain, SourceID string
  Height          uint64
  Hash, TxHash    string
  LogIndex        *uint32
  Contract        string // or empty for Algorand
  AppID           uint64 // Algorand
  Name            string
  Args            map[string]any
}

type Rule struct {
  ID        string
  Match     MatchSpec
  Preds     []string // simple expressions
  Sinks     []string
  DedupeKey string
  DedupeTTL time.Duration
}
```

## 7) Differentiation (vs ethereum-watcher)
- Chains: EVM + Algorand (not EVM-only).
- Config: declarative YAML rules, no code.
- Reliability: confirmations + rewind + exactly-once ledger.
- Ops: `--dry-run`, `--once`, replay, health, optional metrics.
- Packaging: single binary + Docker + Homebrew/Choco-friendly.

## 8) Performance Targets
- Live tail lag ≤ 2 blocks/rounds under normal load.
- Replay ≥ 2k EVM logs/sec/core locally.
- Sink p95 < 300ms; zero duplicate sends under retries.

## 9) Security & Secrets (minimal)
- Secrets only via env; refuse plain secrets in config unless `${...}`.
- Mask secrets in logs; document least-privilege RPC/sink tokens.
- HTTPS required for sinks; no plaintext tokens in exports.

## 10) Roadmap (post-v1, do not implement now)
- More chains: Solana, Base/Arbitrum/Optimism presets.
- Extra sinks: PagerDuty/Opsgenie; signed webhooks; email with DKIM.
- Rule language: arithmetic, cross-event joins, sliding windows.
- Storage: Postgres option; WAL/high-throughput mode.
- Hot reload; rule status UI; WS subscribers; backfill helpers; preset rules.

## 11) Branding
- Name: `watch-tower`. Optional alias `wt` only if trivial.

## 12) Definition of Done (Zero → Prod)
- v0.1.0 release with binaries, Docker, Homebrew/Choco; SBOM.
- Two demo rules proven on public testnets (EVM + Algorand).
- CI green (lint/unit/integration), reproducible builds.
- Docs enable first alert in <15 minutes.

