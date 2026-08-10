#!/usr/bin/env bash
# render-release.sh — build a release video end-to-end from the LOCAL machine
# (the workaround while cloud content-gen is credit/Bedrock-blocked).
#
# Steps: build the Release Context from GitHub → generate blog + storyboard with
# Claude Code → stamp the promptVersion → upload to the content bucket → start the
# blog-gen-video render → wait for it.
#
# Usage:
#   scripts/render-release.sh v0.15.0
#   OWNER=teddynted REPO=designing-an-ai-agent-platform-on-aws scripts/render-release.sh v0.15.0
#
# Requires: gh (authenticated), aws (credentials), go, and Claude Code (`claude`).
set -euo pipefail

TAG="${1:?usage: render-release.sh <tag> (e.g. v0.15.0)}"
OWNER="${OWNER:-teddynted}"
REPO="${REPO:-designing-an-ai-agent-platform-on-aws}"
ACCOUNT="${ACCOUNT:-090686621912}"
REGION="${REGION:-us-east-1}"
CONTENT_BUCKET="${CONTENT_BUCKET:-blog-gen-content-${ACCOUNT}-${REGION}}"
SM_ARN="${SM_ARN:-arn:aws:states:${REGION}:${ACCOUNT}:stateMachine:blog-gen-video}"

FIXTURE="fixtures/designing-${TAG}.json"
OUTDIR="output/releases/${TAG}"
PFX="generated-content/${OWNER}/${REPO}/releases/${TAG}"
STORYBOARD_VERSION="storyboard@$(grep -oE '"storyboard": [0-9]+' internal/promptversion/promptversion.go | grep -oE '[0-9]+' | head -1)"

log() { printf '\033[1;36m▶ %s\033[0m\n' "$*"; }

log "1/5 Building Release Context for ${OWNER}/${REPO} ${TAG}"
GITHUB_TOKEN="$(gh auth token)" go run ./cmd/buildcontext --owner "$OWNER" --repo "$REPO" --tag "$TAG" --out "$FIXTURE"

log "2/5 Generating blog (Claude Code)"
go run ./cmd/content --artifact blog --provider claude-code --no-history --context "$FIXTURE" --output output

log "3/5 Generating storyboard (Claude Code)"
go run ./cmd/content --artifact storyboard --from-blog "$OUTDIR/blog.md" --provider claude-code --no-history --context "$FIXTURE" --output output

log "4/5 Stamping promptVersion + uploading to s3://${CONTENT_BUCKET}/${PFX}"
python3 - "$OUTDIR/.artifacts/storyboard.json" "$STORYBOARD_VERSION" <<'PY'
import json, sys
p, ver = sys.argv[1], sys.argv[2]
d = json.load(open(p))
if not d.get("promptVersion"):
    d["promptVersion"] = ver
    json.dump(d, open(p, "w"), indent=2)
    print(f"stamped {ver}")
PY
aws s3 cp "$OUTDIR/.artifacts/storyboard.json" "s3://${CONTENT_BUCKET}/${PFX}/.artifacts/storyboard.json" --only-show-errors
for f in "$OUTDIR"/*.md; do aws s3 cp "$f" "s3://${CONTENT_BUCKET}/${PFX}/$(basename "$f")" --only-show-errors; done

log "5/5 Starting render + waiting"
NAME="${TAG//./-}-$(date +%s)"
IN=$(printf '{"owner":"%s","name":"%s","tag":"%s","bucket":"%s","prefix":"generated-content"}' "$OWNER" "$REPO" "$TAG" "$CONTENT_BUCKET")
ARN=$(aws stepfunctions start-execution --state-machine-arn "$SM_ARN" --name "$NAME" --input "$IN" --query executionArn --output text)
echo "execution: $NAME"
while :; do
  ST=$(aws stepfunctions describe-execution --execution-arn "$ARN" --query status --output text)
  printf '  %s %s\n' "$(date -u +%H:%M:%SZ)" "$ST"
  [ "$ST" != "RUNNING" ] && break
  sleep 30
done
[ "$ST" = "SUCCEEDED" ] && log "✓ ${TAG} rendered → s3://blog-gen-video-${ACCOUNT}-${REGION}/${OWNER}/${REPO}/${TAG}/" || { echo "render $ST"; exit 1; }
