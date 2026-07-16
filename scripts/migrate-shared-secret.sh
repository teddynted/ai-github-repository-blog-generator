#!/usr/bin/env bash
#
# Migrate per-repository Secrets Manager secrets into the single shared secret
# blog-gen/github/repositories (JSON keyed by "<owner>/<name>").
#
# It reads every secret under the old prefix (default blog-gen/repos/) and
# supports BOTH historical layouts:
#   - two secrets per repo:  <prefix>/<owner>/<name>/pat  and  .../webhook-secret
#   - one JSON secret per repo: <prefix>/<owner>/<name>  ->  {"pat":…,"webhook_secret":…}
# then writes the combined document to the shared secret. It does NOT delete the
# old secrets — verify first, then remove them (see the end of this script).
#
# Requires: AWS CLI v2, jq. Run with credentials that can read the old secrets
# and put the shared secret.
#
# Usage:
#   REGION=us-east-1 scripts/migrate-shared-secret.sh          # dry run (prints the merged JSON)
#   REGION=us-east-1 APPLY=1 scripts/migrate-shared-secret.sh  # write the shared secret
set -euo pipefail

REGION="${REGION:-$(aws configure get region 2>/dev/null || echo us-east-1)}"
PROJECT="${PROJECT:-blog-gen}"
PREFIX="${PREFIX:-${PROJECT}/repos}"
SHARED="${SHARED:-${PROJECT}/github/repositories}"

command -v jq >/dev/null 2>&1 || { echo "error: jq is required" >&2; exit 1; }

echo "Scanning secrets under ${PREFIX}/ in ${REGION}..." >&2
NAMES=$(aws secretsmanager list-secrets --region "$REGION" \
  --filters "Key=name,Values=${PREFIX}/" \
  --query 'SecretList[].Name' --output text 2>/dev/null | tr '\t' '\n' | sort || true)

doc='{}'
get() { aws secretsmanager get-secret-value --region "$REGION" --secret-id "$1" --query SecretString --output text 2>/dev/null || true; }

# Collect repo keys from the "…/pat" and single-JSON secrets (one entry per repo).
while IFS= read -r name; do
  [ -z "$name" ] && continue
  case "$name" in
    */webhook-secret) continue ;;                 # handled alongside its /pat
    */pat)
      key="${name#"${PREFIX}/"}"; key="${key%/pat}"   # owner/name
      pat="$(get "$name")"
      ws="$(get "${PREFIX}/${key}/webhook-secret")"
      ;;
    *)
      key="${name#"${PREFIX}/"}"                        # owner/name (single JSON secret)
      json="$(get "$name")"
      pat="$(printf '%s' "$json" | jq -r '.pat // empty' 2>/dev/null || true)"
      ws="$(printf '%s' "$json" | jq -r '.webhook_secret // empty' 2>/dev/null || true)"
      ;;
  esac
  if [ -z "$pat" ] || [ -z "$ws" ]; then
    echo "  skip ${key}: missing pat or webhook_secret" >&2
    continue
  fi
  echo "  + ${key}" >&2
  doc="$(printf '%s' "$doc" | jq --arg k "$key" --arg p "$pat" --arg w "$ws" \
    '.[$k] = {"pat":$p, "webhook_secret":$w}')"
done <<< "$NAMES"

count="$(printf '%s' "$doc" | jq 'length')"
echo "Merged ${count} repository/ies." >&2

if [ "${APPLY:-0}" != "1" ]; then
  echo "--- DRY RUN (set APPLY=1 to write) — merged shared secret would be: ---" >&2
  printf '%s\n' "$doc" | jq 'to_entries | map(.value.pat="***" | .value.webhook_secret="***") | from_entries'
  exit 0
fi

echo "Writing ${SHARED}..." >&2
printf '%s' "$doc" | aws secretsmanager put-secret-value --region "$REGION" \
  --secret-id "$SHARED" --secret-string file:///dev/stdin >/dev/null
echo "Done. Verify the platform works, then delete the old per-repo secrets, e.g.:" >&2
echo "  for n in \$(aws secretsmanager list-secrets --region $REGION --filters Key=name,Values=${PREFIX}/ --query 'SecretList[].Name' --output text); do" >&2
echo "    aws secretsmanager delete-secret --region $REGION --secret-id \"\$n\" --recovery-window-in-days 7; done" >&2
