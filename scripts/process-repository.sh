#!/usr/bin/env bash
#
# Manually trigger an AI processing run through the authenticated POST /process
# endpoint (the manual-trigger Lambda, docs/manual-trigger.md), then wait for the
# worker to finish and report the outcome — all from your localhost.
#
# It resolves the endpoint URL and API key from the blog-gen-serverless stack
# outputs (so you never hand-copy them), fires the request, and — unless
# --no-wait — tails the worker's CloudWatch log group for the terminal
# "notification" line for this repo (status published | held | failed), which is
# the same signal for both snapshot and release runs.
#
# A successful call STARTS the on-demand EC2 host if it is stopped and launches a
# real (billable) run. Use --no-wait to fire-and-forget.
#
# Usage:
#   scripts/process-repository.sh --owner acme --repository widget
#   scripts/process-repository.sh --owner acme --repository widget --branch dev
#   scripts/process-repository.sh --owner acme --repository widget --release-tag v1.2.0
#   scripts/process-repository.sh --owner acme --repository widget --no-wait
#
# Flags / environment:
#   --owner        <o>   (required) GitHub owner/org
#   --repository   <r>   (required) repository name (without owner)
#   --branch       <b>   branch for a snapshot run          (default: main)
#   --release-tag  <t>   run release-content generation for this tag instead
#   --provider     <p>   AI-provider hint (e.g. bedrock), carried through
#   --force              request reprocessing even if already published
#   --no-wait            fire the request and exit (do not wait for completion)
#   --timeout      <s>   max seconds to wait for completion  (default: 1800)
#   --poll         <s>   seconds between completion checks    (default: 15)
#   --region       <r>   AWS region       (default: aws configure / us-east-1)
#   --stack-name   <s>   serverless stack (default: blog-gen-serverless)
#   --project      <p>   project name → log group /<project>/instance and the
#                        EC2 Project tag                      (default: blog-gen)
#   -h, --help           show this help
#
set -euo pipefail

STACK_NAME="${STACK_NAME:-blog-gen-serverless}"
REGION="${REGION:-$(aws configure get region 2>/dev/null || echo us-east-1)}"
PROJECT="${PROJECT:-blog-gen}"
OWNER=""
REPOSITORY=""
BRANCH="main"
RELEASE_TAG=""
PROVIDER=""
FORCE="false"
NO_WAIT="false"
TIMEOUT="1800"
POLL="15"

die()   { echo "error: $*" >&2; exit 1; }
info()  { echo "$*" >&2; }
usage() {
  awk 'NR==1 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "$0"
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --owner)       OWNER="${2:-}"; shift 2 ;;
    --repository)  REPOSITORY="${2:-}"; shift 2 ;;
    --branch)      BRANCH="${2:-}"; shift 2 ;;
    --release-tag) RELEASE_TAG="${2:-}"; shift 2 ;;
    --provider)    PROVIDER="${2:-}"; shift 2 ;;
    --force)       FORCE="true"; shift ;;
    --no-wait)     NO_WAIT="true"; shift ;;
    --timeout)     TIMEOUT="${2:-}"; shift 2 ;;
    --poll)        POLL="${2:-}"; shift 2 ;;
    --region)      REGION="${2:-}"; shift 2 ;;
    --stack-name)  STACK_NAME="${2:-}"; shift 2 ;;
    --project)     PROJECT="${2:-}"; shift 2 ;;
    -h|--help)     usage 0 ;;
    *)             die "unknown argument: $1 (see --help)" ;;
  esac
done

command -v aws  >/dev/null 2>&1 || die "aws CLI not found on PATH"
command -v curl >/dev/null 2>&1 || die "curl not found on PATH"
command -v jq   >/dev/null 2>&1 || die "jq not found on PATH (needed to build JSON safely)"

[ -n "$OWNER" ]      || die "--owner is required"
[ -n "$REPOSITORY" ] || die "--repository is required"

REPO_FULL="${OWNER}/${REPOSITORY}"
LOG_GROUP="/${PROJECT}/instance"

out() {
  aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text 2>/dev/null
}

info "Resolving endpoint from $STACK_NAME ($REGION)…"
PROCESS_URL="$(out ProcessUrl)"
[ -n "$PROCESS_URL" ] && [ "$PROCESS_URL" != "None" ] \
  || die "could not read ProcessUrl from $STACK_NAME — is the stack deployed?"

API_KEY_ID="$(out RegistrationApiKeyId)"
[ -n "$API_KEY_ID" ] && [ "$API_KEY_ID" != "None" ] \
  || die "could not read RegistrationApiKeyId from $STACK_NAME"
API_KEY="$(aws apigateway get-api-key --api-key "$API_KEY_ID" --include-value \
  --region "$REGION" --query value --output text)"
[ -n "$API_KEY" ] && [ "$API_KEY" != "None" ] || die "could not fetch the API key value"

# Build the request body with jq so all values are escaped correctly and secrets
# never appear in argv. Optional fields are added only when set.
BODY="$(jq -nc \
  --arg o "$OWNER" --arg r "$REPOSITORY" --arg b "$BRANCH" \
  --arg t "$RELEASE_TAG" --arg p "$PROVIDER" --argjson f "$FORCE" '
  {repository:$r, owner:$o, branch:$b, force:$f}
  + (if $t != "" then {releaseTag:$t} else {} end)
  + (if $p != "" then {provider:$p} else {} end)')"

MODE="snapshot run (branch $BRANCH)"
[ -n "$RELEASE_TAG" ] && MODE="release run (tag $RELEASE_TAG)"

# Record the start time (epoch ms) BEFORE the request, so the log tail only sees
# lines produced by this run.
START_MS="$(( $(date +%s) * 1000 ))"

info "POST /process → ${REPO_FULL} — ${MODE}"
RESPONSE="$(curl -sS -X POST "$PROCESS_URL" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d "$BODY" \
  -w $'\n%{http_code}')"

STATUS="${RESPONSE##*$'\n'}"
BODY_OUT="${RESPONSE%$'\n'*}"
echo "$BODY_OUT" | jq . 2>/dev/null || echo "$BODY_OUT"

case "$STATUS" in
  202) info "✓ accepted (HTTP 202)" ;;
  400) die "HTTP 400 — validation failed (see reason above)" ;;
  403) die "HTTP 403 — the API key was rejected or missing" ;;
  *)   die "request failed (HTTP $STATUS)" ;;
esac

REQUEST_ID="$(echo "$BODY_OUT" | jq -r '.requestId // empty' 2>/dev/null)"
[ -n "$REQUEST_ID" ] && info "requestId: $REQUEST_ID"

if [ "$NO_WAIT" = "true" ]; then
  info "--no-wait set; not waiting for completion."
  exit 0
fi

# Show the on-demand host state for context (the manual trigger starts it if
# stopped; generation only begins once it is healthy).
HOST_STATE="$(aws ec2 describe-instances --region "$REGION" \
  --filters "Name=tag:Project,Values=${PROJECT}" \
    "Name=instance-state-name,Values=pending,running,stopping,stopped" \
  --query "Reservations[].Instances[0].State.Name" --output text 2>/dev/null || true)"
[ -n "$HOST_STATE" ] && [ "$HOST_STATE" != "None" ] && info "EC2 host: ${HOST_STATE}"

info ""
info "Waiting for the worker to finish (timeout ${TIMEOUT}s, polling every ${POLL}s)…"
info "  watching ${LOG_GROUP} for the completion line for ${REPO_FULL}"

DEADLINE=$(( $(date +%s) + TIMEOUT ))
while :; do
  # Pull this run's "notification" lines (the LogNotifier terminal signal) and
  # keep only those for our repo. Works for both slog JSON and text output.
  MATCH="$(aws logs filter-log-events --region "$REGION" \
    --log-group-name "$LOG_GROUP" \
    --start-time "$START_MS" \
    --filter-pattern '"notification"' \
    --query 'events[].message' --output text 2>/dev/null \
    | tr '\t' '\n' | grep -F "$REPO_FULL" || true)"

  if echo "$MATCH" | grep -Eq 'published'; then
    ASSETS="$(echo "$MATCH" | grep -Eo '"?assets"?[=:] *[0-9]+' | grep -Eo '[0-9]+' | tail -1)"
    info ""
    info "✓ PIPELINE COMPLETE — ${REPO_FULL} published${ASSETS:+ (${ASSETS} assets)}"
    exit 0
  fi
  if echo "$MATCH" | grep -Eq 'failed'; then
    info ""
    echo "$MATCH" | grep -F failed | tail -1
    die "pipeline FAILED for ${REPO_FULL} (see the line above / ${LOG_GROUP})"
  fi
  if echo "$MATCH" | grep -Eq 'held'; then
    info ""
    info "◐ pipeline output HELD for review for ${REPO_FULL} (approval required before publish)"
    exit 0
  fi

  if [ "$(date +%s)" -ge "$DEADLINE" ]; then
    info ""
    info "⏱  timed out after ${TIMEOUT}s without a completion line."
    info "   The host may still be booting or generating (AI runs take several minutes)."
    info "   Re-check with:"
    info "     aws logs tail ${LOG_GROUP} --region ${REGION} --since ${TIMEOUT}s --follow"
    exit 2
  fi
  sleep "$POLL"
done
