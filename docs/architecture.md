# Architecture

This document describes the architecture of the **GitHub AI Blog Generator**: the design principles, the **commit-message trigger gate**, the event-driven trigger path, the AWS deployment topology, the analysis and local-inference pipeline, content generation, data flow, and storage.

Related: [Requirements](./requirements.md) · [Infrastructure](./infrastructure.md) · [Workflows](./workflows.md) · [Cost Optimisation](./cost-optimization.md).

---

## 1. Design Principles

- **Opt-in generation** — a webhook is received on every push, but a run happens **only** when the commit message matches a configurable publishing trigger (default `blog:`). All other events are acknowledged and ignored.
- **Event-driven** — matched events are published to **Amazon EventBridge**, which starts compute and buffers work. Nothing runs speculatively.
- **Self-hosted inference** — all AI runs locally via **Ollama** with a local **Qwen** model. No Amazon Bedrock, OpenAI, Anthropic, or any paid inference API.
- **Pay only when you compute** — a cost-optimized **EC2 Spot Instance** starts on a matched event and stops after an idle timeout.
- **Durable buffering** — every matched event is stored in **Amazon SQS** so nothing is lost while the instance is stopped or booting.
- **Persistent state, ephemeral compute** — models, n8n state, and **Repository Memory** live on a persistent **gp3 EBS volume**; the compute layer is disposable.
- **Everything as code** — infrastructure is AWS CloudFormation; workflows are versioned n8n JSON; services run via Docker Compose.
- **Least privilege & encryption everywhere** — each component gets only the permissions it needs.

---

## 2. Commit-Message Trigger Gate

The trigger gate is the platform's defining architectural decision. It lives in the **Webhook Handler Lambda** and runs before any compute or AI is invoked.

```mermaid
flowchart TB
    W[Webhook delivery] --> SIG{Valid HMAC signature?}
    SIG -- no --> R401[Return 401 + log rejection]
    SIG -- yes --> EX[Extract commit message + repo]
    EX --> T{Commit message matches<br/>publish trigger? default 'blog:'}
    T -- no --> ACK[Return HTTP 200<br/>acknowledge & ignore — no further processing]
    T -- yes --> PUT[PutEvents → Amazon EventBridge]
    PUT --> OK[Return HTTP 200]
```

- The trigger pattern is configurable via `PublishTrigger` (default `blog:`). Custom patterns (`[blog]`, regex, per-repo rules) are [planned](./roadmap.md).
- A non-matching event is a **terminal success**: the handler returns `200` and does nothing else. This is what prevents unwanted processing and cost.
- Only a **matching** event is published to EventBridge, which begins the rest of the flow.

See [Cost Optimisation → Trigger Pre-Filtering](./cost-optimization.md#1-trigger-pre-filtering) for why this is a primary cost lever, and [Requirements → Publishing Trigger](./requirements.md#2-publishing-trigger-requirements).

---

## 3. High-Level Architecture

```mermaid
flowchart TB
    subgraph GitHub
        EVT[GitHub Webhook<br/>push]
        REPO[(Target Repository)]
    end

    subgraph AWS
        APIGW[Amazon API Gateway]
        LH[Lambda: Webhook Handler<br/>verify + validate trigger]
        EB[Amazon EventBridge<br/>event bus + rule]
        SQS[(Amazon SQS<br/>+ dead-letter queue)]
        ST[Lambda: Instance Starter]
        IDLE[EventBridge idle timer]
        LS[Lambda: Idle Shutdown]
        subgraph EC2["EC2 Spot Instance (Ubuntu + Docker Compose)"]
            N8N[n8n Orchestrator]
            OC[OpenClaw]
            MEM[(Repository Memory)]
            OLL[Ollama + Qwen]
        end
        EBS[(Persistent gp3 EBS Volume)]
        CW[Amazon CloudWatch]
    end

    EVT -->|HTTPS + HMAC| APIGW --> LH
    LH -->|trigger match → PutEvents| EB
    LH -->|no match → 200| EVT
    EB --> SQS
    EB --> ST -->|StartInstances if stopped| EC2
    N8N -->|poll| SQS
    N8N --> OC --> MEM
    OC --> OLL
    OLL -->|content| N8N
    N8N -->|publish + notify| OUT[Published content / users]
    EBS --- EC2
    IDLE --> LS -->|StopInstances on idle| EC2
    LH -. logs .-> CW
    N8N -. logs .-> CW
```

**Component responsibilities**

| Component | Responsibility |
| --- | --- |
| GitHub Webhook | Delivers push events over HTTPS (every push, matched or not) |
| API Gateway | Public HTTPS ingress |
| Webhook Handler (Lambda) | Verify signature, extract commit message, **validate trigger**, publish matched events to EventBridge, return 200 quickly. Does **no** analysis or inference |
| Amazon EventBridge | Central event bus for matched events; routes to SQS and the instance starter; extension point for future trigger sources |
| Amazon SQS | Durable buffer so no matched event is lost during cold start; visibility timeout + DLQ |
| Instance Starter (Lambda) | Starts the EC2 Spot Instance if stopped |
| EC2 Spot Instance | Cost-optimized host running n8n, OpenClaw, Ollama via Docker Compose |
| n8n (on EC2) | Polls SQS, orchestrates the full pipeline |
| OpenClaw | Clones and analyses the repository, assembles context |
| Repository Memory | Persistent per-repo record of prior analyses and published topics |
| Ollama + Qwen | Local LLM inference — no external API |
| Idle Shutdown (Lambda) | Stops the instance after a configurable idle timeout |
| CloudWatch | Central logs, metrics, and alarms |

---

## 4. Webhook Handler Responsibilities

The handler is intentionally **lightweight** so it returns to GitHub in milliseconds. It is limited to:

1. Verify the GitHub webhook signature (HMAC SHA-256, constant-time).
2. Parse the webhook payload.
3. Extract commit information (message, ref, author).
4. Determine the affected repository.
5. **Validate the configured commit-message trigger.**
6. **Publish an event to EventBridge only when the trigger matches.**
7. Return a successful HTTP response to GitHub as quickly as possible.

The handler **must not** clone repositories, perform analysis, invoke Repository Memory, or run AI models — all of that happens downstream on the EC2 instance ([Requirements → Webhook Handler](./requirements.md#3-webhook-handler-requirements)).

---

## 5. AWS Deployment Topology

The webhook front door (API Gateway + Lambda + EventBridge + SQS) is fully serverless and **always available even when the EC2 instance is stopped**. The compute host runs in a **public subnet**; inbound access is tightly restricted by security groups.

```mermaid
flowchart TB
    GH[GitHub Webhook] -->|443 HTTPS| APIGW[API Gateway]
    APIGW --> LH[Webhook Handler Lambda]
    LH --> EB[EventBridge]
    EB --> SQS[(SQS)]
    EB --> ST[Instance Starter Lambda]
    subgraph VPC["Amazon VPC 10.0.0.0/16"]
        IGW[Internet Gateway]
        subgraph Public["Public subnet 10.0.0.0/24"]
            EC2[(EC2 Spot Instance<br/>n8n + OpenClaw + Ollama)]
            EBS[(gp3 EBS volume)]
        end
    end
    ST -->|StartInstances| EC2
    EC2 --- EBS
    EC2 --- IGW
    EC2 -->|poll| SQS
    ADMIN[Operator] -->|SSH key auth<br/>restricted CIDR| EC2
```

- **Ingress:** GitHub reaches the platform through **API Gateway** (managed TLS), not the instance. The instance's inbound is limited to **SSH (22) from operator IPs**; the **n8n and Ollama ports are never publicly exposed**.
- **Egress:** the **Internet Gateway** provides outbound access for cloning, image pulls, and model downloads.
- **Decoupling:** the instance never receives inbound webhook traffic — it **polls SQS** when up.

See [Infrastructure](./infrastructure.md) and [Security](./security.md).

---

## 6. Analysis & Generation Pipeline

Once the instance is up, n8n drains SQS and runs the pipeline. **OpenClaw** analyses the repository, **Repository Memory** provides continuity, and **Ollama** runs the local model.

```mermaid
sequenceDiagram
    participant N as n8n
    participant OC as OpenClaw
    participant M as Repository Memory
    participant OL as Ollama (Qwen)

    N->>OC: Matched event from SQS
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
    N->>N: Quality review
    N->>N: Optional human approval
    N->>N: Publish + notify
    N->>M: Record published topics
```

**Repository Memory** is a persistent, per-repository record (stored on the EBS volume) of previous analyses, identified topics, and published posts. It gives runs continuity — so the platform avoids regenerating duplicate content and can build on what it has already written ([Requirements → Repository Memory](./requirements.md#5-repository-memory-requirements)).

**Optional human approval** is a configurable gate (`REQUIRE_HUMAN_APPROVAL`) that pauses the pipeline for a human to approve or reject content before it is published.

---

## 7. Content Generation

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

All output is clean, portable **GitHub-flavoured Markdown**. A failure on a content type routes to the error/notification path without corrupting previously generated output ([FR-5.5](./requirements.md#16-reliability-retry-logging--error-handling)).

---

## 8. Data Flow

```mermaid
flowchart LR
    EVT[GitHub push] -->|verify + validate trigger| LH[Webhook Handler]
    LH -->|match → PutEvents| EB[(EventBridge)]
    LH -->|no match → 200| STOP[Ignored]
    EB --> SQS[(SQS)]
    EB --> ST[Instance Starter]
    SQS -->|poll| N8N[n8n]
    GH[(GitHub repo)] -->|clone| OC[OpenClaw]
    N8N --> OC
    OC <--> MEM[(Repository Memory)]
    OC -->|context| OL[Ollama / Qwen]
    OL -->|content| N8N
    N8N -->|review + approve| PUB[Publish]
    N8N -->|notification| NOTIF[(Email / Slack / webhook)]
```

**Stages and payloads**

| Stage | Input | Output |
| --- | --- | --- |
| Trigger gate | webhook payload | matched event → EventBridge, or 200 + stop |
| Route & buffer | matched event | SQS message + instance start |
| Poll | SQS message | in-flight run on EC2 |
| Analyse | working copy + memory | structured repository understanding |
| Generate | context + prompts | per-type Markdown content |
| Review & approve | draft content | approved content (or rejection) |
| Publish & notify | approved content | published content + notification + memory update |

---

## 9. Storage Architecture

The only **persistent** store is the **gp3 EBS volume** attached to the instance.

```mermaid
flowchart TB
    subgraph Persistent["gp3 EBS volume"]
        MODELS[Ollama models]
        STATE[n8n state + credentials]
        MEM[Repository Memory]
        WF[Exported workflows]
    end
    subgraph Transient
        CLONE[Repository clones<br/>instance disk during a run]
    end
    subgraph Managed
        EB[(EventBridge)]
        SQS[(SQS buffer + DLQ)]
    end
```

| Store | Contents | Notes |
| --- | --- | --- |
| gp3 EBS volume | Ollama models, n8n state, **Repository Memory**, workflows | Survives start/stop; encrypted; avoids re-downloading models |
| Repository clones | Working copy during a run | Transient — discarded after the run |
| Amazon SQS | Buffered matched events + DLQ | Durable; retention up to 14 days |

Persisting models and Repository Memory on EBS is what makes the start/stop cost model viable and gives runs continuity. Published Markdown is written to whatever destination the deployment configures (a Git repository, an object store, or a CMS). The platform does not mandate a specific content store.
