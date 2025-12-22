# Algorand Application Call Monitor

This example monitors calls to a specific Algorand application and alerts when certain conditions are met.

## Setup

1. **Get Algorand node URLs**: 
   - Algod URL: Usually `https://mainnet-api.algonode.cloud` (public) or your own node
   - Indexer URL: Usually `https://mainnet-idx.algonode.cloud` (public) or your own indexer

2. **Get a Slack webhook** (optional): Create a Slack app and add an incoming webhook.

3. **Set environment variables**:

```bash
export ALGOD_URL="https://mainnet-api.algonode.cloud"
export ALGO_INDEXER_URL="https://mainnet-idx.algonode.cloud"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
export ALLOWED_SENDERS="ADDRESS1,ADDRESS2"  # optional: filter by sender
```

4. **Update the app ID** in `config.yaml` to the application you want to monitor.

5. **Run**:

```bash
watch-tower validate -c config.yaml
watch-tower run -c config.yaml --once
```

## What it does

- Monitors calls to a specific Algorand application
- Filters by sender addresses (if `ALLOWED_SENDERS` is set)
- Sends alerts to Slack
- Deduplicates alerts for 24 hours

## Customization

**Monitor a different app:**
```yaml
match:
  type: app_call
  app_id: 12345678  # Change to your app ID
```

**Monitor ASA transfers instead:**
```yaml
match:
  type: asset_transfer
  # No app_id needed for asset transfers
```

**Add predicates:**
```yaml
where:
  - "sender in env(ALLOWED_SENDERS)"
  - "amount >= microAlgos(1000000)"  # 1 ALGO
```

## Testing

Test with dry-run:
```bash
watch-tower run -c config.yaml --dry-run --once
```

## Finding app IDs

Use AlgoExplorer or the Algorand indexer API to find application IDs:
```bash
curl "https://mainnet-idx.algonode.cloud/v2/applications?creator=ADDRESS"
```
