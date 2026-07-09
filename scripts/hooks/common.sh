#!/usr/bin/env bash
# common.sh — shared helpers for the local hook scripts and CI.
#
# Source this from other scripts: `. "$(dirname "$0")/common.sh"`. It provides
# consistent, professional output (✓ / ✗ / …), tool detection, and the repo
# root, so individual scripts stay small and focused.
set -euo pipefail

# Colours only when writing to a terminal (keeps CI logs clean).
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	C_RESET=$'\033[0m'; C_RED=$'\033[31m'; C_GREEN=$'\033[32m'
	C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'; C_BOLD=$'\033[1m'
else
	C_RESET=''; C_RED=''; C_GREEN=''; C_YELLOW=''; C_BLUE=''; C_BOLD=''
fi

# say <msg>   — informational line.
# ok <msg>    — success (green ✓).
# warn <msg>  — warning (yellow), non-fatal.
# fail <msg>  — error (red ✗) then exit 1.
# step <msg>  — section header.
say()  { printf '%s\n' "$*"; }
ok()   { printf '%s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
fail() { printf '%s✗%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }
step() { printf '\n%s%s%s\n' "$C_BOLD" "$*" "$C_RESET"; }

# have <cmd> — true if a command is on PATH.
have() { command -v "$1" >/dev/null 2>&1; }

# Repo root (works from anywhere inside the tree).
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
export REPO_ROOT

# GO is overridable so CI or a pinned toolchain can be used.
GO="${GO:-go}"
export GO
