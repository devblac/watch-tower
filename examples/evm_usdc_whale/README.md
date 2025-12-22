# EVM USDC Whale Alert

This example monitors USDC transfers on Ethereum mainnet and alerts when transfers exceed 1 million USDC.

## Setup

1. **Get an RPC endpoint**: Sign up for a free RPC provider (Alchemy, Infura, QuickNode) and get your endpoint URL.

2. **Get a Slack webhook** (optional): Create a Slack app and add an incoming webhook to your channel.

3. **Set environment variables**:

```bash
export EVM_RPC_URL="https://eth-mainnet.g.alchemy.com/v2/YOUR_KEY"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

4. **Run**:

```bash
watch-tower validate -c config.yaml
watch-tower run -c config.yaml --once
```

## What it does

- Monitors USDC contract (`0xA0b86991c6218b36c1d19d4a2e9eb0ce3606eb48`) on Ethereum
- Triggers on transfers >= 1,000,000 USDC (1M * 1e6, since USDC has 6 decimals)
- Sends alerts to Slack
- Deduplicates alerts for 24 hours using transaction hash + log index

## Customization

**Change the threshold:**
```yaml
where:
  - "value >= 5_000_000 * 1e6"  # 5M USDC instead
```

**Add more conditions:**
```yaml
where:
  - "value >= 1_000_000 * 1e6"
  - "to != 0x0000000000000000000000000000000000000000"  # exclude burns
```

**Use a different chain:**
Change the `rpc_url` to point to Polygon, Arbitrum, or any EVM chain.

**Add rate limiting:**
```yaml
rate_limit:
  capacity: 5
  rate: 0.5  # max 5 alerts, refill at 0.5/sec
```

## Testing

Test with dry-run first:
```bash
watch-tower run -c config.yaml --dry-run --once
```

This processes events but doesn't send alerts, perfect for validating your setup.
