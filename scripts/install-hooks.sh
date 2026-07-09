#!/usr/bin/env bash
# install-hooks.sh — point Git at the versioned hooks in .githooks/.
#
# Idempotent. Run once after cloning (or via `make hooks`). Uninstall with:
#   git config --unset core.hooksPath
. "$(cd "$(dirname "$0")" && pwd)/hooks/common.sh"

cd "$REPO_ROOT"

git config core.hooksPath .githooks

# Ensure the exec bit is set even on filesystems/clones that dropped it.
chmod +x .githooks/* scripts/hooks/*.sh scripts/install-hooks.sh 2>/dev/null || true

ok "Git hooks installed (core.hooksPath = .githooks)"
say ""
say "Active hooks:"
say "  pre-commit  → gofmt · go vet · unit tests   (fast; no Docker)"
say "  commit-msg  → Conventional Commits check"
say "  pre-push    → act runs the primary CI workflow (needs Docker + act)"
say ""
say "Optional: install 'act' and Docker to enable the pre-push CI mirror."
say "See the README → Local Development Workflow."
