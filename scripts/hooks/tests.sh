#!/usr/bin/env bash
# tests.sh — run the Go unit tests. Shared by the pre-commit hook and CI.
#
# Defaults to a fast run so commits stay quick. Set GO_TEST_FLAGS to add flags —
# CI passes "-race -cover" for a fuller signal:
#
#   GO_TEST_FLAGS="-race -cover" scripts/hooks/tests.sh
#
# GOFLAGS is respected too (e.g. GOFLAGS=-count=1 to bypass the test cache).
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

cd "$REPO_ROOT"
have "$GO" || fail "Go toolchain not found — install from https://go.dev/dl/."

# shellcheck disable=SC2086
if ! "$GO" test ./... ${GO_TEST_FLAGS:-}; then
	fail "Unit tests failed (see above)."
fi
ok "Unit tests passed"
