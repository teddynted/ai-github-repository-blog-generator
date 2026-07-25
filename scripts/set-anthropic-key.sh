#!/usr/bin/env bash
# set-anthropic-key.sh — store the Anthropic API key in Secrets Manager for the
# Claude technical-writer stage. The key is read from the first argument or the
# ANTHROPIC_API_KEY environment variable; it is NEVER printed or written to disk.
#
#   scripts/set-anthropic-key.sh 'sk-ant-...'
#   ANTHROPIC_API_KEY='sk-ant-...' scripts/set-anthropic-key.sh
#
# Env overrides: SECRET_ID (default blog-gen/anthropic/api-key), AWS_REGION
# (default us-east-1).
set -euo pipefail

SECRET_ID="${SECRET_ID:-blog-gen/anthropic/api-key}"
REGION="${AWS_REGION:-us-east-1}"
KEY="${1:-${ANTHROPIC_API_KEY:-}}"

if [ -z "$KEY" ]; then
  echo "error: no key provided. Pass it as the first argument or set ANTHROPIC_API_KEY." >&2
  echo "usage: scripts/set-anthropic-key.sh 'sk-ant-...'" >&2
  exit 1
fi
case "$KEY" in
  sk-ant-*) : ;;
  *) echo "warning: key does not start with 'sk-ant-' — double-check it is an Anthropic key." >&2 ;;
esac

# --secret-string reads the value directly; it is not echoed. Query only returns
# metadata, never the secret value.
aws secretsmanager put-secret-value \
  --secret-id "$SECRET_ID" \
  --secret-string "$KEY" \
  --region "$REGION" \
  --query '{Secret:Name,VersionId:VersionId}' --output json

echo "Anthropic API key stored in Secrets Manager ($SECRET_ID). The value was not printed."
echo "Restart the worker so it re-reads the secret at startup."
