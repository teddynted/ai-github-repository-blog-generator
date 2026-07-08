# Requirements

This document defines the requirements for the **GitHub AI Blog Generator** — an event-driven, fully self-hosted AI platform that turns GitHub repositories into technical content **only when a commit message matches a configurable publishing trigger**.

For the **MVP**, users onboard a repository by providing a **GitHub Repository URL** and a **GitHub Personal Access Token (PAT)**. The platform validates access, creates a GitHub webhook, stores repository metadata, and stores the PAT securely in **AWS Secrets Manager**. GitHub App authentication is a **future enhancement** ([§15](#15-future-enhancements)).

When a matched event arrives, it is published to **Amazon EventBridge**, which starts a cost-optimized **EC2 Spot Instance** where **n8n**, **OpenClaw**, **Repository Memory**, and **Ollama** (a local **Qwen** model) generate content — with **no paid inference APIs**.

Requirements use the following convention:

- **MUST** — mandatory for a compliant release.
- **SHOULD** — strongly recommended; may be deferred with justification.
- **MAY** — optional or future work.

Each requirement has a stable ID:

| Prefix | Category |
| --- | --- |
| `FR-*` | Functional |
| `TRG-*` | Publishing trigger |
| `WHH-*` | Webhook handler |
| `WF-*` | Workflow orchestration |
| `MEM-*` | Repository Memory |
| `AI-*` | AI / local inference |
| `WH-*` | GitHub webhook (delivery/signature/events) |
| `REG-*` | Repository registration |
| `META-*` | Repository metadata |
| `CRED-*` | Secure credential storage |
| `INF-*` | Infrastructure |
| `SEC-*` | Security |
| `COST-*` | Cost optimisation |
| `NFR-*` | Non-functional |

Related: [Architecture](./architecture.md) · [Workflows](./workflows.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Cost Optimisation](./cost-optimization.md) · [Roadmap](./roadmap.md).

---

## 1. Functional Requirements

### 1.1 Triggering & Manual Execution

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-1.1 | The system MUST allow a user to **register a repository** with a URL and a GitHub PAT ([§12](#12-repository-registration-requirements-mvp)). | MUST |
| FR-1.2 | The system MUST receive GitHub webhook events on repository pushes. | MUST |
| FR-1.3 | The system MUST generate content **only** when a webhook event's commit message matches the configured publishing trigger ([§2](#2-publishing-trigger-requirements)). | MUST |
| FR-1.4 | The system MUST acknowledge and ignore all non-matching events (`HTTP 200`, no further processing). | MUST |
| FR-1.5 | The system MUST support **manual repository execution** on demand, without waiting for a matching commit. | MUST |
| FR-1.6 | The system MUST durably buffer every matched event so it survives a stopped/starting instance. | MUST |

### 1.2 Repository Cloning & Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-2.1 | The system MUST clone (or sync) the target repository before analysis, using the repository's PAT ([§14](#14-secure-credential-storage-requirements)). | MUST |
| FR-2.2 | The system MUST analyse the README, source code, folder structure, and project architecture using **OpenClaw**. | MUST |
| FR-2.3 | The system MUST analyse dependencies and package managers. | MUST |
| FR-2.4 | The system MUST analyse Infrastructure as Code, Docker files, and CI/CD configuration where present. | MUST |
| FR-2.5 | The system MUST detect the repository's technology stack. | MUST |
| FR-2.6 | The system MUST respect ignore rules (`.gitignore`, binaries, vendored directories, size caps). | MUST |
| FR-2.7 | The analysis MUST feed the context supplied to the local LLM, augmented by Repository Memory ([§5](#5-repository-memory-requirements)). | MUST |

### 1.3 Documentation & Content Generation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-3.1 | The system MUST generate content using a **local LLM via Ollama** — no external inference API. | MUST |
| FR-3.2 | The system MUST generate **technical blog posts**. | MUST |
| FR-3.3 | The system MUST generate **README** improvements/suggestions. | MUST |
| FR-3.4 | The system MUST generate **project documentation**. | MUST |
| FR-3.5 | The system MUST generate **architecture summaries**. | MUST |
| FR-3.6 | The system SHOULD generate **API documentation** where the repository exposes an API. | SHOULD |
| FR-3.7 | The system MUST generate **project overviews**. | MUST |
| FR-3.8 | The system SHOULD generate **release notes** and **changelogs**. | SHOULD |
| FR-3.9 | The system SHOULD generate **technical tutorials**. | SHOULD |
| FR-3.10 | The system MUST perform **topic identification** and **outline generation** before drafting. | MUST |
| FR-3.11 | Generated content MUST be emitted as valid **GitHub-flavoured Markdown**. | MUST |
| FR-3.12 | Each run MAY produce a bundle of multiple assets rather than a single document. | MAY |

### 1.4 Output, Publishing & Notifications

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-4.1 | The system MUST produce **Markdown output** for every generated asset. | MUST |
| FR-4.2 | The system MUST **publish** approved content to a configured destination. | MUST |
| FR-4.3 | The system MUST **notify users** when content is published. | MUST |
| FR-4.4 | The system MUST **notify users/operators** when a run fails or when approval is required. | MUST |

### 1.5 Quality Review & Approval

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-5.1 | The system MUST run a **quality review** of generated content before publishing. | MUST |
| FR-5.2 | The system MUST support an **optional human approval** gate (`REQUIRE_HUMAN_APPROVAL`) before publishing. | MUST |
| FR-5.3 | When approval is required, the system MUST NOT publish until content is explicitly approved. | MUST |
| FR-5.4 | A rejected draft MUST be discarded without publishing and MUST be logged. | MUST |

### 1.6 Reliability: Retry, Logging & Error Handling

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-6.1 | The system MUST support **retry** of transient failures with exponential backoff. | MUST |
| FR-6.2 | Messages that repeatedly fail MUST be routed to a **dead-letter queue**. | MUST |
| FR-6.3 | Every stage MUST handle failures explicitly and surface actionable errors. | MUST |
| FR-6.4 | The system MUST emit **structured logs** for every stage to CloudWatch. | MUST |
| FR-6.5 | A failed run MUST NOT overwrite or corrupt previously published output. | MUST |
| FR-6.6 | Processing MUST be **idempotent**: reprocessing the same event MUST NOT produce duplicate or corrupted output. | MUST |

---

## 2. Publishing Trigger Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| TRG-1 | Content generation MUST be initiated **only** when a webhook event's commit message matches the configured publishing trigger. | MUST |
| TRG-2 | The default trigger MUST be `blog:` (a commit message beginning with `blog:`). | MUST |
| TRG-3 | The trigger MUST be configurable — a platform default (`PublishTrigger`) and a per-repository **Trigger Pattern** stored in metadata ([META-4](#13-repository-metadata-requirements)). | MUST |
| TRG-4 | Events whose commit message does not match MUST be acknowledged with `HTTP 200` and **not** processed further. | MUST |
| TRG-5 | Trigger validation MUST occur **in the webhook handler, before** any compute is started or AI is invoked. | MUST |
| TRG-6 | The system supports per-repository custom trigger patterns — literal prefixes (`blog:`, `[blog]`) and `regex:`-prefixed regular expressions, validated at registration. | ✅ |
| TRG-7 | The system MAY support additional trigger sources (releases, tags, PR labels, manual, scheduled) that publish to the same event bus. | MAY |

**Examples**

| Commit message | Result |
| --- | --- |
| `blog: Added Repository Memory` | Triggers a run |
| `fix(api): resolve authentication issue` | Acknowledged & ignored |
| `refactor(core): simplify services` | Acknowledged & ignored |
| `docs: update README` | Acknowledged & ignored |

---

## 3. Webhook Handler Requirements

The webhook handler (AWS Lambda) is intentionally lightweight.

| ID | Requirement | Priority |
| --- | --- | --- |
| WHH-1 | The handler MUST resolve the **repository record** (metadata) for the delivery. | MUST |
| WHH-2 | The handler MUST retrieve the repository's **webhook secret** from Secrets Manager and verify the signature (HMAC SHA-256, constant-time) before anything else. | MUST |
| WHH-3 | The handler MUST parse the payload and extract commit information (message, ref, author). | MUST |
| WHH-4 | The handler MUST validate the commit message against the repository's **Trigger Pattern** ([§2](#2-publishing-trigger-requirements)). | MUST |
| WHH-5 | The handler MUST publish an event to **Amazon EventBridge only when the trigger matches**. | MUST |
| WHH-6 | The handler MUST return a successful HTTP response to GitHub as quickly as possible. | MUST |
| WHH-7 | The handler MUST NOT clone repositories, perform analysis, access Repository Memory, or invoke AI models. | MUST |
| WHH-8 | The handler MUST NOT retrieve or log the repository PAT. | MUST |

---

## 4. Workflow Requirements

Orchestration on the instance is implemented with **n8n**.

| ID | Requirement | Priority |
| --- | --- | --- |
| WF-1 | All on-instance stages (analysis, memory, generation, review, approval, publishing, notifications) MUST be orchestrated through n8n workflows. | MUST |
| WF-2 | The n8n workflow MUST be invoked from the matched event once the instance is healthy; matched events MUST be buffered in **Amazon SQS** so none is lost during cold start. | MUST |
| WF-3 | Workflows MUST pass structured data between stages. | MUST |
| WF-4 | Workflows MUST support both event-triggered and **manual** invocation. | MUST |
| WF-5 | Each workflow MUST have explicit success and failure paths. | MUST |
| WF-6 | A buffered event MUST be considered handled only after a successful run; failures MUST allow redelivery. | MUST |
| WF-7 | Workflows MUST be exportable/importable as versioned JSON, free of embedded secret values. | MUST |
| WF-8 | Each stage SHOULD be executable independently for testing. | SHOULD |

---

## 5. Repository Memory Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| MEM-1 | The system MUST maintain a **persistent, per-repository memory** of prior analyses and published topics. | MUST |
| MEM-2 | Repository Memory MUST persist on the **EBS volume** so it survives instance start/stop cycles. | MUST |
| MEM-3 | The pipeline MUST consult Repository Memory to **avoid regenerating duplicate content**. | MUST |
| MEM-4 | A completed run MUST record newly published topics into Repository Memory. | MUST |
| MEM-5 | Repository Memory MUST NOT store secrets. | MUST |

---

## 6. AI / Local Inference Requirements

All AI inference MUST occur locally. The system MUST NOT depend on Amazon Bedrock, OpenAI, Anthropic, or any paid inference API.

| ID | Requirement | Priority |
| --- | --- | --- |
| AI-1 | The system MUST perform all inference **locally using Ollama**. | MUST |
| AI-2 | The default model MUST be a **local Qwen** model, configurable via `OLLAMA_MODEL`. | MUST |
| AI-3 | The system MUST NOT require Amazon Bedrock, OpenAI, Anthropic, or any paid inference API. | MUST |
| AI-4 | **OpenClaw** MUST perform repository analysis and assemble the model context. | MUST |
| AI-5 | The system MUST use distinct, purpose-tuned prompts for each content type. | MUST |
| AI-6 | Prompts MUST enforce a token/context budget and truncate deterministically when exceeded. | MUST |
| AI-7 | Model weights MUST persist on the EBS volume so they are not re-downloaded on each run. | MUST |
| AI-8 | The model, prompt templates, and parameters SHOULD be configurable without code changes. | SHOULD |

---

## 7. GitHub Webhook Requirements

### 7.1 Delivery & Processing

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-1 | The system MUST expose a single **HTTPS** webhook endpoint via **Amazon API Gateway**. | MUST |
| WH-2 | The handler MUST **return HTTP 200 quickly** (both matched and ignored events). | MUST |
| WH-3 | On a matched event, the handler MUST **publish to EventBridge**, which starts the instance and buffers to SQS. | MUST |
| WH-4 | The system MUST process matched events **asynchronously** on the EC2 instance via n8n. | MUST |
| WH-5 | The system MUST log every delivery (accepted, ignored, and rejected) to CloudWatch. | MUST |
| WH-6 | No matched event MUST be lost while the EC2 instance is stopped or starting (SQS durability). | MUST |

### 7.2 Signature Validation

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-7 | The system MUST validate the `X-Hub-Signature-256` header using **HMAC SHA-256** with the **repository's webhook secret** (from Secrets Manager). | MUST |
| WH-8 | The comparison MUST be **constant-time** to prevent timing attacks. | MUST |
| WH-9 | The system MUST **reject** deliveries with a missing or invalid signature and MUST NOT evaluate or publish them. | MUST |
| WH-10 | Webhook secrets MUST be stored in Secrets Manager, never hardcoded in source. | MUST |

### 7.3 Supported Events

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-11 | The system MUST support `push` events (commit-message gated) and **published `release`** events (a release always triggers). | MUST |
| WH-12 | The system SHOULD ignore/skip event types it does not handle without error. | SHOULD |
| WH-13 | The system MAY support further trigger sources (tags, PR labels) in future. | MAY |

---

## 8. Infrastructure Requirements

All infrastructure MUST be provisioned with **AWS CloudFormation**. **Terraform MUST NOT be used.** Templates MUST be modular and reusable.

| ID | Requirement | Priority |
| --- | --- | --- |
| INF-1 | Provision an **Amazon VPC** with a **public subnet**, **Internet Gateway**, and **route tables**. | MUST |
| INF-2 | Provision **security groups** restricting inbound access to the minimum required. | MUST |
| INF-3 | Provision **IAM roles** and policies following least privilege. | MUST |
| INF-4 | Provision an **EC2 Spot Instance** running Ubuntu with Docker and Docker Compose. | MUST |
| INF-5 | Provision a **persistent gp3 EBS volume** for models, n8n state, and Repository Memory. | MUST |
| INF-6 | Provision **Amazon API Gateway** for the webhook ingress and the **registration** endpoint. | MUST |
| INF-7 | Provision **AWS Lambda** functions: registration, webhook handler, instance starter, idle shutdown. | MUST |
| INF-8 | Provision **Amazon EventBridge** (event bus + rules) for matched events and the idle timer. | MUST |
| INF-9 | Provision **Amazon SQS** (with a dead-letter queue) as the durable event buffer. | MUST |
| INF-10 | Provision **AWS Secrets Manager** for GitHub PATs and per-repository webhook secrets. | MUST |
| INF-11 | Provision a **metadata store** (Amazon DynamoDB) for repository metadata. | MUST |
| INF-12 | Provision **Amazon CloudWatch** for logs, metrics, and alarms. | MUST |
| INF-13 | The EC2 host MUST run **n8n**, **OpenClaw**, and **Ollama** via **Docker Compose**. | MUST |
| INF-14 | CloudFormation templates MUST be **modular and reusable**. | MUST |
| INF-15 | No resource MAY be created manually outside CloudFormation (no console drift). | MUST |

---

## 9. Security Requirements

See [Security](./security.md) for full detail.

| ID | Requirement | Priority |
| --- | --- | --- |
| SEC-1 | All IAM roles and policies MUST follow **least privilege**. | MUST |
| SEC-2 | GitHub webhook deliveries MUST be verified with **HMAC SHA-256** before evaluation ([WH-7](#72-signature-validation)). | MUST |
| SEC-3 | The webhook and registration endpoints MUST be **HTTPS only**. | MUST |
| SEC-4 | **Security groups** MUST restrict inbound traffic; the n8n and Ollama ports MUST NOT be publicly exposed. | MUST |
| SEC-5 | GitHub **PATs MUST be stored in AWS Secrets Manager** and MUST NOT be stored in plaintext, config, source, environment variables, or the metadata database ([§14](#14-secure-credential-storage-requirements)). | MUST |
| SEC-6 | Secrets MUST be managed through Secrets Manager, retrieved only when required. | MUST |
| SEC-7 | The EC2 instance MUST use **SSH key authentication**; password authentication MUST be disabled. | MUST |
| SEC-8 | There MUST be **no hardcoded credentials** in code, images, or templates. | MUST |
| SEC-9 | Data MUST be encrypted **at rest** (EBS, SQS, DynamoDB, Secrets Manager) and **in transit** (TLS 1.2+). | MUST |
| SEC-10 | All access and activity MUST be logged to **CloudWatch**; logs MUST NOT contain PATs or other secret material. | MUST |

---

## 10. Cost Optimisation Requirements

See [Cost Optimisation](./cost-optimization.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| COST-1 | **Repository access and token permissions MUST be validated at registration**, before any AI execution. | MUST |
| COST-2 | **Trigger pre-filtering** MUST occur before any compute or AI is invoked. | MUST |
| COST-3 | Compute MUST be **event-driven**: the EC2 instance MUST NOT run continuously. | MUST |
| COST-4 | The compute host MUST be an **EC2 Spot Instance**, started only on a matched event. | MUST |
| COST-5 | The instance MUST be **stopped automatically** after a configurable idle timeout / workflow completion. | MUST |
| COST-6 | Models, state, and Repository Memory MUST persist on **EBS** to avoid re-downloading models. | MUST |
| COST-7 | **Amazon SQS** MUST buffer matched events during cold start so none is lost. | MUST |
| COST-8 | GitHub secrets MUST be retrieved **only when necessary**. | MUST |
| COST-9 | Because inference is **local via Ollama**, the system MUST incur **no per-token inference charges**. | MUST |
| COST-10 | Lightweight processing MUST use **serverless** AWS services (API Gateway, Lambda, EventBridge, SQS). | MUST |
| COST-11 | CloudWatch log retention MUST be bounded. | MUST |

---

## 11. Non-Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-1 | **Scalability** — the platform MUST process many matched events, buffered through SQS, across many registered repositories. | MUST |
| NFR-2 | **Availability** — the front door (API Gateway + Lambda + EventBridge + SQS) MUST remain available even when the EC2 instance is stopped. | MUST |
| NFR-3 | **Reliability** — runs MUST be idempotent and resumable after a Spot interruption. | MUST |
| NFR-4 | **Performance** — a typical repository SHOULD complete a run within a bounded time once the instance is warm. | SHOULD |
| NFR-5 | **Cost efficiency** — an idle system MUST cost only persistent storage (EBS) and cheap managed services. | MUST |
| NFR-6 | **Maintainability** — all infrastructure MUST be defined as code; services MUST run reproducibly via Docker Compose. | MUST |
| NFR-7 | **Developer control** — routine commits MUST never generate content; only explicitly triggered commits MUST. | MUST |
| NFR-8 | **Extensibility** — new content types, trigger sources, and stages SHOULD be addable without rewriting existing ones; the design MUST allow evolving from PAT onboarding to GitHub Apps. | SHOULD |
| NFR-9 | **Observability** — all components MUST emit structured logs and key metrics. | MUST |
| NFR-10 | **Portability** — n8n workflows MUST be importable/exportable as versioned JSON; the AI stack MUST be provider-independent (local). | MUST |

---

## 12. Repository Registration Requirements (MVP)

Onboarding uses **GitHub Personal Access Tokens (PATs)**. GitHub App authentication is a future enhancement ([§15](#15-future-enhancements)).

| ID | Requirement | Priority |
| --- | --- | --- |
| REG-1 | The system MUST allow a user to register a repository by providing a **repository URL** and a **GitHub PAT**. | MUST |
| REG-2 | The system MUST **validate repository access** before completing registration. | MUST |
| REG-3 | The system MUST **validate that the PAT has the required permissions** (repository contents, metadata, webhook management). | MUST |
| REG-4 | The system MUST **create a GitHub webhook** — pointing at the API Gateway endpoint — using the supplied PAT. | MUST |
| REG-5 | The system MUST configure a **webhook secret** for signature verification. | MUST |
| REG-6 | The webhook MUST subscribe **only to the events required for the MVP** (e.g. `push`). | MUST |
| REG-7 | The system MUST **store repository metadata** ([§13](#13-repository-metadata-requirements)). | MUST |
| REG-8 | The system MUST **store the PAT securely in AWS Secrets Manager** ([§14](#14-secure-credential-storage-requirements)) and MUST NOT store it in plaintext. | MUST |
| REG-9 | The registration endpoint MUST be **HTTPS only** and access-controlled. | MUST |
| REG-10 | The PAT is required for: accessing repository contents, registering webhooks, reading repository metadata, and future repository synchronisation. | MUST |
| REG-11 | The system SHOULD support **re-registration / token rotation** for an existing repository. | SHOULD |

**Registration flow (MVP):** enter URL → enter PAT → validate repository access → validate token permissions → create webhook → store metadata → store PAT in Secrets Manager → repository registered.

---

## 13. Repository Metadata Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| META-1 | The system MUST store per-repository metadata in an application database (**Amazon DynamoDB**). | MUST |
| META-2 | Metadata MUST include: Repository ID, Owner, Name, URL, Default Branch, Webhook ID, **Trigger Pattern**, Enabled Status, **Secret Reference** (Secrets Manager ARN/name), Last Processed Commit SHA, Registration Timestamp. | MUST |
| META-3 | The metadata store MUST NOT contain the GitHub PAT — only the **secret reference**. | MUST |
| META-4 | The per-repository **Trigger Pattern** MUST default to `blog:` and MUST be read by the webhook handler. | MUST |
| META-5 | The metadata store MUST be encrypted at rest. | MUST |
| META-6 | The system MUST update **Last Processed Commit SHA** after a successful run. | MUST |

---

## 14. Secure Credential Storage Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| CRED-1 | GitHub PATs MUST be stored as secrets in **AWS Secrets Manager**. | MUST |
| CRED-2 | The application MUST store only a **reference** (Secret ARN/Name), never the token value, in the metadata database. | MUST |
| CRED-3 | The PAT MUST be **retrieved only when required** (clone, webhook management, metadata reads). | MUST |
| CRED-4 | IAM access to secrets MUST follow **least privilege**, scoped to specific secret ARNs. | MUST |
| CRED-5 | PATs MUST NOT be stored in configuration files, source code, environment variables, or the metadata database. | MUST |
| CRED-6 | PATs MUST **never be logged or exposed**. | MUST |
| CRED-7 | Each repository's **webhook signing secret** MUST also be stored in Secrets Manager. | MUST |
| CRED-8 | **Secret rotation** SHOULD be supported (future enhancement). | SHOULD |

---

## 15. Future Enhancements

Planned capabilities (see [Roadmap](./roadmap.md)). These are **future work**, clearly distinct from the MVP.

**Authentication & onboarding**

- **GitHub App authentication** (recommended long-term approach, replacing per-repo PATs)
- OAuth login with GitHub
- Automatic webhook management
- Fine-grained repository permissions
- Secret rotation
- Re-registration / token rotation UX

**Trigger system**

- Additional trigger sources: GitHub Releases, Git Tags, Pull Request labels
- Manual blog generation from the application
- Scheduled repository summaries

**Scale & platform**

- Support for multiple repositories per user
- Repository groups and organisations
- Multi-user workspaces
- Web-based repository management dashboard
- On-Demand fallback when Spot capacity is unavailable
- Multi-model support and fine-tuned local models
