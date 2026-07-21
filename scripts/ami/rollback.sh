#!/usr/bin/env bash
# rollback.sh — roll the shared base AMI back to a previous version.
#
#   scripts/ami/rollback.sh --to v1.1.0 [--region us-east-1] [--apply]
#
# Rollback is a metadata operation: it repoints the SSM "latest" pointers at a
# previously-published, still-registered version. No image is rebuilt and no
# history is destroyed, so it is instant and reversible. Downstream stacks pick
# up the change on their next instance refresh / launch (see docs/baked-ami.md).
#
# DRY-RUN BY DEFAULT. Validates the target exists and is registered before apply.
set -euo pipefail

REGION="us-east-1"
TARGET=""
APPLY="false"
PREFIX="/ai-platform/ami/base"

while [ $# -gt 0 ]; do
  case "$1" in
    --to) TARGET="$2"; shift 2 ;;
    --region) REGION="$2"; shift 2 ;;
    --apply) APPLY="true"; shift ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    -h|--help) sed -n '2,13p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

[ -n "$TARGET" ] || { echo "error: --to <version> is required (e.g. --to v1.1.0)" >&2; exit 2; }
aws() { command aws --region "$REGION" "$@"; }

# 1. Resolve the target version -> AMI id from the immutable per-version param.
TARGET_AMI="$(aws ssm get-parameter --name "${PREFIX}/versions/${TARGET}" --query Parameter.Value --output text 2>/dev/null || echo "")"
[ -n "$TARGET_AMI" ] && [ "$TARGET_AMI" != "None" ] || {
  echo "error: no published AMI for ${TARGET} at ${PREFIX}/versions/${TARGET}" >&2
  echo "hint: list versions with: aws ssm get-parameters-by-path --path ${PREFIX}/versions --region ${REGION}" >&2
  exit 1
}

# 2. Validate the AMI still exists and is available (not deregistered).
STATE="$(aws ec2 describe-images --image-ids "$TARGET_AMI" --query 'Images[0].State' --output text 2>/dev/null || echo "missing")"
[ "$STATE" = "available" ] || {
  echo "error: target AMI ${TARGET_AMI} for ${TARGET} is '${STATE}', not 'available' — cannot roll back to it" >&2
  exit 1
}

CURRENT="$(aws ssm get-parameter --name "${PREFIX}/latest/version" --query Parameter.Value --output text 2>/dev/null || echo unknown)"
echo "Rollback plan: latest ${CURRENT} -> ${TARGET} (${TARGET_AMI}); apply=${APPLY}"

run() { if [ "$APPLY" = "true" ]; then aws "$@" >/dev/null; else echo "  DRY-RUN: aws $*"; fi; }

# 3. Repoint the latest pointers and re-tag lifecycle states.
run ssm put-parameter --name "${PREFIX}/latest" --type String --value "$TARGET_AMI" --overwrite
run ssm put-parameter --name "${PREFIX}/latest/version" --type String --value "$TARGET" --overwrite
run ec2 create-tags --resources "$TARGET_AMI" --tags "Key=Lifecycle,Value=current"

if [ "$APPLY" = "true" ]; then
  echo "Rolled back: ${PREFIX}/latest = ${TARGET_AMI} (${TARGET})"
  echo "Next: trigger an instance refresh on downstream ASGs, or relaunch to adopt it."
  echo "Validate: aws ssm get-parameter --name ${PREFIX}/latest --region ${REGION}"
else
  echo "No changes made (dry-run). Re-run with --apply to roll back."
fi
