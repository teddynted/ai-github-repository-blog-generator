# Configuration Reference

Every environment variable the platform reads, in one place. Values are injected
into each AWS Lambda + the worker by CloudFormation; for local development copy
[`.env.example`](../.env.example) to `.env`.

**Never** put GitHub PATs or the Anthropic API key here — those live only in **AWS
Secrets Manager** (see [Security](./security.md)). Config is loaded through the
injectable `config.Load(getenv)` port (`internal/config`), so nothing reads the
environment directly in business logic.

Legend: **Req?** = Required · **Def** = Default when unset.

## Core

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `AWS_REGION` | AWS region for all SDK clients. | Yes (AWS) | — | `us-east-1` | AWS region id |
| `LOG_LEVEL` | Structured-log verbosity (`slog`). | No | `info` | `debug` | `debug` \| `info` \| `warn` \| `error` |
| `PROJECT_NAME` | Resource name prefix / cost tag. | No | — | `blog-gen` | lowercase-kebab |

## Publishing trigger

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `PUBLISH_TRIGGER` | Optional commit-message prefix stored in metadata (retained for future push-based sources; the MVP is driven by `POST /process`). | No | `blog:` | `blog:` | non-empty string |

## Data stores

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `REPOSITORIES_TABLE` | DynamoDB table of registered repositories. | Yes | — | `blog-gen-repositories` | table name |
| `REPO_SECRET_ID` | Secrets Manager id/ARN holding all repos' credentials (JSON keyed by `owner/name`). | Yes | — | `blog-gen/github/repositories` | name or ARN |
| `SECRETS_PREFIX` | Prefix for per-repo secret lookups. | No | `blog-gen/repos` | `blog-gen/repos` | path prefix |

## Event plumbing

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `STATE_MACHINE_ARN` | Step Functions state machine the manual trigger starts on `POST /process`. | Yes (manual-trigger) | — | `arn:aws:states:…:stateMachine:blog-gen-orchestration` | state machine ARN |
| `QUEUE_URL` | SQS queue the worker drains. | Yes (worker) | — | `https://sqs…/blog-gen` | SQS URL |
| `WEBHOOK_URL` | Optional GitHub webhook URL for registration. Blank (the default) skips creating a GitHub webhook — ingress has been removed. | No | — | — | HTTPS URL |
| `N8N_WEBHOOK_URL` | Optional n8n orchestration entry point. | No | — | `https://…/n8n` | HTTPS URL |

## Compute lifecycle

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `INSTANCE_ID` | On-Demand EC2 host the scheduler powers on/off (fixed daily window). | Yes (scheduler) | — | `i-0abc123` | EC2 instance id |

## AI — Provider Router (Bedrock primary → Anthropic fallback)

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `BEDROCK_MODEL_ID` | Provider Router primary leg — Bedrock Claude model id (IAM auth, no key). Blank disables the Bedrock leg. | No | — | `us.anthropic.claude-opus-4-8` | Bedrock model id |
| `ANTHROPIC_API_KEY_SECRET` | Secrets Manager id/ARN holding the Anthropic API key for the fallback leg. | No | — | `blog-gen/anthropic/api-key` | name or ARN |
| `ANTHROPIC_MODEL` | Provider Router fallback leg — Anthropic API model id (blank uses the client default). | No | — | `claude-opus-4-8` | Anthropic model id |

## Worker filesystem (on the instance, `/data` is the EBS volume)

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `OUTPUT_DIR` | Where published Markdown is written (local mode). | No | `/data/generated-content` | `/data/generated-content` | absolute path |
| `WORK_DIR` | Scratch space for clones/work. | No | `/data/work` | `/data/work` | absolute path |
| `MEMORY_DIR` | Repository Memory store. | No | `/data/memory` | `/data/memory` | absolute path |
| `PENDING_DIR` | Holds artifacts awaiting human approval. | No | `/data/pending` | `/data/pending` | absolute path |

## Output to S3 (optional — overrides local files)

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `OUTPUT_S3_BUCKET` | Publish artifacts to this bucket instead of local disk. | No | — (local) | `my-content-bucket` | bucket name |
| `OUTPUT_S3_PREFIX` | Key prefix under the bucket. | No | `generated-content` | `generated-content` | key prefix |

## Publishing / approval

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `REQUIRE_HUMAN_APPROVAL` | Gate publishing on human approval. | No | `false` | `true` | `true` \| `false` |

## Notifications (optional; blank = log only)

| Variable | Purpose | Req? | Def | Example | Format / values |
|----------|---------|------|-----|---------|-----------------|
| `NOTIFY_WEBHOOK_URL` | POST run outcomes to this URL. | No | — | `https://hooks…/x` | HTTPS URL |
| `NOTIFY_EMAIL_FROM` | Sender address (enables email with `_TO`). | No | — | `bot@acme.dev` | email |
| `NOTIFY_EMAIL_TO` | Recipient(s), comma-separated. | No | — | `a@x.com,b@x.com` | CSV of emails |
| `SMTP_HOST` | SMTP server. | No | `pro.turbo-smtp.com` | `smtp.acme.dev` | hostname |
| `SMTP_PORT` | SMTP port. | No | `587` | `587` | integer |
| `SMTP_USERNAME` | SMTP user (required to send email). | No | — | `apikey` | string |
| `SMTP_PASSWORD` | SMTP password — **local/dev only**. | No | — | — | string (keep out of git) |
| `SMTP_PASSWORD_SECRET` | Secrets Manager id/ARN for the SMTP password; resolved at startup and **wins over** `SMTP_PASSWORD` (preferred in AWS). | No | — | `blog-gen/smtp` | name or ARN |

## Validation rules

- The request Lambdas and worker **fail fast** at startup if a variable they
  require (per the tables above) is missing or malformed.
- `SMTP_PORT` must parse as an integer; `LOG_LEVEL` must be one of the listed
  values; URLs must be well-formed.
- Email sending activates only when `NOTIFY_EMAIL_FROM`, `NOTIFY_EMAIL_TO`,
  `SMTP_USERNAME`, and a password (`SMTP_PASSWORD` or `SMTP_PASSWORD_SECRET`) are
  all present; otherwise notifications fall back to structured logging.
- Secrets (`REPO_SECRET_ID`, `SMTP_PASSWORD_SECRET`) are resolved from AWS Secrets
  Manager at runtime and are **never** logged.

## Standalone-CLI configuration

The generator CLIs (`storyboard`, `youtube`, `generate-all`, …) generate through
the AI Provider Router (Claude Code by default, or `--provider anthropic|bedrock`
with `ANTHROPIC_API_KEY`), and take everything else via flags (`--context`,
`--blog`, `--offline`, `--out`, …). With `--offline` they use no model and no
network. See each command's `--help` and [Full Content Suite](./content-suite.md).
