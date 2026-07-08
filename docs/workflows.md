# Workflows

Control flow spans two layers. A **lightweight Webhook Handler Lambda** performs the **commit-message trigger gate**; only matched events reach the **n8n workflows** on the EC2 Spot Instance, which run the full generation pipeline. Matched events are buffered in **Amazon SQS** (via EventBridge) and **invoke the n8n workflow** once the instance is healthy; n8n then drives checkout, analysis (**OpenClaw**), **Repository Memory**, generation (**Ollama/Qwen**), quality review, optional human approval, publishing, and notifications.

> **Registration is not an n8n workflow.** Onboarding a repository (URL + PAT → validate → create webhook → store metadata + PAT) is handled by the **Registration Lambda** ([Architecture §2](./architecture.md#2-repository-registration-mvp)). These workflows cover only the per-event generation pipeline.

Related: [Architecture](./architecture.md) · [Infrastructure](./infrastructure.md) · [Monitoring](./monitoring.md).

---

## 1. Trigger Gate (Webhook Handler Lambda)

Before any n8n workflow runs, the handler decides whether a run should happen at all.

```mermaid
flowchart TB
    W[Webhook: payload + X-Hub-Signature-256] --> LK[Resolve repo record<br/>DynamoDB]
    LK --> GS[Get webhook secret<br/>Secrets Manager]
    GS --> SIG{Valid HMAC?}
    SIG -- no --> R401[401 + log rejection]
    SIG -- yes --> EX[Extract commit message]
    EX --> T{Matches repo Trigger Pattern?<br/>default 'blog:'}
    T -- no --> ACK[HTTP 200 — acknowledge & ignore<br/>no further processing]
    T -- yes --> PUT[PutEvents → EventBridge]
    PUT --> OK[HTTP 200]
```

- The handler is **not** an n8n workflow — it is a Lambda ([Architecture §3](./architecture.md#3-commit-message-trigger-gate)).
- It performs **only**: resolve repo record → verify signature (per-repo secret) → extract commit info → validate trigger → publish matched event → return 200. It does **no** analysis or inference, and **never reads the PAT**.
- EventBridge routes matched events to **SQS** (buffer) and the **Instance Starter Lambda** (start the Spot host).

---

## 2. Workflow Catalog (n8n)

| Workflow | File | Trigger | Purpose |
| --- | --- | --- | --- |
| Event Ingestion | `event-ingestion.json` | **SQS poll** | Pulls matched events, parses the payload |
| Repository Analysis | `repository-analysis.json` | Called by ingestion | Checkout + structure/README/source/config analysis via OpenClaw |
| Repository Memory | `repository-memory.json` | Called by analysis | Looks up prior analyses and published topics; records new ones |
| Content Generation | `content-generation.json` | Called by analysis | Topic ID, outline, and local inference via Ollama per content type |
| Quality Review | `quality-review.json` | Called by generation | Reviews drafts before publishing |
| Approval & Publishing | `approval-publishing.json` | Called by review | Optional human approval, then publishes Markdown |
| Notifications | `notifications.json` | Called on success/failure | Notifies users |

Each workflow can also be executed independently for testing ([WF-8](./requirements.md#4-workflow-requirements)).

> **Trigger note:** every event in SQS has already passed the commit-message trigger gate in the handler. n8n never sees ignored events. Instance start/stop is handled by Lambda (starter + idle-shutdown), **not** by n8n.

---

## 3. End-to-End Pipeline

```mermaid
flowchart LR
    SQS[(Amazon SQS)] --> I[Event Ingestion]
    I --> A[Repository Analysis<br/>OpenClaw]
    A --> M[Repository Memory<br/>lookup]
    M --> G[Content Generation<br/>topic → outline → Ollama]
    G --> R[Quality Review]
    R --> P[Approval & Publishing]
    P --> N[Notifications]
    P -.record topics.-> M
    I -. on error .-> N
    A -. on error .-> N
    G -. on error .-> N
    R -. on error .-> N
    P -. on error .-> N
```

---

## 4. Event Ingestion

Drains the queue and starts a run per matched event.

```mermaid
flowchart TB
    POLL[Poll SQS] --> MSG{Message available?}
    MSG -- no --> IDLE[Idle → instance may auto-stop]
    MSG -- yes --> PARSE[Parse matched event<br/>repo, ref, commit]
    PARSE --> HANDOFF[Handoff → Analysis]
    HANDOFF --> DONE{Run succeeded?}
    DONE -- yes --> DEL[Delete SQS message]
    DONE -- no --> KEEP[Leave message<br/>visibility timeout → retry / DLQ]
```

A message is **deleted only after a successful run** ([WF-6](./requirements.md#4-workflow-requirements)); a failure lets the visibility timeout expire so the message is retried, and repeated failures land in the **dead-letter queue**.

**Input:** SQS message `{ repo, ref, commit, delivery_id }` · **Output:** `{ runId, repoMeta }`

---

## 5. Repository Analysis

Builds a structured understanding of the repository using **OpenClaw**.

```mermaid
flowchart TB
    C[Checkout / sync repository] --> ST[Structure analysis]
    ST --> RD[README analysis]
    RD --> SR[Source code analysis]
    SR --> CF[Configuration + IaC + Docker + CI/CD]
    CF --> TD[Technology stack detection]
    TD --> O[Output: context manifest]
```

**Input:** `{ runId, repoMeta }` · **Output:** `{ context, contextManifest }`

Implements [FR-2](./requirements.md#12-repository-cloning--analysis). Clones are transient — they live on the instance disk during the run.

---

## 6. Repository Memory

Provides continuity across runs so the platform avoids duplicate content and builds on prior work.

```mermaid
flowchart TB
    IN[context manifest] --> LK[Look up prior analyses<br/>+ published topics for this repo]
    LK --> CTX[Augment context with memory]
    CTX --> OUT[Memory-aware context]
    PUBLISHED[Published topics from a completed run] --> REC[Record into memory]
```

Repository Memory is a **persistent per-repository store on the EBS volume**. On lookup it returns previously identified topics and published posts; after publishing, the run records new topics ([REG-MEM requirements](./requirements.md#5-repository-memory-requirements)).

**Input:** `{ context }` · **Output:** `{ memoryAwareContext }`

---

## 7. Content Generation

Identifies topics, builds an outline, then runs **local inference via Ollama** once per content type; retries transient failures with backoff.

```mermaid
flowchart TB
    P[memory-aware context] --> TID[Topic identification]
    TID --> OUT[Outline generation]
    OUT --> LOOP{For each content type}
    LOOP --> OL[Ollama generate<br/>local Qwen model]
    OL --> C{Success?}
    C -- transient --> RB[Backoff + retry ≤ N]
    RB --> OL
    C -- failed --> E[Error → Notifications]
    C -- ok --> ACC[Accumulate Markdown]
    ACC --> LOOP
    LOOP --> O[Draft content set]
```

**Content types:** technical blog post, README improvements, project documentation, architecture summary, API documentation, project overview, release notes, changelog, technical tutorial ([FR-3](./requirements.md#13-documentation--content-generation)).

The model (`OLLAMA_MODEL`, default Qwen) and parameters come from configuration ([AI-2](./requirements.md#6-ai--local-inference-requirements)); there is **no external inference API**.

---

## 8. Quality Review

```mermaid
flowchart TB
    D[Draft content set] --> Q[Automated quality checks<br/>structure, completeness, Markdown validity]
    Q --> V{Passes?}
    V -- no --> E[Flag → Notifications / retry]
    V -- yes --> A[Approved for approval gate]
```

A review stage checks drafts before they can be published ([FR-3.12](./requirements.md#13-documentation--content-generation)).

---

## 9. Approval & Publishing

```mermaid
flowchart TB
    C[Reviewed content set] --> G{Human approval required?<br/>REQUIRE_HUMAN_APPROVAL}
    G -- yes --> WAIT[Notify approver + wait for decision]
    WAIT --> DEC{Approved?}
    DEC -- no --> REJ[Discard draft + log + notify]
    DEC -- yes --> PUB
    G -- no --> PUB[Render Markdown + publish]
    PUB --> REC[Record published topics → Repository Memory]
    REC --> SHA[Update last processed commit SHA → DynamoDB]
    SHA --> ME[Emit CloudWatch metrics]
    ME --> OK[Success → Notifications]
```

**Optional human approval** (`REQUIRE_HUMAN_APPROVAL`, default `false`) pauses the pipeline for a human decision before publishing. All output is **GitHub-flavoured Markdown**; a failure never corrupts previously published output ([FR-5.5](./requirements.md#16-reliability-retry-logging--error-handling)).

---

## 10. Notifications

A shared sub-workflow invoked on success, failure, and approval-request paths.

```mermaid
flowchart LR
    IN[result / approval request] --> R{Channel}
    R --> EMAIL[Email]
    R --> SLACK[Slack webhook]
    R --> WH[Generic webhook]
```

**Input:** `{ status, repo, publishedRefs?, approvalRequest?, error? }` ([FR-4](./requirements.md#14-output-publishing--notifications)).

---

## 11. Importing Workflows

1. Start the instance and open n8n over an **SSH tunnel** (the UI is not publicly exposed — see [Deployment §5](./deployment.md#5-import-the-n8n-workflows)); locally it's `http://localhost:5678`.
2. **Workflows → Import from File** and select each JSON in `workflows/`.
3. Configure **Credentials** (AWS/SQS, GitHub, notification channel).
4. Activate the workflows.

### Exporting after changes

```bash
# via the n8n editor: Workflow → Download
git add workflows/*.json
git commit -m "chore(workflows): update content generation flow"
```

Keep exported JSON free of embedded secrets — credentials are referenced by ID, not value ([WF-7](./requirements.md#4-workflow-requirements)).
