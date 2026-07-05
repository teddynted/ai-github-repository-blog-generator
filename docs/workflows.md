# Workflows

The platform's control flow is implemented as **n8n workflows**. Each workflow is a discrete, versioned unit exported as JSON under `workflows/n8n/`. n8n (running on the EC2 host) performs cloning, analysis, prompt building, Amazon Bedrock invocation, packaging, and notifications, passing structured data between stages.

Related: [Architecture](./architecture.md) · [Infrastructure](./infrastructure.md) · [Monitoring](./monitoring.md).

---

## 1. Workflow Catalog

| Workflow | File | Trigger | Purpose |
| --- | --- | --- | --- |
| Webhook Ingestion | `webhook-ingestion.json` | **GitHub Webhook** (HTTPS) | Receives events, validates the HMAC signature, extracts repo info |
| Repository Analysis | `repository-analysis.json` | Called by ingestion | Structure, README, source, config, tech stack, architecture |
| Content Generation | `content-generation.json` | Called by analysis | Invokes Amazon Bedrock per content type |
| Packaging & Publishing | `packaging-publishing.json` | Called by generation | Assembles the package, writes it to S3 |
| Notifications | `notifications.json` | Called on success/failure | Sends notifications |

The workflows compose into a single pipeline; each can also be executed independently for testing ([WF-6](./requirements.md#7-workflow-requirements)).

> **Trigger note:** the **primary trigger is a GitHub Webhook** delivered to the n8n HTTPS endpoint ([GitHub Webhook Requirements](./requirements.md#3-github-webhook-requirements)); manual invocation is also supported. The daily EC2 start/stop (19:00–21:00) is **not** an n8n workflow — it is an **EventBridge Scheduler** rule invoking the `ec2-scheduler` Go Lambda ([Infrastructure §6](./infrastructure.md#6-amazon-eventbridge)).

---

## 2. End-to-End Pipeline

```mermaid
flowchart LR
    T[GitHub Webhook / manual] --> I[Webhook Ingestion]
    I --> A[Repository Analysis]
    A --> G[Content Generation]
    G --> P[Packaging & Publishing]
    P --> N[Notifications]
    I -. on error .-> N
    A -. on error .-> N
    G -. on error .-> N
    P -. on error .-> N
```

---

## 3. Webhook Ingestion

Entry point. Receives the GitHub Webhook over HTTPS, **validates the HMAC SHA-256 signature** against the secret in Secrets Manager, extracts repository information, clones/updates the repository on the EC2 working directory, and hands off to analysis. Invalid deliveries are rejected and logged.

```mermaid
flowchart TB
    W[Webhook: payload + X-Hub-Signature-256] --> SEC[Get webhook secret<br/>Secrets Manager]
    SEC --> HM{HMAC SHA-256 valid?<br/>constant-time compare}
    HM -- no --> R401[Respond 401 + log rejection]
    HM -- yes --> EX[Extract owner, repo, event, ref]
    EX --> SUP{Supported event?}
    SUP -- no --> SKIP[Skip + log]
    SUP -- yes --> TOK[Get GitHub token<br/>Secrets Manager]
    TOK --> C[Clone / update repository]
    C --> OK{Clone OK?}
    OK -- no --> E[Emit error → Notifications]
    OK -- yes --> H[Respond 202 + handoff → Analysis]
```

**Triggers:** GitHub Webhook events — `push`, `release`, `pull_request`, `workflow_dispatch`, `repository`; plus manual ([GitHub Webhook Requirements](./requirements.md#3-github-webhook-requirements)).
**Input:** `{ headers, payload }` · **Output:** `{ workingCopyRef, repoMeta, event }`

See [Security → Webhook Security](./security.md#10-webhook-security) for the validation contract.

---

## 4. Repository Analysis

Builds a structured understanding of the repository.

```mermaid
flowchart TB
    A[working copy] --> ST[Structure analysis]
    ST --> RD[README analysis]
    RD --> SR[Source code analysis]
    SR --> CF[Configuration file analysis]
    CF --> TD[Technology stack detection]
    TD --> AR[Architecture understanding]
    AR --> PB[Prompt assembly<br/>per platform, token-budgeted]
    PB --> O[Output: prompt set + context]
```

**Input:** `{ workingCopyRef, repoMeta }` · **Output:** `{ prompts, contextManifest }`

Implements [FR-1](./requirements.md#11-repository-analysis) (repository analysis) and feeds [AI Requirements](./requirements.md#4-ai-requirements).

---

## 5. Content Generation

Invokes Amazon Bedrock once per content type with platform-tuned prompts and configured model parameters; retries on throttling with exponential backoff.

```mermaid
flowchart TB
    P[prompt set + params] --> LOOP{For each content type}
    LOOP --> B[Bedrock InvokeModel]
    B --> C{Success?}
    C -- throttled/transient --> RB[Backoff + retry ≤ N]
    RB --> B
    C -- failed --> E[Error → Notifications]
    C -- ok --> ACC[Accumulate content + token usage]
    ACC --> LOOP
    LOOP --> O[Content set]
```

**Content types:** `blog`, `medium`, `devto`, `hashnode`, `newsletter`, `linkedin`, `twitter-thread`, `reddit`, `faq`, `readme-suggestions`, `image-prompts`, `seo`, `metadata` ([FR-2](./requirements.md#12-content-generation)).
**Input:** `{ prompts, model, temperature, maxTokens }` · **Output:** `{ contentSet, tokenUsage }`

Model ID and parameters come from Terraform config ([AI-2](./requirements.md#4-ai-requirements)).

---

## 6. Packaging & Publishing

```mermaid
flowchart TB
    C[Content set] --> R[Render Markdown + JSON assets]
    R --> KV[Compute dated key:<br/>generated-content/&lt;repo&gt;/&lt;YYYY-MM-DD&gt;/]
    KV --> W[Write package → S3]
    W --> ME[Emit CloudWatch metrics]
    ME --> OK[Success → Notifications]
```

The package is written under `generated-content/<repository-name>/<YYYY-MM-DD>/` with S3 versioning enabled, containing all articles plus `seo.json`, `metadata.json`, and `image-prompts.md`. Metadata (`title`, `date`, `source_repo`, `tags`, `reading_time`, `model`) is captured in `metadata.json` ([FR-2](./requirements.md#12-content-generation), [Storage Requirements](./requirements.md#8-storage-requirements)). Failures never overwrite an existing package ([FR-6.3](./requirements.md#16-error-handling--retries)).

---

## 7. Notifications

A shared sub-workflow invoked on both success and failure paths.

```mermaid
flowchart LR
    IN[result: success/failure + context] --> R{Channel}
    R --> SNS[SNS → email]
    R --> SLACK[Slack webhook]
    R --> WH[Generic webhook]
```

**Input:** `{ status, repo, packagePrefix?, error? }` ([FR-6](./requirements.md#16-error-handling--retries)).

---

## 8. Importing Workflows

1. Open n8n on the EC2 host via SSM port-forwarding (the management UI is not publicly exposed — only the webhook path is — see [Deployment §7](./deployment.md#7-deployment-verification)); locally it's `http://localhost:5678`.
2. **Workflows → Import from File** and select each JSON in `workflows/n8n/`.
3. Configure **Credentials** (AWS, GitHub, notification channel) in the n8n editor — these reference Secrets Manager values in AWS.
4. Activate the workflows.

### Exporting after changes

Export edited workflows back to JSON and commit them so the repo stays the source of truth:

```bash
# via the n8n editor: Workflow → Download
git add workflows/n8n/*.json
git commit -m "chore(workflows): update content generation flow"
```

Keep exported JSON free of embedded secrets — credentials are referenced by ID, not value ([WF-5](./requirements.md#7-workflow-requirements)).
