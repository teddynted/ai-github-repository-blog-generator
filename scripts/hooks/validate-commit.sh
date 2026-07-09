#!/usr/bin/env bash
# validate-commit.sh — enforce Conventional Commits on the commit message.
#
#   validate-commit.sh <path-to-commit-msg-file>
#
# Called by the commit-msg hook with the file Git provides. Merge, revert, and
# fixup/squash commits are allowed through unchanged.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

msg_file="${1:?usage: validate-commit.sh <commit-msg-file>}"
[ -f "$msg_file" ] || fail "Commit message file not found: $msg_file"

# First non-empty, non-comment line is the subject.
subject="$(grep -vE '^[[:space:]]*#' "$msg_file" | grep -vE '^[[:space:]]*$' | head -n1 || true)"
[ -n "$subject" ] || fail "Empty commit message."

# Allow tool-generated commits (merges, reverts, fixup/squash) through.
case "$subject" in
	"Merge "*|"Revert "*|"fixup! "*|"squash! "*|"amend! "*) exit 0 ;;
esac

# type(optional-scope)(optional !): subject
types='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'
pattern="^(${types})(\([a-z0-9._/-]+\))?(!)?: .+"

if printf '%s' "$subject" | grep -Eq "$pattern"; then
	# Soft length guidance (does not block): keep subjects readable.
	if [ "${#subject}" -gt 100 ]; then
		warn "Subject is ${#subject} chars; aim for ≤ 72 (100 max)."
	fi
	ok "Conventional Commit validation passed"
	exit 0
fi

warn "Conventional Commit validation failed"
cat >&2 <<EOF

  Your subject line:
    $subject

  Expected format:
    <type>(<optional scope>): <description>

  Allowed types:
    feat      a new feature
    fix       a bug fix
    docs      documentation only
    style     formatting; no code change
    refactor  code change that neither fixes a bug nor adds a feature
    perf      performance improvement
    test      adding or correcting tests
    build     build system or dependencies
    ci        CI configuration and scripts
    chore     maintenance; no production code change
    revert    revert a previous commit

  Examples:
    feat(webhook): implement GitHub signature validation
    fix(lambda): handle invalid GitHub signature
    docs(readme): document local development workflow
    ci(actions): share hook scripts with CI

  Add '!' after the type/scope for a breaking change, e.g. feat(api)!: ...
EOF
exit 1
