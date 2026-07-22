# Architecture

This document describes the architecture of the **GitHub AI Blog Generator**: the design principles, **repository registration (MVP)**, the **commit-message trigger gate**, the event-driven trigger path, the AWS deployment topology, the analysis and local-inference pipeline, content generation, data flow, and storage.

Related: [Requirements](./requirements.md) · [Infrastructure](./infrastructure.md) · [Workflows](./workflows.md) · [Cost Optimisation](./cost-optimization.md) · [Security](./security.md).

---

## 1. Design Principles

- **Simple MVP onboarding** — users connect a repository with its URL and a **GitHub Personal Access Token (PAT)**; the PAT is stored in **AWS Secrets Manager** and only a reference is kept in the metadata store. GitHub App authentication is a future enhancement.
- **Opt-in generation** — a webhook is received on every push, but a run happens **only** when the commit message matches the repository's publishing trigger (default `blog:`) or a release is published. All other events are acknowledged and ignored.
- **Two trigger sources, one pipeline** — a run can be initiated **automatically by a GitHub webhook**, or **manually by the authenticated `POST /process` endpoint** ([Manual Trigger](./manual-trigger.md)). Both publish the same `blog.publish.requested` event, so the downstream pipeline is identical regardless of how it was triggered; future sources (CLI, Slack, cron) plug in the same way.
- **Event-driven** — triggered events are published to **Amazon EventBridge** and buffered in **Amazon SQS**; the worker drains them while the instance is up. Nothing runs speculatively.
- **Self-hosted inference** — all AI runs locally via **Ollama** with a local **Qwen** model. No Amazon Bedrock, OpenAI, Anthropic, or any paid inference API.
- **Pay only during the window** — an **On-Demand EC2 instance** runs on a fixed daily schedule (18:00–20:00, 7 days a week) set by EventBridge Scheduler; compute cost is bounded and predictable.
- **Durable buffering** — every matched event is buffered in **Amazon SQS** so nothing is lost while the instance is stopped or booting.
- **Persistent state, ephemeral compute** — models, n8n state, and **Repository Memory** live on a persistent **gp3 EBS volume**.
- **Secure by default** — least-privilege IAM, secrets in Secrets Manager (never logged), HMAC-verified webhooks, encryption everywhere.
- **Everything as code** — infrastructure is AWS CloudFormation; workflows are versioned n8n JSON; services run via Docker Compose.

---

## 2. Repository Registration (MVP)

Onboarding connects a repository using a **GitHub PAT**. It validates access, creates the webhook, stores metadata, and stores the PAT securely — all before any content is ever generated.

```mermaid
flowchart TB
    U[User: Add Repository] --> IN[Enter Repository URL + PAT]
    IN --> REGAPI[Registration API<br/>API Gateway + Lambda]
    REGAPI --> VA[Validate repository access<br/>GitHub API]
    VA -- fail --> ERR[Reject + log]
    VA -- ok --> VP[Validate token permissions]
    VP --> WH[Create GitHub webhook<br/>+ generate webhook secret]
    WH --> META[Store metadata → DynamoDB<br/>secret reference, trigger pattern, ...]
    META --> SEC[Store PAT + webhook secret → Secrets Manager]
    SEC --> DONE[Repository successfully registered]
```

**What the PAT is used for:** accessing repository contents, registering the GitHub webhook, reading repository metadata, and (future) repository synchronisation.

**Where things are stored:**

| Data | Store | Notes |
| --- | --- | --- |
| GitHub PAT | **AWS Secrets Manager** | Never in plain text, source, config, env vars, or the database |
| Webhook signing secret | **AWS Secrets Manager** | Per repository |
| Repository metadata | **Amazon DynamoDB** | Repository ID, owner, name, URL, default branch, webhook ID, **trigger pattern**, enabled status, **secret reference (ARN)**, last processed commit SHA, registration timestamp |

The metadata store holds only a **reference** to the secret — never the PAT itself. The PAT is retrieved **only when required** (clone, webhook management), under least-privilege IAM ([Security → Credentials](./security.md#2-github-pat--secret-storage), [Requirements §12–§14](./requirements.md#12-repository-registration-requirements-mvp)).

> **GitHub App authentication is a future enhancement, not part of the MVP** ([Roadmap](./roadmap.md)).

---

## 3. Commit-Message Trigger Gate

The trigger gate lives in the **Webhook Handler Lambda** and runs before any compute or AI is invoked.

```mermaid
flowchart TB
    W[Webhook delivery] --> LK[Resolve repository record<br/>DynamoDB]
    LK --> GS[Get webhook secret<br/>Secrets Manager]
    GS --> SIG{Valid HMAC signature?}
    SIG -- no --> R401[Return 401 + log rejection]
    SIG -- yes --> EX[Extract commit message]
    EX --> T{Matches repository's<br/>Trigger Pattern? default 'blog:'}
    T -- no --> ACK[Return HTTP 200<br/>acknowledge & ignore — no further processing]
    T -- yes --> PUT[PutEvents → Amazon EventBridge]
    PUT --> OK[Return HTTP 200]
```

- The trigger pattern is **per repository** (stored in metadata, default `blog:`). Custom patterns (`[blog]`, regex) are [planned](./roadmap.md).
- A non-matching event is a **terminal success**: the handler returns `200` and does nothing else.
- Only a **matching** event is published to EventBridge.

See [Cost Optimisation → Trigger Pre-Filtering](./cost-optimization.md#2-trigger-pre-filtering) and [Requirements → Publishing Trigger](./requirements.md#2-publishing-trigger-requirements).

---

## 4. High-Level Architecture

```mermaid
flowchart TB
    subgraph GitHub
        REG[Registration: URL + PAT]
        EVT[GitHub Webhook<br/>push]
        REPO[(Target Repository)]
    end

    subgraph AWS
        APIGW[Amazon API Gateway<br/>webhook + registration]
        RL[Lambda: Registration]
        LH[Lambda: Webhook Handler<br/>verify + validate trigger]
        SM[AWS Secrets Manager<br/>PAT + webhook secret]
        DDB[(DynamoDB<br/>repository metadata)]
        EB[Amazon EventBridge]
        SQS[(Amazon SQS + DLQ)]
        SCH[EventBridge Scheduler<br/>18:00 / 20:00 daily]
        PWR[Lambda: scheduled-start / scheduled-stop]
        subgraph EC2["EC2 On-Demand Instance (Ubuntu + Docker Compose)"]
            N8N[n8n Orchestrator]
            OC[OpenClaw]
            MEM[(Repository Memory)]
            OLL[Ollama + Qwen]
        end
        EBS[(Persistent gp3 EBS Volume)]
        CW[Amazon CloudWatch]
    end

    REG -->|HTTPS| APIGW --> RL
    RL -->|validate + create hook| REPO
    RL -->|metadata| DDB
    RL -->|PAT + webhook secret| SM
    EVT -->|HTTPS + HMAC| APIGW --> LH
    LH -->|lookup| DDB
    LH -->|webhook secret| SM
    LH -->|trigger match → PutEvents| EB
    LH -->|no match → 200| EVT
    EB --> SQS
    SCH --> PWR -->|Start/StopInstances| EC2
    SQS -.->|drained while up| N8N
    N8N --> OC --> MEM
    OC -->|PAT| SM
    OC --> OLL
    OLL -->|content| N8N
    N8N -->|publish + notify| OUT[Published content / users]
    EBS --- EC2
    LH -. logs .-> CW
    N8N -. logs .-> CW
```

**Component responsibilities**

| Component | Responsibility |
| --- | --- |
| Registration API (API Gateway + Lambda) | Validate repo access + token permissions, create webhook, store metadata (DynamoDB) and PAT/webhook secret (Secrets Manager) |
| AWS Secrets Manager | One shared secret holding all repos' PATs + webhook secrets (JSON keyed by owner/name); retrieved only when needed |
| Amazon DynamoDB | Repository metadata (secret reference, trigger pattern, webhook ID, …) — **never the PAT** |
| API Gateway | Public HTTPS ingress for webhook + registration |
| Webhook Handler (Lambda) | Resolve repo metadata, verify signature, **validate trigger**, publish matched events. **No analysis or inference; never fetches the PAT** |
| Amazon EventBridge | Route matched events to the SQS buffer |
| Amazon SQS | Durable buffer so no matched event is lost while the instance is outside its window; DLQ |
| EventBridge Scheduler | Authority for instance power — starts/stops the host on the daily window (18:00–20:00, 7 days a week) |
| scheduled-start / scheduled-stop (Lambda) | Start/stop the On-Demand instance on the schedule (idempotent) |
| EC2 On-Demand Instance | Host running n8n, OpenClaw, Ollama via Docker Compose |
| OpenClaw | Clone (using the PAT) and analyse the repository |
| Repository Memory | Per-repo continuity and topic de-duplication |
| Ollama + Qwen | Local LLM inference |
| Release Context API (Lambda: `release-context`) | Build and persist the structured Release Context for a repo + release; the Content Intelligence entrypoint (`POST /release-context`) |
| Release Context Builder (`internal/releasecontext`) | Analyse repository, release, commits, changed files, docs, CloudFormation, and Mermaid into a versioned, AI-ready Release Context |
| Content Generation (`internal/releasegen`) | Generate a long-form blog post plus release summary, social, and SEO variants from a Release Context via the local model |
| Release Content Pipeline (worker) | On a published release: build the context, generate content, review, publish, and notify |

---

### Content Intelligence & Generation (Milestones 2–3)

Beyond the trigger gate, the platform turns a repository + release into
publication-ready content. This is **Content Intelligence** (analysis) feeding
**Content Generation** (writing):

```mermaid
flowchart LR
    SRC[GitHub API + git] --> RCB[Release Context Builder]
    RCB --> RCX[(Release Context<br/>versioned JSON)]
    RCX --> GEN[Content Generation<br/>local Ollama]
    GEN --> REV[Review] --> PUB[Publish] --> OUT[(Output bucket / EBS)]
```

- The **Release Context** is the single, versioned source of truth
  (`schemaVersion 2.0.0`) — what changed, how it was implemented, why it
  matters, and how it fits the architecture. It is built by pure, decoupled
  analyzers behind a `Sources` port, so it is reusable from a Lambda, the worker,
  or a CLI.
- **Generation** is decoupled by a `Model` port (local Ollama today), grounded
  strictly in the context, and produces a long-form blog post (with SEO front
  matter and accurate embedded Mermaid) plus other formats.
- On a **published release**, the worker runs this end to end automatically;
  reads use the repository's registered PAT. See
  [Release Context](./release-context.md) and [Blog Generation](./blog-generation.md).

---

## 5. Webhook Handler Responsibilities

The handler is intentionally **lightweight** so it returns to GitHub in milliseconds. It is limited to:

1. **Resolve the repository record** (metadata) for the delivery.
2. Retrieve the repository's **webhook secret** and verify the signature (HMAC SHA-256, constant-time).
3. Parse the payload and extract commit information (message, ref, author).
4. **Validate the commit message against the repository's Trigger Pattern.**
5. **Publish an event to EventBridge only when the trigger matches.**
6. Return a successful HTTP response to GitHub as quickly as possible.

The handler **must not** clone repositories, perform analysis, access Repository Memory, invoke AI models, or **retrieve/log the PAT** ([Requirements → Webhook Handler](./requirements.md#3-webhook-handler-requirements)).

---

## 6. AWS Deployment Topology

The front door (API Gateway + Lambda + EventBridge + SQS + Secrets Manager + DynamoDB) is fully serverless and **always available even when the EC2 instance is stopped**. The compute host runs in a **public subnet**; inbound access is tightly restricted.

```mermaid
flowchart TB
    GH[GitHub] -->|443 HTTPS| APIGW[API Gateway]
    APIGW --> LH[Webhook Handler Lambda]
    APIGW --> RL[Registration Lambda]
    LH --> EB[EventBridge]
    EB --> SQS[(SQS)]
    SCH[EventBridge Scheduler] --> PWR[scheduled-start / scheduled-stop Lambda]
    subgraph VPC["Amazon VPC 10.0.0.0/16"]
        IGW[Internet Gateway]
        subgraph Public["Public subnet 10.0.0.0/24"]
            EC2[(EC2 On-Demand Instance<br/>n8n + OpenClaw + Ollama)]
            EBS[(gp3 EBS volume)]
        end
    end
    PWR -->|Start/StopInstances| EC2
    EC2 --- EBS
    EC2 --- IGW
    ADMIN[Operator] -->|SSH key auth<br/>restricted CIDR| EC2
```

- **Ingress:** GitHub reaches the platform through **API Gateway** (managed TLS). The instance's inbound is limited to **SSH (22) from operator IPs**; the **n8n and Ollama ports are never publicly exposed**.
- **Egress:** the **Internet Gateway** provides outbound access for cloning, image pulls, and model downloads.

---

## 7. Analysis & Generation Pipeline

Once the instance is up and n8n is invoked, it runs the pipeline. **OpenClaw** clones the repository (retrieving the PAT from Secrets Manager only now), **Repository Memory** provides continuity, and **Ollama** runs the local model.

```mermaid
sequenceDiagram
    participant N as n8n
    participant SM as Secrets Manager
    participant OC as OpenClaw
    participant M as Repository Memory
    participant OL as Ollama (Qwen)

    N->>SM: Get PAT (only now, when needed)
    N->>OC: Matched event + PAT
    OC->>OC: Checkout / sync repository
    OC->>OC: Structure, README, source, config analysis
    OC->>M: Look up prior analyses + published topics
    M-->>OC: Memory context (avoid duplicates)
    OC->>OC: Topic identification + outline
    loop each content type
        OC->>OL: Prompt with repo context + memory
        OL-->>OC: Generated content
    end
    OC->>N: Draft content set
    N->>N: Quality review → optional human approval
    N->>N: Publish + notify
    N->>M: Record published topics
```

**Optional human approval** (`REQUIRE_HUMAN_APPROVAL`) pauses before publishing for a human decision.

---

## 8. Content Generation

Each run can produce a bundle of assets rather than a single document.

```mermaid
flowchart TB
    A[Repository understanding + memory] --> P{Fan-out per content type}
    P --> A1[Technical blog post]
    P --> A2[README improvements]
    P --> A3[Project documentation]
    P --> A4[Architecture summary]
    P --> A5[API documentation]
    P --> A6[Project overview]
    P --> A7[Release notes]
    P --> A8[Changelog]
    P --> A9[Technical tutorial]
    A1 & A2 & A3 & A4 & A5 & A6 & A7 & A8 & A9 --> REV[Quality review]
    REV --> APP{Optional human approval}
    APP --> PKG[Render Markdown → publish]
```

All output is clean, portable **GitHub-flavoured Markdown**.

---

## 9. Data Flow

```mermaid
flowchart LR
    REG[Register: URL + PAT] --> RL[Registration Lambda]
    RL --> SM[(Secrets Manager)]
    RL --> DDB[(DynamoDB metadata)]
    RL --> HOOK[GitHub webhook created]

    EVT[GitHub push] -->|verify + validate trigger| LH[Webhook Handler]
    LH --> DDB
    LH --> SM
    LH -->|match → PutEvents| EB[(EventBridge)]
    LH -->|no match → 200| STOP[Ignored]
    EB --> SQS[(SQS)]
    SCH[EventBridge Scheduler<br/>18:00 / 20:00 daily] --> PWR[scheduled-start / scheduled-stop]
    PWR -.->|power| N8N
    SQS --> N8N[n8n]
    GH[(GitHub repo)] -->|clone w/ PAT| OC[OpenClaw]
    N8N --> OC
    OC <--> MEM[(Repository Memory)]
    OC -->|context| OL[Ollama / Qwen]
    OL -->|content| N8N
    N8N -->|review + approve| PUB[Publish]
    N8N -->|update last commit SHA| DDB
    N8N -->|notification| NOTIF[(Email / Slack / webhook)]
```

---

## 10. Storage Architecture

| Store | Contents | Notes |
| --- | --- | --- |
| **AWS Secrets Manager** | One shared secret (all repos' PATs + webhook secrets, keyed by owner/name) | Encrypted; retrieved only when needed; **never** in the database or logs |
| **Amazon DynamoDB** | Repository metadata (incl. secret reference) | Encrypted at rest; **never** stores the PAT |
| **gp3 EBS volume** | Ollama models, n8n state, **Repository Memory**, workflows | Persistent across start/stop; encrypted |
| Repository clones | Working copy during a run | Transient — discarded after the run |
| **Amazon SQS** | Buffered matched events + DLQ | Durable; retention up to 14 days |

Persisting models and Repository Memory on EBS makes the start/stop cost model viable; keeping PATs in Secrets Manager (with only references in DynamoDB) keeps credentials isolated and least-privilege. Published Markdown is written to whatever destination the deployment configures.
