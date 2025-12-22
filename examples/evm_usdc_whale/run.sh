#!/bin/bash
# Quick run script for USDC whale alert

set -e

if [ ! -f .env ]; then
    echo "Error: .env file not found"
    echo "Copy .env.example to .env and fill in your values"
    exit 1
fi

source .env

if [ -z "$EVM_RPC_URL" ] || [ -z "$SLACK_WEBHOOK_URL" ]; then
    echo "Error: EVM_RPC_URL and SLACK_WEBHOOK_URL must be set in .env"
    exit 1
fi

echo "Validating config..."
watch-tower validate -c config.yaml

echo "Running watch-tower (press Ctrl+C to stop)..."
watch-tower run -c config.yaml
