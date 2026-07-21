#!/usr/bin/env bash
# build-base-ami.sh — build the shared AI Platform Base AMI and publish it.
#
#   scripts/build-base-ami.sh --version v1.2.0 [--region us-east-1] [--publish]
#
# It stamps the SemVer + git commit into the image, builds with Packer, extracts
# the AMI id from the manifest, and (with --publish) tags the AMI and writes the
# metadata to SSM Parameter Store via scripts/ami/publish-metadata.sh.
#
# Rebuild monthly, on a critical CVE, or when the base software stack changes.
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
REGION="us-east-1"
VERSION=""
PUBLISH="false"

usage() { sed -n '2,12p' "$0"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --region)  REGION="$2"; shift 2 ;;
    --publish) PUBLISH="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

# SemVer is mandatory — it names, tags, and versions the artifact.
if ! printf '%s' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo "error: --version must be SemVer, e.g. v1.2.0 (got '${VERSION}')" >&2
  exit 2
fi

command -v packer >/dev/null 2>&1 || {
  echo "packer not found — install from https://developer.hashicorp.com/packer/install" >&2
  exit 1
}

GIT_COMMIT="$(git -C "$HERE" rev-parse --short HEAD 2>/dev/null || echo unknown)"
RELEASE="$(git -C "$HERE" describe --tags --abbrev=0 2>/dev/null || echo unreleased)"

cd "$HERE"
packer init packer/
echo "Building AI Platform Base AMI ${VERSION} (region=${REGION} commit=${GIT_COMMIT})…"
packer build \
  -var "region=${REGION}" \
  -var "version=${VERSION}" \
  -var "git_commit=${GIT_COMMIT}" \
  -var "release=${RELEASE}" \
  packer/ai-platform-base.pkr.hcl

# Extract region:ami-id from the manifest Packer wrote.
AMI_ID="$(jq -r '.builds[-1].artifact_id' packer-manifest.json | cut -d: -f2)"
[ -n "$AMI_ID" ] && [ "$AMI_ID" != "null" ] || { echo "error: could not read AMI id from packer-manifest.json" >&2; exit 1; }
echo "Built AMI: ${AMI_ID} (${VERSION})"

if [ "$PUBLISH" = "true" ]; then
  "$HERE/scripts/ami/publish-metadata.sh" \
    --ami-id "$AMI_ID" --version "$VERSION" --region "$REGION" \
    --git-commit "$GIT_COMMIT" --release "$RELEASE"
else
  echo "Skipping publish (pass --publish to write SSM Parameter Store + tags)."
  echo "Manual publish: scripts/ami/publish-metadata.sh --ami-id ${AMI_ID} --version ${VERSION}"
fi
