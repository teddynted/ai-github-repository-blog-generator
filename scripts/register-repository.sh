#!/usr/bin/env bash
#
# Register (or rotate / deregister) a repository against the deployed
# registration endpoint — the curl request documented in docs/deployment.md §6.
#
# It resolves the registration URL and API key from the blog-gen-serverless
# stack outputs (so you never hand-copy them), then POSTs owner/repository/pat/
# webhook_secret. The platform validates access, creates the GitHub webhook, and
# stores the credentials in the shared Secrets Manager secret.
#
# Usage:
#   scripts/register-repository.sh --owner acme --repository widget
#   scripts/register-repository.sh --owner acme --repository widget --update
#   scripts/register-repository.sh --owner acme --repository widget --delete
#
# Secrets are read from the environment (never passed on the command line, so
# they don't land in your shell history) and are prompted for if unset:
#   GITHUB_PAT       fine-grained PAT (Contents:Read, Webhooks:RW, Metadata:Read)
#   WEBHOOK_SECRET   the HMAC secret GitHub signs deliveries with
# (both are ignored with --delete, which needs no credentials.)
#
# Flags / environment:
#   --owner        <o>   (required) GitHub owner/org
#   --repository   <r>   (required) repository name
#   --update             rotate credentials for an already-registered repo
#   --delete             deregister the repo (removes webhook + stored creds)
#   --region       <r>   AWS region      (default: aws configure / us-east-1)
#   --stack-name   <s>   serverless stack (default: blog-gen-serverless)
#   -h, --help           show this help
#
set -euo pipefail

STACK_NAME="${STACK_NAME:-blog-gen-serverless}"
REGION="${REGION:-$(aws configure get region 2>/dev/null || echo us-east-1)}"
OWNER=""
REPOSITORY=""
UPDATE="false"
DELETE="false"

die() {
  echo "error: $*" >&2
  exit 1
}

usage() {
  # Print the header comment block (everything after the shebang up to the first
  # non-comment line), stripping the leading "# ".
  awk 'NR==1 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "$0"
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --owner)       OWNER="${2:-}"; shift 2 ;;
    --repository)  REPOSITORY="${2:-}"; shift 2 ;;
    --update)      UPDATE="true"; shift ;;
    --delete)      DELETE="true"; shift ;;
    --region)      REGION="${2:-}"; shift 2 ;;
    --stack-name)  STACK_NAME="${2:-}"; shift 2 ;;
    -h|--help)     usage 0 ;;
    *)             die "unknown argument: $1 (see --help)" ;;
  esac
done

command -v aws >/dev/null 2>&1 || die "aws CLI not found on PATH"
command -v curl >/dev/null 2>&1 || die "curl not found on PATH"
command -v jq >/dev/null 2>&1 || die "jq not found on PATH (needed to build JSON safely)"

[ -n "$OWNER" ]      || die "--owner is required"
[ -n "$REPOSITORY" ] || die "--repository is required"

out() {
  aws cloudformation describe-stacks --stack-name "$STACK_NAME" --region "$REGION" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text 2>/dev/null
}

echo "Resolving endpoint from $STACK_NAME ($REGION)…" >&2
REGISTRATION_URL="$(out RegistrationUrl)"
[ -n "$REGISTRATION_URL" ] && [ "$REGISTRATION_URL" != "None" ] \
  || die "could not read RegistrationUrl from $STACK_NAME — is the stack deployed?"

API_KEY_ID="$(out RegistrationApiKeyId)"
[ -n "$API_KEY_ID" ] && [ "$API_KEY_ID" != "None" ] \
  || die "could not read RegistrationApiKeyId from $STACK_NAME"
API_KEY="$(aws apigateway get-api-key --api-key "$API_KEY_ID" --include-value \
  --region "$REGION" --query value --output text)"
[ -n "$API_KEY" ] && [ "$API_KEY" != "None" ] || die "could not fetch the API key value"

# Build the JSON body with jq so special characters in the secrets are escaped
# correctly (and the PAT/secret never appear in argv).
if [ "$DELETE" = "true" ]; then
  METHOD="DELETE"
  BODY="$(jq -nc --arg o "$OWNER" --arg r "$REPOSITORY" '{owner:$o, repository:$r}')"
else
  METHOD="POST"
  PAT="${GITHUB_PAT:-}"
  if [ -z "$PAT" ]; then
    read -rsp "GitHub PAT (github_pat_…): " PAT; echo >&2
  fi
  WHS="${WEBHOOK_SECRET:-}"
  if [ -z "$WHS" ]; then
    read -rsp "Webhook secret: " WHS; echo >&2
  fi
  [ -n "$PAT" ] || die "a GitHub PAT is required (set GITHUB_PAT or enter it when prompted)"
  [ -n "$WHS" ] || die "a webhook secret is required (set WEBHOOK_SECRET or enter it when prompted)"
  BODY="$(jq -nc --arg o "$OWNER" --arg r "$REPOSITORY" --arg p "$PAT" --arg w "$WHS" \
    --argjson u "$UPDATE" '{owner:$o, repository:$r, pat:$p, webhook_secret:$w} + (if $u then {update:true} else {} end)')"
fi

echo "${METHOD} ${OWNER}/${REPOSITORY} -> ${REGISTRATION_URL}" >&2

# -w writes the HTTP status on its own line after the body so we can gate on it.
RESPONSE="$(curl -sS -X "$METHOD" "$REGISTRATION_URL" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d "$BODY" \
  -w $'\n%{http_code}')"

STATUS="${RESPONSE##*$'\n'}"
BODY_OUT="${RESPONSE%$'\n'*}"

# Pretty-print JSON bodies when possible.
echo "$BODY_OUT" | jq . 2>/dev/null || echo "$BODY_OUT"

case "$STATUS" in
  2*) echo "✓ ${METHOD} succeeded (HTTP $STATUS)" >&2 ;;
  403) die "HTTP 403 — the API key was rejected or missing (registration requires x-api-key)" ;;
  *)   die "request failed (HTTP $STATUS)" ;;
esac
