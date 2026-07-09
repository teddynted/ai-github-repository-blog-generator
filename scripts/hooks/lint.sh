#!/usr/bin/env bash
# lint.sh — Go static analysis. Mirrors the CI `vet` step.
#
# Always runs `go vet ./...` (the baseline the CI gates on). If golangci-lint is
# installed it is run too, giving contributors richer feedback locally without
# adding a hard dependency (CI stays the source of truth).
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

cd "$REPO_ROOT"
have "$GO" || fail "Go toolchain not found — install from https://go.dev/dl/."

if ! "$GO" vet ./...; then
	fail "go vet reported problems (see above)."
fi
ok "Static analysis passed (go vet)"

if have golangci-lint; then
	if ! golangci-lint run; then
		fail "golangci-lint reported problems (see above)."
	fi
	ok "Lint successful (golangci-lint)"
else
	say "  (golangci-lint not installed — skipping; optional, see README)"
fi
