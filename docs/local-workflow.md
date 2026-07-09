# Local Development Workflow

Versioned Git hooks and an [`act`](https://nektosact.com) integration that mirror
GitHub Actions locally. The goal is a mature-open-source developer experience:
**fast commits, fail fast, clear messages, minimal manual intervention, fewer red
CI builds.** The hooks *complement* GitHub Actions — **CI remains the source of
truth.**

## Contents

- [Design](#design)
- [Repository layout](#repository-layout)
- [Installation](#installation)
- [The hooks](#the-hooks)
- [act integration](#act-integration)
- [GitHub Actions alignment](#github-actions-alignment)
- [Configuration & escape hatches](#configuration--escape-hatches)
- [Troubleshooting](#troubleshooting)
- [Future enhancements](#future-enhancements)

## Design

- **Keep commits fast.** `pre-commit` runs only lightweight Go checks — no
  Docker, no `act`, no GitHub Actions. Heavy, container-based CI emulation is
  deferred to `pre-push`.
- **Fail fast, clearly.** Every script prints `✓` / `✗` with an actionable
  message; no cryptic shell errors.
- **One source of truth.** Hooks *and* CI call the same scripts in
  `scripts/hooks/`, so local and CI logic cannot drift.
- **Lightweight hooks.** The files in `.githooks/` are thin wrappers; all logic
  lives in reusable, testable scripts under `scripts/hooks/`.
- **Contributor-friendly.** Missing `act`/Docker never hard-blocks a push
  (opt-in strict mode exists). Standard `--no-verify` escape hatches are
  documented.

## Repository layout

```
.githooks/              # thin hook wrappers (git core.hooksPath target)
  pre-commit            #   format · vet/lint · unit tests
  commit-msg            #   Conventional Commits validation
  pre-push              #   act runs the primary CI workflow
scripts/
  install-hooks.sh      # sets core.hooksPath = .githooks
  hooks/
    common.sh           # shared output/detection helpers (sourced)
    format.sh           # gofmt (+ goimports); --check | --staged | apply
    lint.sh             # go vet (+ golangci-lint if installed)
    tests.sh            # go test ./... (GO_TEST_FLAGS for -race -cover)
    validate-commit.sh  # Conventional Commits check
    run-act.sh          # runs act with prerequisite detection
.actrc                  # pins the act runner image
```

## Installation

```bash
make hooks          # or: ./scripts/install-hooks.sh
```

This runs `git config core.hooksPath .githooks`. Because the hooks are versioned,
they update automatically when you pull. Uninstall with:

```bash
git config --unset core.hooksPath
```

## The hooks

### pre-commit (fast, no Docker)

1. **`format.sh --staged`** — `gofmt` (and `goimports` if installed) on your
   *staged* Go files, then re-stages them. If a staged file also has *unstaged*
   changes, the hook stops rather than silently commit partial work.
2. **`lint.sh`** — `go vet ./...`, plus `golangci-lint run` when installed.
3. **`tests.sh`** — `go test ./...` (fast; no `-race` here to keep commits quick).

Any failure aborts the commit with a clear message. Bypass in an emergency with
`git commit --no-verify`.

### commit-msg (Conventional Commits)

Validates the subject line against
[Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <description>
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`,
`ci`, `chore`, `revert`. Append `!` for a breaking change
(`feat(api)!: …`). Merge/revert/fixup/squash commits pass through.

| ✓ Valid | ✗ Invalid |
| --- | --- |
| `feat(webhook): implement GitHub webhook validation` | `updated stuff` |
| `fix(lambda): handle invalid GitHub signature` | `WIP` |
| `docs(readme): update architecture` | `Fixed bug` |
| `ci(actions): improve workflow caching` | `feature: add x` (unknown type) |
| `chore(deps): update dependencies` | `feat add x` (missing colon) |

### pre-push (act CI mirror)

Runs the primary workflow (`.github/workflows/go.yml`) with `act`. On failure,
the push is **aborted** and the failing job is shown; on success, the push
proceeds. See below for how missing prerequisites are handled.

## act integration

[`act`](https://nektosact.com) executes GitHub Actions workflows locally inside
Docker containers.

**Requirements**

| Requirement | Install |
| --- | --- |
| Docker (running daemon) | [Docker Desktop](https://docs.docker.com/get-docker/) or Docker Engine |
| `act` | macOS: `brew install act` · Linux: `curl -fsSL https://raw.githubusercontent.com/nektos/act/master/install.sh \| sudo bash` |
| OS | Linux or macOS (Windows: use **WSL2**) |

**Recommended runner image** — pinned in `.actrc`:

```
-P ubuntu-latest=catthehacker/ubuntu:act-latest
```

This is act's "medium" image (Go toolchain + git + common tooling) — a balance
between the tiny default image (missing tools) and the multi-GB full image.

**Supported workflows.** `pre-push`/`make act` target `go.yml` (build · vet ·
test · cross-compile). Others (`cloudformation.yml`, `security.yml`) can be run
on demand:

```bash
scripts/hooks/run-act.sh .github/workflows/cloudformation.yml pull_request
```

**Limitations of act** (why CI stays the source of truth): it cannot perfectly
reproduce GitHub-hosted runners — no real cloud OIDC/secrets, some actions behave
differently, `schedule`/`workflow_dispatch` need manual event selection, and the
`deploy.yml` workflow (AWS OIDC) is **not** meant to run locally.

**Graceful degradation.** `run-act.sh` detects a missing/unreachable Docker
daemon, a missing `act`, or an unsupported OS and exits with a distinct code so
`pre-push` can **warn and allow the push** instead of blocking. Require it with
`PREPUSH_STRICT=1`.

## GitHub Actions alignment

To avoid duplicated logic, `.github/workflows/go.yml` calls the **same scripts**
as the hooks:

```yaml
- name: gofmt
  run: scripts/hooks/format.sh --check
- name: vet / lint
  run: scripts/hooks/lint.sh
- name: test
  run: GO_TEST_FLAGS="-race -cover" scripts/hooks/tests.sh
```

The only intentional difference: `pre-commit` runs tests without `-race` (speed),
while CI adds `-race -cover` (thoroughness) via `GO_TEST_FLAGS`. This keeps
commits fast while CI stays rigorous — the checks themselves are identical.

## Configuration & escape hatches

| Variable / command | Effect |
| --- | --- |
| `git commit --no-verify` | Skip `pre-commit` + `commit-msg` |
| `git push --no-verify` | Skip `pre-push` |
| `SKIP_ACT=1 git push` | Push, but skip only the `act` run |
| `PREPUSH_STRICT=1` | Block the push if Docker/`act` are unavailable |
| `GO_TEST_FLAGS="-race -cover"` | Extra flags for `tests.sh` |
| `ACT_EXTRA_ARGS="--container-architecture linux/amd64"` | Extra flags for `act` |
| `GO=/path/to/go` | Use a specific Go binary |
| `NO_COLOR=1` | Disable coloured output |
| `make check` | Run fmt-check · lint · test manually |
| `make act` | Run the primary workflow via `act` manually |

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `gofmt not found` / `Go toolchain not found` | Install Go: <https://go.dev/dl/> |
| `Docker ... daemon is not reachable` | Start Docker Desktop / the Docker service |
| `act is not installed` | `brew install act` or the Linux installer above |
| First `act` run is slow | It pulls the runner image once; later runs are cached |
| Apple Silicon workflow needs amd64 | `ACT_EXTRA_ARGS="--container-architecture linux/amd64"` |
| "staged files need formatting but also have unstaged changes" | Stage or stash the unstaged edits, then commit |
| Windows | Use WSL2 (bash + Docker); native `cmd`/PowerShell is unsupported |

## Future enhancements

Documented, intentionally **not** implemented — to be adopted as the project
matures:

- **commitizen** — interactive Conventional Commit authoring (`cz commit`).
- **Automatic CHANGELOG generation** — from Conventional Commit history.
- **semantic-release** — automated version bumps and releases from commit types.
- **Release automation** — tagging and GitHub Releases in CI.
- **Commit signing verification** — enforce GPG/SSH-signed commits.
- **Secret scanning in hooks** — `gitleaks` at `pre-commit` (currently CI-only).
- **Dependency vulnerability scanning in hooks** — `govulncheck` at `pre-push`
  (currently CI-only, in `security.yml`).
- **License validation** — verify dependency license compatibility.

These are deferred so the local workflow stays fast and low-friction; each can be
layered on using the same shared-script pattern established here.
