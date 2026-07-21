#!/usr/bin/env bash
# lifecycle.sh — AI Platform Base AMI lifecycle management.
#
#   scripts/ami/lifecycle.sh [--region us-east-1] [--retain 3] [--apply]
#
# Enforces the retention policy over the versioned base AMIs (tagged
# Project=ai-platform, Component=base-ami):
#   - keeps the newest --retain images "current"/"previous" (rollback headroom),
#   - marks older images "deprecated" (EC2 image deprecation + tag),
#   - deregisters images older than the retention window and deletes their
#     backing EBS snapshots (obsolete cleanup).
#
# DRY-RUN BY DEFAULT — it prints the plan. Pass --apply to make changes.
# The current "latest" (per SSM /ai-platform/ami/base/latest) is NEVER touched.
set -euo pipefail

REGION="us-east-1"
RETAIN=3
APPLY="false"
PREFIX="/ai-platform/ami/base"

while [ $# -gt 0 ]; do
  case "$1" in
    --region) REGION="$2"; shift 2 ;;
    --retain) RETAIN="$2"; shift 2 ;;
    --apply) APPLY="true"; shift ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    -h|--help) sed -n '2,18p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

aws() { command aws --region "$REGION" "$@"; }
run() { if [ "$APPLY" = "true" ]; then aws "$@"; else echo "  DRY-RUN: aws $*"; fi; }

LATEST="$(aws ssm get-parameter --name "${PREFIX}/latest" --query Parameter.Value --output text 2>/dev/null || echo "")"
echo "Retention policy: keep newest ${RETAIN} (apply=${APPLY}); current latest=${LATEST:-none}"

# All base AMIs owned by us, newest first (CreationDate desc).
mapfile -t IMAGES < <(aws ec2 describe-images --owners self \
  --filters "Name=tag:Project,Values=ai-platform" "Name=tag:Component,Values=base-ami" \
  --query 'reverse(sort_by(Images,&CreationDate))[].[ImageId,CreationDate]' --output text | awk '{print $1}')

total=${#IMAGES[@]}
echo "Found ${total} base AMIs."
[ "$total" -eq 0 ] && exit 0

i=0
for ami in "${IMAGES[@]}"; do
  i=$((i+1))
  if [ "$ami" = "$LATEST" ] || [ "$i" -le "$RETAIN" ]; then
    echo "[keep]       ${ami} (rank ${i})"
    continue
  fi
  # Within the deprecation grace band (retain+1 .. retain*2): deprecate, don't delete.
  if [ "$i" -le $((RETAIN * 2)) ]; then
    echo "[deprecate]  ${ami} (rank ${i})"
    run ec2 enable-image-deprecation --image-id "$ami" \
      --deprecate-at "$(date -u -d '+1 day' +%FT%TZ 2>/dev/null || date -u -v+1d +%FT%TZ)"
    run ec2 create-tags --resources "$ami" --tags "Key=Lifecycle,Value=deprecated"
    continue
  fi
  # Beyond the window: deregister + delete backing snapshots (obsolete cleanup).
  echo "[delete]     ${ami} (rank ${i})"
  mapfile -t SNAPS < <(aws ec2 describe-images --image-ids "$ami" \
    --query 'Images[0].BlockDeviceMappings[?Ebs].Ebs.SnapshotId' --output text | tr '\t' '\n')
  run ec2 deregister-image --image-id "$ami"
  for snap in "${SNAPS[@]}"; do
    [ -n "$snap" ] && [ "$snap" != "None" ] || continue
    echo "  [snapshot] ${snap}"
    run ec2 delete-snapshot --snapshot-id "$snap"
  done
done

echo "Lifecycle pass complete."
[ "$APPLY" = "true" ] || echo "No changes made (dry-run). Re-run with --apply to enforce."
