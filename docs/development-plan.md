# Development Plan

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
| **n8n invocation** | EventBridge → SQS buffer + `instance-starter`; n8n invoked once healthy | Preserves cold-start durability (hybrid model). |

---

## Milestones

Each milestone compiles, is independently testable, and ships as a small PR.

| # | Milestone | Status |
| --- | --- | --- |
| 1 | **Project foundation** — module, config, logging, errors, bootstrap, build tooling | ✅ Implemented |
| 2 | **Infrastructure as Code** — CloudFormation stacks (network, serverless, compute, observability) | ✅ Implemented |
| 3 | **Repository registration** — validate repo + PAT, create webhook, store metadata + secret | ✅ Implemented |
| 4 | **Webhook receiver** — signature verification + commit-message trigger validation | ✅ Implemented |
| 5 | **Event processing** — matched event → EventBridge → SQS + Spot start (n8n stubbed) | ✅ Implemented |
| 6 | **Infrastructure lifecycle** — Spot start, health/readiness, n8n invoke, idle shutdown, retries | ✅ Implemented |
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
| `serverless.yaml` | REST API Gateway (`/webhook` open, `/repositories` API-key), 4 Lambdas, per-function IAM roles, EventBridge bus + matched-event rule + idle timer, SQS events queue + DLQ, DynamoDB metadata table |
| `compute.yaml` | EC2 Spot launch template + instance, persistent gp3 EBS volume (retained), instance IAM role/profile, base-host user data |
| `observability.yaml` | CloudWatch log groups (bounded retention), SNS alarm topic, failure alarms, ops dashboard |

Validate: `make lint-cfn` (runs `cfn-lint infrastructure/*.yaml`).

**Implementation notes**

- **Spot via Launch Template.** CloudFormation's `AWS::EC2::Instance` does not
  accept `InstanceMarketOptions`; Spot options are declared on an
  `AWS::EC2::LaunchTemplate` (`SpotInstanceType: persistent`,
  `InstanceInterruptionBehavior: stop`) that the instance references. This
  matches the start/stop cost model — a persistent Spot instance can be stopped
  by `idle-shutdown` and restarted by `instance-starter`.
- **Tag-scoped start/stop.** The starter/idle Lambdas are created before the
  instance, so their IAM grants `ec2:Start/StopInstances` conditioned on
  `aws:ResourceTag/Project`, and they resolve the instance by tag rather than a
  hard-coded ID. This avoids a stack dependency cycle (serverless ⇄ compute).
- **Lambda code.** The templates reference deployment packages in an artifacts
  S3 bucket (`ArtifactsBucket` + `*CodeKey` parameters). The function code is
  implemented in Milestones 3–6; until then the stacks validate but the Lambdas
  are not yet deployable with real behaviour.

---

## Milestone 3 — Repository Registration ✅

The `registration` Lambda, built with Clean Architecture: a pure use case with
small ports, plus thin AWS/GitHub adapters. Introduces `aws-lambda-go` and the
AWS SDK v2 (which raise the module's minimum Go to 1.24).

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
that publishes `blog.publish.requested` and starts the Spot instance.

---

## Milestone 5 — Event Processing ✅

The matched event now flows all the way to compute:
**webhook-handler → EventBridge → (SQS buffer + instance-starter → EC2 Spot start)**.

| Package | Responsibility |
| --- | --- |
| `internal/eventbus` | `EventBridgePublisher` — `PutEvents` of `blog.publish.requested`; swapped into the webhook handler in place of `LogPublisher`. |
| `internal/lifecycle` | `Starter.EnsureRunning` — start the instance unless already running (idempotent), located by Project tag. |
| `internal/awsec2` | EC2 adapter (`DescribeInstances` by tag, `StartInstances`). |
| `lambdas/instance-starter` | EventBridge-invoked Lambda that ensures the Spot host is running. |

Config gains `PROJECT_NAME` and `EVENT_SOURCE`; the serverless template passes
`EVENT_SOURCE=<project>.webhook` to the handler (matching the rule pattern).

**Stub:** the instance-starter ensures the host is running but does **not** yet
invoke the n8n workflow — that (and readiness detection + idle shutdown) is
Milestone 6. The event payload is available to the starter for that step.

---

## Milestone 6 — Infrastructure Lifecycle ✅

Completes the cost-optimised compute loop with automatic shutdown.

| Package | Responsibility |
| --- | --- |
| `internal/lifecycle` | `Shutdowner.StopIfIdle` — stop the instance once it has been up beyond the idle timeout **and** the queue is drained (visible + in-flight == 0). Idempotent. `EC2` port extended with `StopInstance` and an `Instance` value (id/state/launch time). |
| `internal/awssqs` | SQS adapter reporting queue depth (visible + not-visible). |
| `internal/awsec2` | Extended with `StopInstances` and launch-time. |
| `lambdas/idle-shutdown` | Scheduled Lambda (EventBridge idle timer) that runs `StopIfIdle`. |

**Readiness & n8n invocation (design decision).** In the hybrid model, n8n
**pulls** work from SQS (its SQS-trigger workflow) rather than being pushed an
HTTP call. So there is no Lambda-side n8n invoke or readiness probe: EventBridge
buffers the matched event in SQS, the instance-starter brings the host up, and
n8n drains SQS via long-polling once its container is healthy. This keeps n8n
unexposed (no inbound) and needs no Lambda-in-VPC networking. The n8n
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
routing, on-demand Spot compute, automatic shutdown, and the processing seam —
all AWS-native, IaC-provisioned, and unit-tested. The next phase is Repository
Intelligence and content generation on top of the `processing.Snapshot`.
