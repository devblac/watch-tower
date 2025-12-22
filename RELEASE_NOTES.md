# Release Notes

## v0.1.0 (2025-12-22)

First stable release of watch-tower! 🎉

### What's New

**Core Features:**
- Cross-chain monitoring for EVM chains (Ethereum, Polygon, etc.) and Algorand
- Declarative YAML configuration with environment variable interpolation
- Reorg-safe block processing with configurable confirmations
- Exactly-once alert delivery via SQLite ledger
- Built-in deduplication with TTL-based expiration

**Event Matching:**
- EVM event log matching with ABI decoding
- Algorand application call and asset transfer monitoring
- Simple predicate expressions (`==`, `!=`, `>`, `<`, `in`, `contains`)
- Numeric helpers for wei and microAlgos

**Alerting:**
- Slack webhook integration
- Microsoft Teams webhook support
- Generic HTTP webhook with customizable templates
- Template system with helpers (`pretty_json`, `short_addr`)

**Operational:**
- Health check endpoint (`/healthz`) for DB and RPC status
- Optional Prometheus metrics endpoint (`/metrics`)
- Structured logging with secret redaction
- Configurable log levels (debug, info, warn, error)

**CI-Friendly:**
- `--dry-run` mode for testing without sending alerts
- `--once` flag for single-pass execution
- `--from/--to` flags for historical replay
- `validate` command for config and RPC connectivity checks

**CLI Commands:**
- `watch-tower init` - Scaffold configuration files
- `watch-tower validate -c config.yaml` - Validate config and test RPCs
- `watch-tower run -c config.yaml` - Run the monitoring pipeline
- `watch-tower state` - Show current cursors and status
- `watch-tower export` - Export alerts and cursors (JSON/CSV)
- `watch-tower version` - Show version information

### Examples

Two complete examples are included:
- `examples/evm_usdc_whale/` - Monitor large USDC transfers on Ethereum
- `examples/algo_app_watch/` - Monitor Algorand application calls

### Installation

**Download binaries:**
- Linux (amd64/arm64): `watch-tower_*_linux_*.tar.gz`
- macOS (amd64/arm64): `watch-tower_*_darwin_*.tar.gz`
- Windows (amd64/arm64): `watch-tower_*_windows_*.zip`

**Or install via Go:**
```bash
go install github.com/devblac/watch-tower/cmd/watch-tower@v0.1.0
```

**Docker:**
```bash
docker pull ghcr.io/devblac/watch-tower:v0.1.0
```

### Documentation

- [README](https://github.com/devblac/watch-tower#readme) - Quick start and configuration guide
- [Examples](https://github.com/devblac/watch-tower/tree/main/examples) - Working example configurations
- [Contributing](https://github.com/devblac/watch-tower/blob/main/CONTRIBUTING.md) - Development guidelines

### Breaking Changes

None - this is the first release!

### Known Limitations

- Predicates are simple expressions only (no complex joins or time windows)
- SQLite storage only (Postgres option coming later)
- Basic sink types (Slack, Teams, Webhook)
- No hot reload of configuration (restart required)

### What's Next

See the [roadmap](https://github.com/devblac/watch-tower/blob/main/tasks.md) for planned features:
- More chains (Solana, Base, Arbitrum, Optimism)
- Additional sinks (PagerDuty, Opsgenie, Email)
- Enhanced predicate language
- Postgres storage option
- Hot reload support

### Thanks

Thanks to all contributors and early testers! This release represents months of development focused on reliability and simplicity.

### Security

If you find a security vulnerability, please report it privately via [SECURITY.md](https://github.com/devblac/watch-tower/blob/main/SECURITY.md).

---

**Full Changelog**: https://github.com/devblac/watch-tower/compare/v0.0.0...v0.1.0
