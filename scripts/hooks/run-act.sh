#!/usr/bin/env bash
# run-act.sh — run the project's primary CI workflow locally with `act`.
#
#   run-act.sh [workflow] [event]
#     workflow  path to a workflow file (default: .github/workflows/go.yml)
#     event     GitHub event to simulate (default: push)
#
# Exit codes:
#   0  workflow passed
#   1  workflow failed
#   2  prerequisites unavailable (Docker or act not usable) — caller decides
#      whether to skip or block. This is intentionally distinct from a failure.
#
# The runner image is pinned in .actrc so every contributor runs the same thing.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

cd "$REPO_ROOT"

workflow="${1:-.github/workflows/go.yml}"
event="${2:-push}"

# --- prerequisite detection (actionable, never cryptic) ------------------
os="$(uname -s 2>/dev/null || echo unknown)"
case "$os" in
	Linux|Darwin) ;;
	*) warn "act is supported on Linux and macOS; on $os use WSL2. Skipping."; exit 2 ;;
esac

if ! have act; then
	warn "act is not installed — skipping local GitHub Actions run."
	cat >&2 <<-EOF
	  Install it to mirror CI before pushing:
	    macOS:  brew install act
	    Linux:  curl -fsSL https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
	    More:   https://nektosact.com
	EOF
	exit 2
fi

if ! have docker; then
	warn "Docker is not installed — act requires a container runtime. Skipping."
	say "  Install Docker Desktop (macOS) or Docker Engine (Linux): https://docs.docker.com/get-docker/" >&2
	exit 2
fi

if ! docker info >/dev/null 2>&1; then
	warn "Docker is installed but the daemon is not reachable. Skipping."
	say "  Start Docker (e.g. open Docker Desktop) and try again." >&2
	exit 2
fi

[ -f "$workflow" ] || fail "Workflow not found: $workflow"

# --- run -----------------------------------------------------------------
step "Running $workflow ($event event) with act…"
# ACT_EXTRA_ARGS holds optional extra flags; word-split intentionally (may be empty).
if [ -n "${ACT_EXTRA_ARGS:-}" ]; then
	# shellcheck disable=SC2086
	act "$event" -W "$workflow" $ACT_EXTRA_ARGS && { ok "act workflow passed ($workflow)"; exit 0; }
else
	act "$event" -W "$workflow" && { ok "act workflow passed ($workflow)"; exit 0; }
fi
warn "act workflow failed ($workflow)"
exit 1
