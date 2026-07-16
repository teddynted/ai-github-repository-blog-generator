# Semantic Versioning & Release Management

This project uses **[Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html)**,
**[Conventional Commits](https://www.conventionalcommits.org/)**, annotated Git
tags, a maintained `CHANGELOG.md`, and **GitHub Releases**. A single `release`
CLI ([`cmd/release`](../cmd/release)) drives the whole workflow deterministically.

Every **GitHub Release** is the authoritative record of a versioned software
release — and, on the deployed platform, a **trigger for AI content generation**
(the webhook handler treats a published release as an intentional event). The
release tooling and the content platform stay decoupled: the CLI just creates
the release; the platform reacts to it.

Related: [Workflows](./workflows.md) · [CI/CD](./ci-cd.md) · [Contributing](./contributing.md).

---

## 1. Semantic Versioning strategy

Versions are `MAJOR.MINOR.PATCH[-prerelease][+build]`:

| Increment | When | Example |
| --- | --- | --- |
| **MAJOR** | breaking change | `1.4.2 → 2.0.0` |
| **MINOR** | backward-compatible feature | `1.4.2 → 1.5.0` |
| **PATCH** | backward-compatible fix | `1.4.2 → 1.4.3` |
| **pre-release** | unstable preview | `2.0.0-rc.1` |
| **build metadata** | ignored for precedence | `2.0.0+exp.sha.5114f85` |

All versions are validated against the SemVer 2.0.0 grammar; invalid formats are
rejected. Precedence follows §11 of the spec (a pre-release has lower precedence
than the associated normal version; build metadata never affects ordering).

## 2. Conventional Commit guidelines

Commit subjects must be `type(scope)!: description`:

```text
feat(api): add POST /process endpoint
fix: handle nil head commit
refactor(worker)!: change SQS envelope shape

BREAKING CHANGE: the envelope field was renamed
```

Recognised **types**: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, `revert`. A `!` after the type/scope **or** a
`BREAKING CHANGE:` footer marks a breaking change. Scope is optional.

The release workflow **rejects** non-conforming commits made since the last tag.

### Version determination

| Commit(s) since last tag | Next version |
| --- | --- |
| any breaking change | **major** |
| at least one `feat` | **minor** |
| at least one `fix`/`perf`/`revert` | **patch** |
| only `docs`/`chore`/`test`/`ci`/`build`/`style` | **no release** |

You can always **override** with an explicit `release major|minor|patch`.

## 3. Release workflow

`release <bump>` runs, in order:

1. Validate repository state (clean tree, release branch, in sync with remote, GitHub auth).
2. Validate Conventional Commits since the last tag.
3. Determine the next version (auto or forced).
4. Generate release notes.
5. Update `CHANGELOG.md`.
6. **Commit `CHANGELOG.md`** with `chore(release): <tag>` and **push it to the
   release branch** (`main`). The changelog reaches the remote **first**, so
   other clones just `git pull` to pick it up.
7. Create an annotated Git tag **on the changelog commit**.
8. Push the tag.
9. Create the GitHub Release.
10. Print a summary.

Because the changelog is committed and pushed **before** the tag is created, the
tag — and the GitHub Release built from it — always points at a commit that
already contains the CHANGELOG entry. The changelog commit is made with
`--no-verify` (the CLI has already validated everything, so the pre-commit hooks
are not re-run for a generated changelog-only commit).

The workflow is **idempotent** where practical: an existing tag or release is
detected and not recreated; re-adding a version to the CHANGELOG is a no-op.

## 4. Git tagging strategy

- Tags are **annotated** and prefixed (`v` by default): `v1.0.0`, `v1.1.0`, `v2.0.0`.
- The tag format is validated; **duplicate tags are refused**; a version that
  does not increase over the latest tag (**downgrade or repeat**) is refused.
- The tag is pushed to `origin` only after validation passes.

## 5. CHANGELOG management

`CHANGELOG.md` follows Keep-a-Changelog style. Each release adds a section:

```markdown
## [1.5.0] - 2026-07-16

### Breaking Changes
- ...

### Features
- **api:** add POST /process endpoint

### Bug Fixes
- handle nil head commit
```

Sections are grouped by category (Features, Bug Fixes, Performance, Refactoring,
Documentation, plus Breaking Changes) and inserted newest-first below the header.
**Existing entries are never duplicated.**

On a real release the CLI **writes, commits, and pushes** `CHANGELOG.md` to the
release branch itself (message `chore(release): <tag>`). You do **not** commit it
by hand — after the release, other clones sync it with a plain `git pull`. The
commit message template is configurable via `release_commit_message` in
`.release.json` (see [§8](#8-configuration)).

## 6. GitHub Releases

Each release publishes generated notes (version, date, summary, features, fixes,
breaking changes, contributors) against the pushed tag, with a link to the
CHANGELOG section. **Duplicate releases are prevented** (the CLI checks for an
existing release for the tag first). Release assets are supported by the API for
future use.

Authentication: set `GITHUB_TOKEN` or `GH_TOKEN` (a token with `contents:write`).
The CLI validates auth before attempting to create a release.

## 7. CLI usage

### How to invoke it

The CLI is `cmd/release`. Run it any of these ways (they're equivalent):

```bash
go run ./cmd/release <command> [flags]       # no build needed
make release ARGS="<command> [flags]"        # via the Makefile
make build-release && ./dist/release ...     # build once to dist/release
```

Throughout this doc, `release <command>` is shorthand for one of the above —
substitute your preferred form. If you use the binary a lot, alias it:
`alias release='go run ./cmd/release'` (from the repo root).

> **Flag order matters.** Flags must come **before** the subcommand
> (`release --dry-run minor` ✓, `release minor --dry-run` ✗) — Go's flag parser
> stops at the first non-flag argument.

### Commands

```bash
go run ./cmd/release major        # force a major release
go run ./cmd/release minor
go run ./cmd/release patch
go run ./cmd/release              # auto-derive the bump from commits

go run ./cmd/release validate     # run all pre-release checks; no changes
go run ./cmd/release version      # print current + next version
go run ./cmd/release notes        # print the release notes for the next version
go run ./cmd/release changelog    # print CHANGELOG with the next release added
```

Flags: `--dry-run`, `--pre <id>` / `--prerelease`, `--config <path>`,
`--repo <owner/name>`, `--changelog <path>`, `--no-verify`. Exit codes: `0`
success, `1` runtime/validation failure, `2` usage error.

### Dry-run examples

A **dry-run still runs validation** (it's a rehearsal of the whole workflow), so
on a feature branch / without a token it will report the blockers and exit `1`
*before* the preview. Add **`--no-verify`** to skip the checks and just see the
computed version, CHANGELOG, and notes:

```bash
# Pure preview — version, CHANGELOG section, and release notes; no checks, no changes:
go run ./cmd/release --dry-run --no-verify minor

# Full dry-run including validation (as it would run for real):
go run ./cmd/release --dry-run minor

# What version would auto-derivation choose?
go run ./cmd/release version
#   current: v1.4.2
#   next:    v1.5.0 (minor)

# Pre-release candidate:
go run ./cmd/release --dry-run --pre rc.1 major     # → v2.0.0-rc.1
```

A real release (on the release branch, clean tree, token set):

```bash
git checkout main
export GITHUB_TOKEN=ghp_xxx
go run ./cmd/release minor
```

### Validation model & severity

Every check reports one of three severities, so `--dry-run` works as a local
**planning tool without GitHub credentials** while a real release still enforces
everything:

| Severity | Meaning | Dry-run | Real release |
| --- | --- | --- | --- |
| **✓ success** | passed | — | — |
| **⚠ warning** | noted, non-blocking | does **not** fail | (see below) |
| **✗ error** | release-blocking | fails | fails |

| Check | Dry-run | Real release |
| --- | --- | --- |
| Semantic Version valid | error | error |
| Conventional Commits (root commit excluded) | error | error |
| Clean working tree | error | error |
| Release branch | error | error |
| Tag does not exist | error | error |
| Synchronized with remote | error | error |
| **GitHub authentication** | **warning** | **error** |
| **GitHub connectivity** | **warning (skipped)** | **error** |
| GitHub Release creation | skipped | performed |

So a dry-run on `main` with a clean, synced tree and **no token** exits `0` with
two warnings; the same state for a real release exits non-zero (auth required).

**The repository's initial (root) commit is excluded** from Conventional Commit
validation — a non-conventional `first commit` never fails validation, and you
never need to rewrite history to satisfy the validator.

### Exit codes

| | Only warnings | Any error |
| --- | --- | --- |
| `--dry-run` | **0** | non-zero |
| real release | non-zero (auth is required) | non-zero |
| usage error (bad flag/command) | — | **2** |

### Example output

```text
Release Plan
────────────────────────────────────

Current Version  : (none)
Next Version     : v0.1.0
Increment        : Minor

Commits Analysed : 101
Conventional     : 101

Validation
────────────────────────────────────

✓ Semantic Version valid — 0.1.0
✓ Conventional Commits validated — 101 commit(s)
✓ Repository clean
✓ Release branch verified — main
✓ Tag does not exist — v0.1.0
✓ Synchronized with remote
⚠ GitHub authentication not configured — GITHUB_TOKEN or GH_TOKEN not found.
⚠ GitHub connectivity skipped (dry-run)

Planned Actions  (simulated — not executed)
────────────────────────────────────
✓ Update CHANGELOG.md
✓ Commit + push CHANGELOG.md to main
✓ Create Git tag (on the changelog commit)
✓ Push tag
✓ Create GitHub Release

Dry run completed successfully.

No release actions were executed.
```

On a blocking failure the run aborts and prints the offending check(s):

```text
✗ Current branch
  Current : feat/release-management
  Expected: main

Release aborted.
```

## 8. Configuration

Optional `.release.json` at the repo root (defaults apply when absent):

| Key | Default | Meaning |
| --- | --- | --- |
| `initial_version` | `0.1.0` | version used when no tags exist yet |
| `tag_prefix` | `v` | prepended to versions to form tags |
| `release_branch` | `main` | the only branch a release may be cut from; also where the CHANGELOG commit is pushed |
| `release_commit_message` | `chore(release): %s` | commit message for the CHANGELOG update (`%s` → tag); must stay a valid Conventional Commit |
| `prerelease_id` | `rc` | default identifier for `--prerelease` |
| `ignored_types` | docs, style, test, chore, ci, build | types that don't trigger a release on their own |
| `commit.types` | the 10 standard types | allowed commit types |
| `commit.minor_types` | `feat` | types that bump minor |
| `commit.patch_types` | `fix`, `perf`, `revert` | types that bump patch |
| `changelog_categories` | Features, Bug Fixes, Performance, Refactoring, Documentation | changelog grouping/order |

See [`.release.json`](../.release.json) for the shipped example.

## 9. Logging

The CLI logs a structured line (`msg="validation complete"`) with the
**validation duration**, the **planned release version**, the **bump**, the
number of commits **analysed** and **conventional**, and the **dry-run status**,
plus version-calculation and GitHub-API events. Human-readable progress (the plan
+ report) goes to stdout; logs go to stderr. Tokens and other secrets are never
logged.

## 10. Troubleshooting

| Symptom | Cause / fix |
| --- | --- |
| `tag vX already exists` | that version is already released; choose a higher bump |
| `version … does not increase` | the forced bump would repeat/lower the version |
| `N non-conventional commit(s)` | fix the offending commit messages (rebase) or they can't be released |
| `no release-worthy commits` | only docs/chore since last tag; use an explicit bump to force |
| `working tree is not clean` | commit or stash changes first |
| `on branch "x"; releases must be cut from "main"` | switch to the release branch |
| `local branch is not in sync with the remote` | `git pull` / `git push` first |
| `no GitHub authentication available` | export `GITHUB_TOKEN` or `GH_TOKEN` |
| `commit changelog` / `push main` failed | the CLI stages, commits, and pushes `CHANGELOG.md` for you — check write access to the release branch (branch protection can block a direct push) |

Use `go run ./cmd/release validate` (or any `--dry-run` command) to diagnose
without making changes — it prints the full severity report. Warnings (e.g. a
missing token) don't fail a dry-run; only ✗ errors do.

> **Initial commit:** the repository's root commit is **excluded** from
> Conventional Commit validation, so a non-conventional `first commit` never
> fails — no history rewrite needed. Only commits after the root (and after the
> latest tag, once one exists) are validated.

## 11. Migration guide

Adopting this on an existing repository:

1. **Start committing** with Conventional Commits (a `commit-msg` git hook
   already validates messages — see [`scripts/hooks/validate-commit.sh`](../scripts/hooks/validate-commit.sh)).
2. **Seed the changelog**: `CHANGELOG.md` ships with just the header; the first
   `release` populates it.
3. **Establish a baseline tag** if the repo already has releases: create the
   current version as an annotated tag so the CLI computes the *next* version
   from it, e.g. `git tag -a v1.4.2 -m "v1.4.2" && git push origin v1.4.2`.
   Without any tag, the first `release` uses `initial_version` (`0.1.0`).
4. **Dry-run first**: `go run ./cmd/release --dry-run --no-verify` to preview the
   version, CHANGELOG, and notes before publishing.
5. **Cut the release**: on `main` with a clean tree and `GITHUB_TOKEN` set, run
   `go run ./cmd/release <bump>`. The CLI commits and pushes the updated
   `CHANGELOG.md` to `main` for you, then tags and publishes — no manual commit
   needed. Other clones sync with `git pull`.

No existing functionality changes — the release tooling is additive and lives
entirely in `cmd/release` + `internal/{semver,conventional,changelog,release}`.
