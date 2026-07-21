#!/usr/bin/env bash
# publish-metadata.sh — publish AI Platform Base AMI metadata for cross-project reuse.
#
#   scripts/ami/publish-metadata.sh --ami-id ami-0abc --version v1.2.0 \
#       [--region us-east-1] [--git-commit abc123] [--release v0.16.0] [--arch x86_64]
#
# Publishing surface (downstream projects consume ONE of these):
#   SSM Parameter Store (the source of truth, cross-account/region friendly):
#     /ai-platform/ami/base/latest              -> current AMI id  (String)
#     /ai-platform/ami/base/latest/version      -> current version
#     /ai-platform/ami/base/versions/<version>  -> AMI id for a specific version
#     /ai-platform/ami/base/metadata            -> full JSON manifest
#   EC2 tags on the AMI (queryable, drives lifecycle.sh).
#
# The companion CloudFormation stack infrastructure/ami-registry.yaml re-exports
# the SSM value as a CloudFormation Export for stacks that prefer Fn::ImportValue.
set -euo pipefail

REGION="us-east-1"
AMI_ID=""
VERSION=""
GIT_COMMIT="unknown"
RELEASE="unreleased"
ARCH="x86_64"
PREFIX="/ai-platform/ami/base"
OWNER="ai-github-repository-blog-generator"

while [ $# -gt 0 ]; do
  case "$1" in
    --ami-id) AMI_ID="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --region) REGION="$2"; shift 2 ;;
    --git-commit) GIT_COMMIT="$2"; shift 2 ;;
    --release) RELEASE="$2"; shift 2 ;;
    --arch) ARCH="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

[ -n "$AMI_ID" ] && [ -n "$VERSION" ] || { echo "error: --ami-id and --version are required" >&2; exit 2; }

aws() { command aws --region "$REGION" "$@"; }
BUILD_DATE="$(date -u +%FT%TZ)"

echo "Publishing ${AMI_ID} as ${VERSION} to ${PREFIX} (region ${REGION})…"

# 1. Tag the AMI (idempotent) so lifecycle + audits can query it.
aws ec2 create-tags --resources "$AMI_ID" --tags \
  "Key=Project,Value=ai-platform" \
  "Key=Component,Value=base-ami" \
  "Key=Version,Value=${VERSION}" \
  "Key=Architecture,Value=${ARCH}" \
  "Key=GitCommit,Value=${GIT_COMMIT}" \
  "Key=Release,Value=${RELEASE}" \
  "Key=Owner,Value=${OWNER}" \
  "Key=Lifecycle,Value=current" \
  "Key=BuildDate,Value=${BUILD_DATE}"

# 2. Build the full JSON manifest.
METADATA="$(jq -cn \
  --arg id "$AMI_ID" --arg v "$VERSION" --arg d "$BUILD_DATE" \
  --arg os "ubuntu-22.04" --arg a "$ARCH" --arg c "$GIT_COMMIT" \
  --arg r "$RELEASE" --arg o "$OWNER" \
  '{amiId:$id, version:$v, buildDate:$d, baseOS:$os, architecture:$a, gitCommit:$c, release:$r, owner:$o, lifecycle:"current"}')"

# 3. Write SSM Parameter Store (String params; overwrite for latest, immutable
#    per-version). Downstream stacks read these directly.
put() { aws ssm put-parameter --name "$1" --type String --value "$2" --overwrite \
          --tier Standard --description "$3" >/dev/null; }

put "${PREFIX}/versions/${VERSION}" "$AMI_ID"          "AI Platform Base AMI id for ${VERSION} (immutable)"
put "${PREFIX}/latest"              "$AMI_ID"           "Current AI Platform Base AMI id"
put "${PREFIX}/latest/version"      "$VERSION"          "Current AI Platform Base AMI version"
put "${PREFIX}/metadata"            "$METADATA"         "Full AI Platform Base AMI metadata (JSON)"

echo "Published:"
echo "  ${PREFIX}/latest            = ${AMI_ID}"
echo "  ${PREFIX}/latest/version    = ${VERSION}"
echo "  ${PREFIX}/versions/${VERSION} = ${AMI_ID}"
echo "  ${PREFIX}/metadata          = <json>"
echo
echo "Downstream consumption:"
echo "  CFN param type: AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>"
echo "  Default:        ${PREFIX}/latest"
