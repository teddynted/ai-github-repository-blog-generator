#!/usr/bin/env bash
# format.sh — Go formatting (gofmt, and goimports when available).
#
#   format.sh            format the whole module in place        (make fmt)
#   format.sh --check    report unformatted files, don't edit    (CI)
#   format.sh --staged   format only staged Go files, re-stage   (pre-commit)
#
# gofmt is always required; goimports is used only if it is installed.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

cd "$REPO_ROOT"
have gofmt || fail "gofmt not found — install the Go toolchain (https://go.dev/dl/)."

apply_goimports() { # $@ = files
	if have goimports && [ "$#" -gt 0 ]; then
		goimports -w "$@" 2>/dev/null || true
	fi
}

case "${1:-}" in
--check)
	unformatted="$(gofmt -l . 2>/dev/null || true)"
	if [ -n "$unformatted" ]; then
		warn "These files are not gofmt-clean:"
		printf '  %s\n' $unformatted >&2
		fail "Run 'make fmt' and commit the result."
	fi
	ok "Formatting check passed (gofmt)"
	;;

--staged)
	# Only consider staged Go files (Added/Copied/Modified/Renamed).
	# Portable array fill (no mapfile — macOS ships bash 3.2).
	staged=()
	while IFS= read -r line; do
		[ -n "$line" ] && staged+=("$line")
	done < <(git diff --cached --name-only --diff-filter=ACMR -- '*.go' 2>/dev/null || true)
	[ "${#staged[@]}" -eq 0 ] && { ok "Formatting successful (no staged Go files)"; exit 0; }

	unformatted=()
	for f in "${staged[@]}"; do
		[ -f "$f" ] || continue
		[ -n "$(gofmt -l "$f" 2>/dev/null)" ] && unformatted+=("$f")
	done
	[ "${#unformatted[@]}" -eq 0 ] && { ok "Formatting successful (staged files gofmt-clean)"; exit 0; }

	# Refuse to auto-stage files that also carry unstaged edits, to avoid
	# silently committing work the developer had not staged.
	conflicts=()
	for f in "${unformatted[@]}"; do
		git diff --name-only -- "$f" | grep -qx "$f" && conflicts+=("$f")
	done
	if [ "${#conflicts[@]}" -gt 0 ]; then
		warn "These staged files need formatting but also have unstaged changes:"
		printf '  %s\n' "${conflicts[@]}" >&2
		fail "Stage or stash the unstaged changes, then commit again (avoids staging partial work)."
	fi

	gofmt -w "${unformatted[@]}"
	apply_goimports "${unformatted[@]}"
	git add -- "${unformatted[@]}"
	ok "Formatting applied and re-staged:"
	printf '  %s\n' "${unformatted[@]}"
	;;

*)
	changed="$(gofmt -l . 2>/dev/null || true)"
	if [ -n "$changed" ]; then
		gofmt -w .
		# shellcheck disable=SC2086
		apply_goimports $changed
		ok "Formatting applied to:"
		printf '  %s\n' $changed
	else
		ok "Formatting successful (already gofmt-clean)"
	fi
	;;
esac
