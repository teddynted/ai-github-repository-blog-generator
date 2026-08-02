# Development Plan

> **Historical.** This plan records the incremental build, including the earlier
> local-Ollama / GitHub-webhook design. The platform has since migrated to
> `POST /process` → **AWS Step Functions** → **AI Provider Router** (Amazon
> Bedrock → Anthropic) with S3 idempotency, on a t4g.small running n8n +
> PostgreSQL + Redis. For the current architecture see [Architecture](./architecture.md),
> [AI Provider Router](./hybrid-ai-routing.md), and [Infrastructure](./infrastructure.md).
> The milestone history below is kept for traceability.

This document tracks the incremental implementation of the MVP against the
[Requirements](./requirements.md) and [Roadmap](./roadmap.md). The documentation
is the single source of truth; this plan records **how** the MVP is being built
and **what is implemented so far**.

Principles: documentation-first, AWS-first, event-driven, serverless where
appropriate, cost-optimised, secure by design, Clean Architecture / SOLID,
small PR-sized changes, Infrastructure as Code.

---

## Implementation Decisions

These resolve ambiguities in the docs; they are deliberate and documented here
so behaviour is never undocumented.

| Decision | Choice | Rationale |
| --- | --- | --- |
| **IaC tool** | **AWS CloudFormation** (no Terraform) | Mandated throughout the docs; the platform is AWS-first. |
| **Go module layout** | **Single monorepo module** | One `go.mod` at the root; shared code in `internal/`; each Lambda is `lambdas/<fn>/` built with `go build ./lambdas/<fn>`. Maximises reuse (config, logging, signature, GitHub client) and suits Clean Architecture. |
| **Lambda runtime** | `provided.al2023`, **Linux/arm64** (Graviton) | Documented; best price/performance. |
| **Registration auth (MVP)** | **API Gateway API key** (`x-api-key`) | Simplest way to satisfy REG-9 "access-controlled"; revisited if GitHub App onboarding lands. |
| **Instance power** | EventBridge → SQS buffer; **EventBridge Scheduler** starts/stops the On-Demand host on a daily window; the worker drains SQS while up | The schedule owns availability; SQS keeps events durable between windows. |

---

## Milestones

Each milestone compiles, is independently testable, and ships as a small PR.

| # | Milestone | Status |
| --- | --- | --- |
| 1 | **Project foundation** — module, config, logging, errors, bootstrap, build tooling | ✅ Implemented |
| 2 | **Infrastructure as Code** — CloudFormation stacks (network, serverless, compute, observability) | ✅ Implemented |
| 3 | **Repository registration** — validate repo + PAT, create webhook, store metadata + secret | ✅ Implemented |
| 4 | **Webhook receiver** — signature verification + commit-message trigger validation | ✅ Implemented |
| 5 | **Event processing** — matched event → EventBridge → SQS buffer (n8n stubbed) | ✅ Implemented |
| 6 | **Infrastructure lifecycle** — scheduled On-Demand start/stop (EventBridge Scheduler), health/readiness, retries | ✅ Implemented |
| 7 | **Repository processing** — placeholder clone / README / docs / commit retrieval | ✅ Implemented |

Out of scope for the MVP (see [Roadmap](./roadmap.md)): GitHub Apps, multi-user,
SaaS dashboard, billing, analytics, advanced AI agents, multi-platform
publishing, enterprise functionality.

---

## Milestone 1 — Project Foundation ✅

A single Go module with the shared building blocks every Lambda depends on.

| Package | Responsibility |
| --- | --- |
| `internal/config` | Environment-driven configuration with defaults, validation, and a `Require` check. Injectable `Getenv` for testability. |
| `internal/logging` | Structured JSON logging (`log/slog`), run-ID correlation, and secret redaction. |
| `internal/apperror` | Typed errors with stable codes and HTTP-status mapping; composes with `errors.Is/As`. |
| `internal/app` | Bootstrap container wiring config + logger for a Lambda entry point. |

Tooling: `Makefile` (`fmt`, `vet`, `test`, `check`, `build`, `tidy`), `.gitignore`,
`.env.example`. Build/test:

```bash
make check          # gofmt check + go vet + go test -race -cover
make build          # compile each implemented Lambda to dist/<fn>/bootstrap (Linux/arm64)
```

No external dependencies yet — the foundation is standard-library only. AWS SDK
and `aws-lambda-go` arrive with Milestones 3–4.

---

## Milestone 2 — Infrastructure as Code ✅

Four modular, `cfn-lint`-clean CloudFormation templates under `infrastructure/`.
Deploy order: **network → serverless → compute → observability**.

| Stack | Provisions |
| --- | --- |
| `network.yaml` | VPC, public subnet, Internet Gateway, route table, instance security group (SSH from operator CIDR only) |
| `serverless.yaml` | REST API Gateway (`/webhook` open, `/repositories` API-key), 2 Lambdas (registration, webhook-handler), per-function IAM roles, EventBridge bus + matched-event rule, SQS events queue + DLQ, DynamoDB metadata table |
| `compute.yaml` | On-Demand EC2 launch template + instance, persistent gp3 EBS volume (retained), instance IAM role/profile, base-host user data |
| `scheduler.yaml` | EventBridge Scheduler daily start/stop schedules + `scheduled-start`/`scheduled-stop` Lambdas + least-privilege roles (targets the instance by ID) |
| `observability.yaml` | CloudWatch log groups (bounded retention), SNS alarm topic, failure alarms, ops dashboard |

Validate: `make lint-cfn` (runs `cfn-lint infrastructure/*.yaml`).

**Implementation notes**

- **On-Demand via Launch Template.** A launch template centralizes the instance
  configuration (type, AMI, key, profile, security group, root volume, user
  data) that the `AWS::EC2::Instance` references. There are no
  `InstanceMarketOptions`: it is an On-Demand instance, so a scheduled or manual
  start always succeeds without depending on Spot capacity. Instance power is
  owned by `scheduler.yaml` (start 18:00 / stop 20:00, daily).
- **ID-scoped start/stop.** The scheduler's `scheduled-start`/`scheduled-stop`
  Lambdas take the compute stack's `InstanceId` and grant
  `ec2:Start/StopInstances` scoped to that single instance ARN. The webhook
  handler keeps a read-only tag-based lookup (`ec2:DescribeInstances`) to report
  processed-now vs deferred, but never starts or stops the host.
- **Lambda code.** The templates reference deployment packages in an artifacts
  S3 bucket (`ArtifactsBucket` + `*CodeKey` parameters). The function code is
  implemented in Milestones 3–6; until then the stacks validate but the Lambdas
  are not yet deployable with real behaviour.

---

## Milestone 3 — Repository Registration ✅

The `registration` Lambda, built with Clean Architecture: a pure use case with
small ports, plus thin AWS/GitHub adapters. Introduces `aws-lambda-go` and the
AWS SDK v2 (which raise the module's minimum Go to 1.25; the build toolchain is
pinned in `go.mod` to a patched release).

| Package | Responsibility |
| --- | --- |
| `internal/repo` | Repository domain model + `ParseRepositoryURL` (HTTPS + SSH forms). |
| `internal/github` | Minimal GitHub REST client (`GetRepository`, `CreateWebhook`) over `net/http`, status→typed-error mapping. |
| `internal/registration` | The onboarding use case + ports (`GitHub`, `SecretStore`, `MetadataStore`) and the JSON handler. |
| `internal/secrets` | Secrets Manager adapter — stores PAT + webhook secret, returns only a reference. |
| `internal/metadata` | DynamoDB adapter — persists repository metadata (never the PAT). |
| `lambdas/registration` | API Gateway entry point wiring the collaborators. |

Onboarding order guarantees no half-registered repo is visible: validate access
→ create webhook → store credentials → write metadata. The PAT is never logged
and never written to DynamoDB (guarded by a test). All logic is unit-tested with
fakes/`httptest`; the Lambda cross-compiles to `linux/arm64`.

**Decision:** the registration endpoint is protected by an **API Gateway API
key** (`x-api-key`); the key ID is a stack output and its value is retrieved
from API Gateway after deploy.

---

## Milestone 4 — Webhook Receiver ✅

The `webhook-handler` Lambda — deliberately lightweight, no cloning/analysis/AI,
and it never reads the PAT.

| Package | Responsibility |
| --- | --- |
| `internal/trigger` | Commit-message trigger matching (prefix, default `blog:`). 100% covered. |
| `internal/githubsig` | HMAC-SHA256 signature verify/sign, constant-time; GitHub known-answer test. |
| `internal/webhook` | The handler: parse payload → resolve metadata → verify signature → evaluate trigger → publish (via port). |
| `internal/eventbus` | `LogPublisher` — a placeholder Publisher for M4 (real EventBridge adapter in M5). |
| `lambdas/webhook-handler` | API Gateway entry point (handles base64 bodies). |
| `internal/metadata`, `internal/secrets` | Extended with `Get` and `WebhookSecret` read paths. |

Request flow and outcomes: invalid signature → `401`; unregistered repo → `404`;
non-`push`/disabled/no-match → `200 ignored`; matched `blog:` commit → publish
+ `200 accepted`. The per-repository **Trigger Pattern** from metadata is used
(falling back to the platform default). Every branch is unit-tested with fakes.

**Placeholder:** on a match the handler calls the `Publisher` port, which is
wired to `LogPublisher` for now. Milestone 5 swaps in the EventBridge adapter
that publishes `blog.publish.requested` (buffered in SQS for the scheduled
window).

---

## Milestone 5 — Event Processing ✅

The matched event now flows to the durable buffer:
**webhook-handler → EventBridge → SQS buffer** (drained by the worker during the scheduled window).

| Package | Responsibility |
| --- | --- |
| `internal/eventbus` | `EventBridgePublisher` — `PutEvents` of `blog.publish.requested`; swapped into the webhook handler in place of `LogPublisher`. |
| `internal/lifecycle` | `Instance` type + `EC2` port (`FindInstance` by Project tag) — reused by the webhook window gate. |
| `internal/awsec2` | EC2 adapter (`DescribeInstances` by tag/ID, `Start`/`StopInstances`). |
| `lambdas/webhook-handler` | Reports `accepted` (instance running, in-window) vs `deferred` (stopped) via the read-only window gate; never starts the host. |

Config gains `PROJECT_NAME` and `EVENT_SOURCE`; the serverless template passes
`EVENT_SOURCE=<project>.webhook` to the handler (matching the rule pattern) and
`PROJECT_NAME` for the window gate.

> **Migration note.** Earlier revisions started a Spot instance per matched event
> via an `instance-starter` Lambda. That was replaced by On-Demand compute on a
> fixed daily schedule (Milestone 6); the webhook no longer starts the host.

---

## Milestone 6 — Infrastructure Lifecycle ✅

Completes the cost-optimised compute loop with a scheduled daily window.

| Package | Responsibility |
| --- | --- |
| `internal/power` | `Switch.EnsureStarted`/`EnsureStopped` — start/stop a specific instance by ID unless already in the target state. Idempotent, inspects current state first. |
| `internal/awssqs` | SQS adapter: receive/delete for the worker, plus `Depth` (visible + not-visible) for operational visibility. |
| `internal/awsec2` | `InstanceState`/`Start`/`StopInstances` by ID (for the scheduler) and `FindInstance` by tag (for the window gate). |
| `lambdas/scheduled-start`, `lambdas/scheduled-stop` | EventBridge Scheduler-invoked Lambdas that power the On-Demand host on/off at 18:00/20:00 daily (`scheduler.yaml`). |

**Readiness & n8n invocation (design decision).** In the hybrid model, n8n
**pulls** work from SQS (its SQS-trigger workflow) rather than being pushed an
HTTP call. So there is no Lambda-side n8n invoke or readiness probe: EventBridge
buffers the matched event in SQS, EventBridge Scheduler brings the host up at
18:00, and n8n drains SQS via long-polling once its container is healthy. This
keeps n8n unexposed (no inbound) and needs no Lambda-in-VPC networking. The n8n
SQS-trigger workflow + Docker Compose bring-up live on the instance and are part
of the instance configuration (`instance/`, Milestone 7).

**Retries.** Run retries come free from SQS visibility timeout + DLQ; the
scheduled idle check and instance start/stop are idempotent, so EventBridge/
Lambda retries are safe.

All four Lambdas now build; `make check` (`-race`) is green.

---

## Milestone 7 — Repository Processing ✅

The processing seam where Repository Intelligence will plug in — **placeholders
only**, no analysis or generation.

| Package / file | Responsibility |
| --- | --- |
| `internal/processing` | Ports (`Cloner`, `ReadmeRetriever`, `DocsRetriever`, `CommitRetriever`) + `Processor` that gathers a `Snapshot` (clone → README → docs → commits). |
| `internal/processing` (placeholder.go) | Canned implementations returning representative data; `NewPlaceholderProcessor` wires them. |
| `instance/docker-compose.yml` | Instance runtime: n8n + Ollama (state/models on `/data` = the EBS volume). OpenClaw is a documented placeholder service. |

The `Snapshot` is the fixed input contract for Repository Intelligence: swapping
the placeholders for real (e.g. OpenClaw-backed) retrievers requires **no change**
to the `Processor` or its ports. Every path is unit-tested.

**Out of scope (future roadmap):** Repository Intelligence (analysis), Repository
Memory population, AI generation, quality review, publishing, and multi-platform
output — all deliberately deferred, with the seams in place.

---

## MVP status

Milestones 1–7 are complete: an opt-in, event-driven, cost-optimised vertical
slice from repository registration through the commit-trigger gate, event
routing, scheduled On-Demand compute (daily window), and the processing seam —
all AWS-native, IaC-provisioned, and unit-tested. The next phase is Repository
Intelligence and content generation on top of the `processing.Snapshot`.

## Testing

Every package is unit-tested (ports exercised with fakes; HTTP clients with
`httptest`; git operations against real local repos via go-git). An **end-to-end
integration test** (`internal/pipeline`) wires the real adapters — filesystem
retrieval, analysis, quality review, file publish, and Repository Memory —
through the pipeline with only the LLM stubbed, and verifies the memory-based
skip on a repeat commit. Run with `make check` (`-race`).

## Continuous integration

GitHub Actions under `.github/workflows/` run on every push/PR and are green
without repository secrets:

- `go.yml` — gofmt, `go vet`, `go test -race -cover`, and cross-compiles all
  four Lambdas for `linux/arm64`.
- `cloudformation.yml` — `cfn-lint` on all templates.
- `security.yml` — `govulncheck` + `gosec` (SAST, high-severity gate) +
  `gitleaks` (secret scan with a placeholder allowlist); `checkov` runs
  informationally on the CloudFormation.
- `deploy.yml` — on merge to `main`, packages the Lambdas (SHA-versioned keys)
  and deploys network → serverless → compute → observability via OIDC. **Opt-in**:
  dormant until `DEPLOY_ENABLED=true` and the AWS role/vars are set
  (see [CI/CD → Enabling deploy.yml](./ci-cd.md#enabling-deployyml)).

CI/CD is now complete end to end; deploying just needs the AWS side configured.

### Deployment enablement

`infrastructure/bootstrap.yaml` + `scripts/bootstrap.sh` provision, once with
admin credentials, the pieces the deploy path needs: the **Lambda artifacts S3
bucket**, the **GitHub Actions OIDC provider** (optional), and the **deploy
role** the workflow assumes. The script prints the exact repository
secrets/variables to set (`AWS_DEPLOY_ROLE_ARN`, `ARTIFACTS_BUCKET`, …,
`DEPLOY_ENABLED=true`); after that, a merge to `main` deploys the app stacks.
Runbook: [Deployment → First-Time Bootstrap](./deployment.md#first-time-bootstrap-automated-deploy).

**Remaining to actually run in AWS (your side):** an account and On-Demand quota
for a GPU instance type (the key pair is stack-managed) — then bootstrap → set
the vars → deploy → register a repo → push a `blog:` commit.

---

## Hardening (correctness vs. requirements)

- **Retry with backoff (FR-6.1).** `internal/retry` — bounded exponential
  backoff + jitter, retrying only transient (`upstream`/`unavailable`) errors and
  respecting context cancellation. Wired into the Ollama and GitHub clients
  (401/403/404 still fail fast); AWS SDK calls rely on the SDK's own retry.
- **Prompt/context budget (AI-6).** `generation.Generator.MaxPromptBytes`
  (default 24 KB) deterministically truncates the repository context so a large
  repo cannot overflow the model window; instructions and the closing directive
  are preserved, and a truncation marker is appended.
- **CloudWatch custom metrics (MON-2).** `internal/metrics` emits
  Embedded-Metric-Format (EMF) records to stdout — no `PutMetricData`, no extra
  IAM, no latency. The webhook handler emits `WebhookReceived` / `WebhookRejected`
  / `WebhookIgnored` / `TriggerMatched`; the worker emits `RunsStarted` /
  `RunsSucceeded` / `RunsFailed` / `RunsHeld` / `RunsSkipped` / `AssetsGenerated`.
  The `observability.yaml` dashboard gained two `BlogGenerator`-namespace widgets
  and a `RunsFailed` alarm.
- **Deeper analysis (FR-2.3–2.5).** `reposource.FSAnalyzer` detects the
  technical profile from the working copy — languages, dependency managers, IaC
  (Terraform, CloudFormation), containers (Docker/Compose), and CI/CD (GitHub
  Actions, GitLab CI, …). It is added to the `processing.Snapshot` and rendered
  into every generation prompt as a "Technical profile" section, so content is
  grounded in the real stack rather than just the README.

## Phase 2 (post-MVP) — Content Generation

Beyond the MVP slice, building on `processing.Snapshot`. In progress.

| Package | Responsibility | Status |
| --- | --- | --- |
| `internal/ollama` | Local LLM client (`/api/generate`, non-streaming) — realises "local inference via Ollama" with real, testable code. No external/paid API. | ✅ Implemented |
| `internal/generation` | `Generator.Generate`/`GenerateAll` — build deterministic prompts from a `Snapshot` and produce Markdown per kind via the `Model` port (Ollama satisfies it). Kinds: blog, README improvements, documentation, architecture summary, release notes. `GenerateAll` continues past a per-kind failure and aggregates errors. | ✅ Implemented |

**Scope note.** Five content outputs are implemented. Repository Memory
population, quality review, optional human approval, and publishing are still
future work — the ports keep them additive, and `GenerateAll` never lets one
failing kind corrupt the rest of the package.

| `internal/pipeline` | `Pipeline.Run` — composes process → generate → publish for a request. Resilient: publishes the assets that succeeded even on a partial generation failure. | ✅ Implemented |
| `internal/publish` | `FilePublisher` writes each asset to `<dir>/<owner>/<name>/<YYYY-MM-DD>/<kind>.md` (path-traversal guarded); `LogPublisher` remains for logging. The worker uses `FilePublisher` (`OUTPUT_DIR`, default `/data/generated-content` on the EBS volume). Remote destinations (Git / object store / CMS) can follow behind the same port. | ✅ Implemented |
| `internal/awssqs` | Extended with `Receive`/`Delete` (message consumption) alongside depth. | ✅ Implemented |
| `cmd/worker` | Instance worker: long-polls SQS → `Pipeline.Run` → deletes on success (leaves failures for SQS redelivery/DLQ). Message-handling logic is unit-tested. | ✅ Implemented |

**Runtime decision.** The MVP runtime is a **Go worker** (`cmd/worker`) draining
SQS and invoking `pipeline.Pipeline`. It was chosen over hand-authored n8n
workflow JSON because it is fully unit-testable and keeps the whole slice in one
verifiable codebase. The documented **n8n** SQS-trigger design remains a valid
alternative orchestration for the same seams (it would call the same
composition, e.g. via an exec node) and a future visual-orchestration option;
this worker does not remove that path. Build it with `make build-worker`
(Linux/amd64 for the g4dn host).

### Real repository processing ✅

`internal/reposource` replaces the placeholder processor with real
implementations of the processing ports:

| Type | Responsibility |
| --- | --- |
| `GitCloner` | Shallow, single-branch clone via **go-git** (no external git binary); token passed via `Auth`, never in the URL; per-repo work dir replaced each run. |
| `FSReadme` / `FSDocs` | Read the README and `docs/*.md` from the working copy (size- and count-capped). |
| `GitCommits` | Read recent commits (SHA, subject, author) from the clone. |
| `MetaTokenSource` | Resolve the repo's PAT: metadata `Get` → `secret_ref` → Secrets Manager `PAT`. |

The worker now wires this real processor (metadata + secrets clients,
`WORK_DIR` default `/data/work`), so a run clones the real repository, reads its
content, generates the package via Ollama, and writes Markdown files. Tested
with go-git against a real local repo (clone, README/docs retrieval, commit log)
and fakes for the token source.

### Repository Memory ✅

`internal/memory` is a filesystem-backed, per-repository record of published
commits (persisted under `MEMORY_DIR`, default `/data/memory` on the EBS volume
— not a managed DB, per the requirements; never stores secrets). The pipeline
now consults it: a matched event whose commit was **already published is skipped**
(`Result.Skipped`), and a completed run **records** the commit + kinds. Memory is
an optional port and a read failure never blocks a run. Tested (record → dedup,
per-repo isolation, idempotent per commit) plus pipeline behaviour (skip, record,
proceed-on-error).

### Quality review & optional human approval ✅

Two more pipeline stages, both optional ports:

| Package | Responsibility |
| --- | --- |
| `internal/review` | Rule-based `Reviewer`: each asset must be non-empty, meet a minimum length, and contain a Markdown heading. Only passing assets are published; failures are reported. A second local-model pass is a future enhancement behind the same shape. |
| `internal/approval` | `HoldForReview`: when `REQUIRE_HUMAN_APPROVAL=true`, generated content is stashed under `PENDING_DIR` (default `/data/pending`) and the gate returns not-approved, so the pipeline **holds** (`Result.Held`) without publishing or recording memory — a later approved run proceeds. |

Pipeline order is now **process → generate → review → approve → publish → record memory**. The worker always runs review and enables the approval gate from config. Tested: review pass/fail split, held-not-published-nor-recorded, approved-publishes.

### Notifications ✅

`internal/notify` delivers run outcomes (`published` / `held` / `failed`). The
worker notifies after each run: failure (message retained for SQS retry), held
(approval pending), or published. Channels behind the `Notifier` port:
`LogNotifier` (always on) and `WebhookNotifier` — a Slack-compatible (`text` +
structured fields) HTTP POST, retried on transient failure, enabled by setting
`NOTIFY_WEBHOOK_URL`. `Multi` fans out to both. Email is future.

**Functional MVP complete.** Every documented functional requirement and
pipeline stage now has real, tested code.

Publishing destinations behind the `Publisher` port: `FilePublisher` (default,
to `/data` on the EBS volume) and `S3Publisher` — set `OUTPUT_S3_BUCKET` to
publish to `s3://<bucket>/<prefix>/<owner>/<name>/<date>/<kind>.md` instead. The
compute stack grants the instance `s3:PutObject` on that bucket only when
`OutputS3Bucket` is set.

**Approvals dashboard.** When `REQUIRE_HUMAN_APPROVAL` holds content under
`PENDING_DIR`, the `approve` CLI (`cmd/approve`, built with the worker) is the
review queue: `approve` lists pending packages, `approve -all` auto-approves
(publishes) everything to the live destination and clears the queue, and
`approve -reject-all` discards. `internal/approval.PendingStore` is unit-tested.

Notification channels: `LogNotifier` (always on), `WebhookNotifier`
(Slack/webhook, `NOTIFY_WEBHOOK_URL`), and `EmailNotifier` over **SMTP (Turbo
SMTP)**, enabled by `NOTIFY_EMAIL_FROM` + `NOTIFY_EMAIL_TO` plus `SMTP_USERNAME`
+ `SMTP_PASSWORD` (host/port default to `pro.turbo-smtp.com:587`, STARTTLS);
`Multi` fans out to all configured channels. `SMTPClient` supports STARTTLS
(587) and implicit TLS (465), authenticates with PLAIN over an encrypted
connection, and guards against header injection. Because delivery is outbound
SMTP, **no AWS mail IAM is required** — the instance just needs egress on
587/465, and the SMTP password supplied at runtime (env, or Secrets Manager at
deploy time).

### Worker runtime service ✅

The compute stack installs the worker as a **systemd service**
(`blog-gen-worker`) on the instance: `deploy.yml` builds the linux/amd64 binary
and uploads it to the artifacts bucket (`<sha>/worker`); the instance downloads
it, writes `/etc/blog-gen/worker.env` (0600) from stack parameters plus the
imported `QueueUrl`/table, and runs it with `Restart=always`. Email credentials
follow the no-plaintext principle: the stack creates a `NotificationsSecret`
(`${ProjectName}/notifications/smtp-password`) whose value the operator sets
out-of-band with `put-secret-value`; the worker resolves it via
`SMTP_PASSWORD_SECRET` at startup (`secrets.Store.Value`), so the password never
touches env files, CloudFormation parameters, or stack history. The instance
role grants `secretsmanager:GetSecretValue` on that secret and `s3:GetObject` on
the artifacts bucket.

### Ollama model serving ✅

The compute UserData now provisions **Ollama** — the local inference the worker
calls — completing the on-instance pipeline. When `EnableGpu=true` (default) it
installs the NVIDIA driver + container toolkit and runs the `ollama/ollama`
container with `--gpus all`; if the GPU start fails it falls back to CPU, and
`EnableGpu=false` forces CPU (useful for testing on a non-GPU instance). Models
are pulled once (`OllamaModel`, default `qwen2.5:7b`) and **persist on the gp3
volume** (`/data/ollama`), so they survive the daily stop/start; the API binds to
`127.0.0.1:11434` only. The worker unit gains an `ExecStartPre` readiness wait so
it does not start generating before Ollama answers. This makes the full path —
webhook → SQS → worker → Ollama → review → publish/notify — deployable on one
instance. (n8n as an alternative orchestrator remains optional/future.)

### Instance startup optimization (custom AMI) ✅

Startup still matters (the instance is stopped outside its window and started at
18:00, so a faster boot means more of the window is usable), so the slow,
network-heavy setup — NVIDIA driver, Docker, the Ollama image, and the
~4.7 GB model — is **pre-baked into a custom AMI** instead of run every launch.
One canonical script, [`scripts/ami/provision.sh`](../scripts/ami/provision.sh),
is both baked by Packer ([`packer/worker/blog-gen.pkr.hcl`](../packer/worker/blog-gen.pkr.hcl),
via `scripts/build-ami.sh`) and run at boot as the stock-AMI fallback, so the two
never drift. The model is baked as a seed and **copied** to `/data` at first boot
(no download). UserData shrank to runtime-only work (mount, seed, write env,
fetch the small worker binary, start services). Ollama and the worker are now
**systemd units** (`blog-gen-ollama`, `blog-gen-worker`) that auto-start on every
boot; `start-ollama.sh` detects the GPU at runtime (one AMI runs GPU or CPU), the
worker's `ExecStartPre` waits for the model, and `health.sh` gates readiness. The
compute stack gained `CustomAmi` (fast path) and `AmiScriptsKey` (fallback)
parameters; `deploy.yml` uploads `provision.sh` and passes the AMI id read from
SSM (`/blog-gen/worker-ami`). Result:
time-to-ready drops from ~10–15 min to well under a minute, with no change to the
cost model. See README → **Optimizing Instance Startup** and
[docs/ami.md](./ami.md).

**Trigger sources.** Registration subscribes the webhook to `push` and
`release`. The handler branches by event type: a `push` is commit-message gated
(default `blog:` or the per-repo pattern); a **published `release`** always
triggers (a release is itself an intentional event) with the tag as the memory
dedup key. Git tags and PR labels remain future.

### Architecture diagrams ✅

`internal/archdiagram` turns a `processing.Snapshot` into evidence-grounded AWS
architecture diagrams (Mermaid) that embed straight into the blog. It is
**deterministic** — no LLM guessing — which directly serves the module's primary
principle (repository evidence over assumption): every service carries the file +
token that produced it, and the Mermaid is valid **by construction** (built as a
`Graph` model, then `Validate`d for dangling edges / orphans / dup ids before
render). Detection scans the working copy for Terraform (`aws_*`),
CloudFormation (`AWS::*`), AWS SDK client packages, dependency hints
(Postgres → RDS, Redis → ElastiCache, Medium), and docker-compose/K8s manifests;
it maps LLM usage to **OpenClaw on EC2, never Bedrock**, and defaults compute to
EC2 (On-Demand) only as explicit deployment context. Confidence is High / Medium /
Low; only High + Medium reach the primary diagram (omission over speculation).
It emits up to six diagrams — AWS Solution (primary), Component, Data Flow,
Deployment, CI/CD (if CI/CD), AI Workflow (if AI) — plus a confidence report and
an evidence report. Wired into the pipeline as the optional `Diagrammer` seam
(after generation, before review), so the diagram asset flows through review,
approval, publishing, and memory like any other. The worker enables it by
default; a diagram failure never fails the run.

**Still future (roadmap, not blocking the MVP).** OpenClaw-based deeper
analysis; Git-tag / PR-label trigger sources; GitHub Apps auth; a **web**
approvals UI; and the n8n workflow as an alternative orchestration. Real
deployment + end-to-end validation require an AWS account and a GPU instance.
