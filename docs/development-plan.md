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
| 3 | **Repository registration** — validate repo + PAT, create webhook, store metadata + secret | ⏳ Planned |
| 4 | **Webhook receiver** — signature verification + commit-message trigger validation | ⏳ Planned |
| 5 | **Event processing** — matched event → EventBridge → SQS + Spot start (n8n stubbed) | ⏳ Planned |
| 6 | **Infrastructure lifecycle** — Spot start, health/readiness, n8n invoke, idle shutdown, retries | ⏳ Planned |
| 7 | **Repository processing** — placeholder clone / README / docs / commit retrieval | ⏳ Planned |

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
