# Runbook: Release → Content (end-to-end validation)

This runbook validates the complete pipeline on a live AWS account: publishing a
GitHub Release produces the **full, reviewed, published content suite** — blog,
storyboard, voice-over, YouTube, Shorts, TikTok, visual assets, SEO, architecture,
LinkedIn, and X thread. It exercises Semantic Version validation, Milestone 2
(Release Context), the Milestone 3–13 content suite, and the review/publish/notify
stages together. No manual `generate-all` invocation is required — the worker's
release path runs the whole suite automatically.

Related: [Deployment](./deployment.md) · [Release Context](./release-context.md) · [Full Content Suite](./content-suite.md) · [Blog Generation](./blog-generation.md).

---

## 1. The flow being validated

```mermaid
flowchart LR
    REL[Publish GitHub Release] --> WH[webhook-handler]
    WH -->|release event| EB[EventBridge]
    EB --> SQS[(SQS)]
    SQS --> W[worker on EC2]
    W --> SV{SemVer valid?}
    SV -- no --> STOP[reject early]
    SV -- yes --> CTX[Release Context]
    CTX --> GEN[full content suite via Ollama\nblog → … → X thread]
    GEN --> REV[review] --> APP[approval] --> PUB[publish] --> OUT[(output bucket / EBS)]
    W -.-> NOTE[notify]
```

The webhook front door is always up; the **worker only runs during the instance
window** (18:00–20:00 Mon–Fri by default). A release published outside the window
is buffered in SQS and processed at the next start — or force a start (§5).

---

## 2. Prerequisites

- The stacks are deployed (`blog-gen-serverless`, `blog-gen-compute`,
  `blog-gen-scheduler`, `blog-gen-observability`). See [Deployment](./deployment.md)
  and [Installation → bootstrap](../README.md#installation).
- The EC2 instance runs **Ollama** with the configured model pulled, and the
  `blog-gen-worker` service is active.
- The target repository is **registered** (webhook created, PAT stored) — see §4.
- AWS CLI v2 configured for the account/region.

Set these once for the commands below:

```bash
export AWS_REGION=us-east-1
export STACK=blog-gen
```

---

## 3. Confirm the deployment is healthy

```bash
# Serverless endpoints (note ReleaseContextUrl, RegistrationUrl, WebhookUrl)
aws cloudformation describe-stacks --stack-name ${STACK}-serverless \
  --query "Stacks[0].Outputs" --output table

# Release-context Lambda is deployed
aws lambda get-function --function-name ${STACK}-release-context \
  --query 'Configuration.[State,LastUpdateStatus]' --output text

# The worker is running on the instance (SSH in first — see Deployment §5)
systemctl status blog-gen-worker --no-pager
curl -s http://127.0.0.1:11434/api/tags   # Ollama lists the pulled model
```

---

## 4. Register the target repository

Registration creates the GitHub webhook (subscribed to **push and release**
events) and stores the repo's PAT in the shared secret. The worker reads the
repo with that same PAT.

```bash
GITHUB_PAT=github_pat_xxx WEBHOOK_SECRET=$(openssl rand -hex 16) \
  scripts/register-repository.sh --owner <owner> --repository <repo>
```

Confirm in GitHub: **Settings → Webhooks → Recent Deliveries** shows a green ✓,
and **Settings → Webhooks** lists the `release` event.

> The PAT needs **Contents: Read** and **Metadata: Read** (plus **Webhooks:
> Read and write** for registration). The worker reads the repo via the GitHub
> API using this PAT; `GITHUB_TOKEN` on the instance is only a public/unregistered
> fallback.

---

## 5. Trigger a run

### Option A — full loop (recommended): publish a release

1. On the registered repo, publish a GitHub Release (e.g. tag `v0.2.0`).
2. If **outside the instance window**, force a start so the worker drains the
   queue now:

```bash
aws lambda invoke --function-name ${STACK}-scheduled-start /dev/stdout
```

### Option B — context only (no instance needed): call the endpoint

Builds and stores the Release Context without generating content — useful to
verify Milestone 2 in isolation.

```bash
URL=$(aws cloudformation describe-stacks --stack-name ${STACK}-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='ReleaseContextUrl'].OutputValue" --output text)
KEY_ID=$(aws cloudformation describe-stacks --stack-name ${STACK}-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationApiKeyId'].OutputValue" --output text)
API_KEY=$(aws apigateway get-api-key --api-key "$KEY_ID" --include-value --query value --output text)

curl -sS -X POST "$URL" -H "x-api-key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"owner":"<owner>","repository":"<repo>","releaseTag":"v0.2.0"}'
# -> {"status":"accepted","contextId":"…","location":"s3://…/release-contexts/…json", "warnings":N}
```

---

## 6. Verify each stage

| Stage | Check |
| --- | --- |
| **Webhook received** | GitHub → Recent Deliveries: `200`, body `accepted`/`deferred`. |
| **Event published** | `blog-gen-webhook-handler` logs show `release` → PutEvents. |
| **Buffered** | SQS `ApproximateNumberOfMessages` increments (drains when the worker is up). |
| **Context built** | `release-contexts/<date>/<id>.json` exists in the artifacts bucket (Option B), or worker logs `release context built`. |
| **Content generated** | Worker logs `blog post generated` / `content generated`; metric `ReleaseRunsSucceeded`. |
| **Published** | New Markdown in the output destination (§7). |
| **Notified** | Log/webhook/email notification (if configured). |

Commands:

```bash
# Webhook-handler logs (last 5 min)
aws logs tail /aws/lambda/${STACK}-webhook-handler --since 5m --format short

# Release-context Lambda logs
aws logs tail /aws/lambda/${STACK}-release-context --since 10m --format short

# Worker logs on the instance
journalctl -u blog-gen-worker -n 100 --no-pager

# Queue depth (the events queue URL is a serverless-stack output)
QURL=$(aws cloudformation describe-stacks --stack-name ${STACK}-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='QueueUrl'].OutputValue" --output text)
aws sqs get-queue-attributes --queue-url "$QURL" \
  --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible
```

---

## 7. Where the output lands

- **S3 output bucket** (when `OUTPUT_S3_BUCKET` is set on the worker): objects
  under the configured prefix, one Markdown file per asset (`blog`,
  `release-summary`, …), dated.
- **Instance filesystem** otherwise: under the worker's `OUTPUT_DIR`
  (default `/data/generated-content`) on the persistent EBS volume.
- **Release Contexts**: `release-contexts/<yyyy>/<mm>/<dd>/<contextId>.json` in
  the artifacts bucket.

Fetch and eyeball the blog post:

```bash
aws s3 ls "s3://<output-bucket>/<prefix>/" --recursive | tail
aws s3 cp "s3://<output-bucket>/<prefix>/<owner>-<repo>/<date>/blog.md" - | head -60
```

Acceptance: the `blog.md` opens with YAML front matter (`title`, a **150–160
char** `description`, `tags`), an H1, the full section structure
(Introduction … Conclusion), and — if the repo has Mermaid docs — an
`## Architecture Diagrams` section with fenced `mermaid` blocks.

---

## 8. Troubleshooting

| Symptom | Likely cause / fix |
| --- | --- |
| Webhook `deferred`, nothing generated | Instance outside its window — force a start (§5) or wait for 18:00. |
| Worker logs `release run failed … unauthorized` | Repo PAT lacks Contents:Read, or repo not registered — re-register (§4). |
| `no content generated` | Ollama not reachable/model not pulled — `curl 127.0.0.1:11434/api/tags`, `ollama pull <model>`. |
| Context `warnings` include "no CHANGELOG"/"no commits" | Expected for sparse releases; content still generates from what's present. |
| Message keeps redelivering | A stage errors each attempt; check worker logs. After max receives it moves to the DLQ. |
| 403 from `/release-context` | Missing/!wrong `x-api-key` (§5, Option B). |

---

## 9. Cleanup

- Delete generated test objects from the output bucket and
  `release-contexts/` prefix.
- Deregister the test repo (removes webhook + stored PAT):

  ```bash
  scripts/register-repository.sh --owner <owner> --repository <repo> --delete
  ```

- If you force-started the instance, it stops at the next scheduled stop (20:00),
  or stop it now via `aws lambda invoke --function-name ${STACK}-scheduled-stop /dev/stdout`.
