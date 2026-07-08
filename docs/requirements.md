# Requirements

This document defines the requirements for the **GitHub AI Blog Generator** — an event-driven, fully self-hosted AI platform that automatically transforms GitHub repositories into high-quality technical content. Repository events trigger a **GitHub Webhook**; an **AWS Lambda** validates the signature and enqueues the payload in **Amazon SQS**; a cost-optimized **EC2 Spot Instance** is started on demand; and **n8n**, **OpenClaw**, and **Ollama** (running a local **Qwen** LLM) generate documentation and technical content — with **no paid inference APIs**.

Requirements use the following convention:

- **MUST** — mandatory for a compliant release.
- **SHOULD** — strongly recommended; may be deferred with justification.
- **MAY** — optional or future work.

Each requirement has a stable ID so it can be referenced from issues, pull requests, and tests:

| Prefix | Category |
| --- | --- |
| `FR-*` | Functional |
| `WH-*` | GitHub webhook |
| `AI-*` | AI / local inference |
| `WF-*` | Workflow orchestration |
| `NFR-*` | Non-functional |
| `INF-*` | Infrastructure |
| `SEC-*` | Security |
| `COST-*` | Cost optimisation |

Related: [Architecture](./architecture.md) · [Workflows](./workflows.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Cost Optimisation](./cost-optimization.md) · [Roadmap](./roadmap.md).

---

## 1. Functional Requirements

### 1.1 Triggering & Manual Execution

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-1.1 | The system MUST be triggered automatically by **GitHub Webhooks** on supported events. | MUST |
| FR-1.2 | The system MUST support **manual repository execution** for any repository on demand, without waiting for a webhook. | MUST |
| FR-1.3 | The system MUST accept a repository URL (and any required access token) for a manual run. | MUST |
| FR-1.4 | The system MUST enqueue every triggered event durably so it survives a stopped/starting instance. | MUST |

### 1.2 Repository Cloning & Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-2.1 | The system MUST clone (or update) the target repository before analysis. | MUST |
| FR-2.2 | The system MUST analyse the README, source code, folder structure, and project architecture using **OpenClaw**. | MUST |
| FR-2.3 | The system MUST analyse dependencies and package managers. | MUST |
| FR-2.4 | The system MUST analyse Infrastructure as Code, Docker files, and CI/CD configuration where present. | MUST |
| FR-2.5 | The system MUST detect the repository's technology stack from the analysis. | MUST |
| FR-2.6 | The system MUST respect ignore rules (`.gitignore`, binaries, vendored directories, size caps). | MUST |
| FR-2.7 | The analysis MUST be used to build the context supplied to the local LLM. | MUST |

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
| FR-3.8 | The system SHOULD generate **release notes** and **changelogs** for release events. | SHOULD |
| FR-3.9 | The system SHOULD generate **technical tutorials**. | SHOULD |
| FR-3.10 | Generated content MUST be emitted as valid **GitHub-flavoured Markdown**. | MUST |
| FR-3.11 | Each generation run MAY produce a bundle of multiple assets rather than a single document. | MAY |

### 1.4 Output, Publishing & Notifications

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-4.1 | The system MUST produce **Markdown output** for every generated asset. | MUST |
| FR-4.2 | The system MUST **publish** generated content to a configured destination. | MUST |
| FR-4.3 | The system MUST **notify users** when content is published. | MUST |
| FR-4.4 | The system MUST **notify users/operators** when a run fails. | MUST |

### 1.5 Reliability: Retry, Logging & Error Handling

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-5.1 | The system MUST support **retry** of transient failures with exponential backoff. | MUST |
| FR-5.2 | Messages that repeatedly fail MUST be routed to a **dead-letter queue**. | MUST |
| FR-5.3 | Every stage MUST handle failures explicitly and surface actionable errors. | MUST |
| FR-5.4 | The system MUST emit **structured logs** for every stage to CloudWatch. | MUST |
| FR-5.5 | A failed run MUST NOT overwrite or corrupt a previously generated output. | MUST |
| FR-5.6 | Processing MUST be **idempotent**: reprocessing the same event MUST NOT produce duplicate or corrupted output. | MUST |

---

## 2. GitHub Webhook Requirements

GitHub Webhooks are the **primary trigger mechanism**.

### 2.1 Delivery & Processing

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-1 | The system MUST expose a single **HTTPS** webhook endpoint via **Amazon API Gateway**. | MUST |
| WH-2 | The webhook handler MUST **return HTTP 200 immediately** so GitHub does not time out. | MUST |
| WH-3 | The webhook handler MUST **store the validated payload in Amazon SQS** before returning. | MUST |
| WH-4 | The webhook handler MUST **start the EC2 Spot Instance** if it is currently stopped. | MUST |
| WH-5 | The system MUST process queued events **asynchronously** on the EC2 instance via n8n. | MUST |
| WH-6 | The system MUST log every delivery (accepted and rejected) to CloudWatch. | MUST |
| WH-7 | No webhook event MUST be lost while the EC2 instance is stopped or starting (SQS durability). | MUST |

### 2.2 Signature Validation

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-8 | The system MUST validate the `X-Hub-Signature-256` header using **HMAC SHA-256** with the configured webhook secret. | MUST |
| WH-9 | The comparison MUST be **constant-time** to prevent timing attacks. | MUST |
| WH-10 | The system MUST **reject** deliveries with a missing or invalid signature and MUST NOT enqueue or process them. | MUST |
| WH-11 | The webhook secret MUST be provided via environment/secret configuration, never hardcoded in source. | MUST |

### 2.3 Supported Events

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-12 | The system MUST support the primary events: `push`, `release`, `pull_request`. | MUST |
| WH-13 | The system SHOULD ignore/skip events it does not handle without error. | SHOULD |
| WH-14 | The system MAY support additional events (e.g. `workflow_dispatch`, `issues`, `discussion`). | MAY |

---

## 3. AI / Local Inference Requirements

All AI inference MUST occur locally. The system MUST NOT depend on Amazon Bedrock, OpenAI, Anthropic, or any paid inference API.

| ID | Requirement | Priority |
| --- | --- | --- |
| AI-1 | The system MUST perform all inference **locally using Ollama**. | MUST |
| AI-2 | The default model MUST be a **local Qwen** model, configurable via environment variable. | MUST |
| AI-3 | The system MUST NOT require Amazon Bedrock, OpenAI, Anthropic, or any paid inference API. | MUST |
| AI-4 | **OpenClaw** MUST perform repository analysis and assemble the context supplied to the model. | MUST |
| AI-5 | The system MUST use distinct, purpose-tuned prompts for each content type. | MUST |
| AI-6 | Prompts MUST enforce a token/context budget and truncate deterministically when exceeded. | MUST |
| AI-7 | Model weights MUST persist on the EBS volume so they are not re-downloaded on each run. | MUST |
| AI-8 | The model, prompt templates, and generation parameters SHOULD be configurable without code changes. | SHOULD |
| AI-9 | The system SHOULD support swapping the local model (e.g. a larger Qwen variant) via configuration. | SHOULD |

---

## 4. Workflow Requirements

Orchestration is implemented with **n8n** running on the EC2 Spot Instance.

| ID | Requirement | Priority |
| --- | --- | --- |
| WF-1 | All stages (ingestion, analysis, generation, publishing, notifications) MUST be orchestrated through n8n workflows. | MUST |
| WF-2 | n8n MUST **poll Amazon SQS** for queued events and drive the pipeline from them. | MUST |
| WF-3 | Workflows MUST pass structured data between stages. | MUST |
| WF-4 | Workflows MUST support both webhook-triggered and **manual** invocation. | MUST |
| WF-5 | Each workflow MUST have explicit success and failure paths. | MUST |
| WF-6 | A processed SQS message MUST be deleted only after successful handling; failures MUST allow redelivery. | MUST |
| WF-7 | Workflows MUST be exportable/importable as versioned JSON, free of embedded secret values. | MUST |
| WF-8 | Each content type SHOULD be generatable independently for testing. | SHOULD |

---

## 5. Infrastructure Requirements

All infrastructure MUST be provisioned with **AWS CloudFormation**. **Terraform MUST NOT be used.** Templates MUST be modular and reusable.

| ID | Requirement | Priority |
| --- | --- | --- |
| INF-1 | Provision an **Amazon VPC**. | MUST |
| INF-2 | Provision a **public subnet**. | MUST |
| INF-3 | Provision an **Internet Gateway** and **route tables** for controlled connectivity. | MUST |
| INF-4 | Provision **security groups** that restrict inbound access to the minimum required. | MUST |
| INF-5 | Provision **IAM roles** and policies following least privilege. | MUST |
| INF-6 | Provision an **EC2 Spot Instance** running Ubuntu with Docker and Docker Compose. | MUST |
| INF-7 | Provision a **persistent gp3 EBS volume** for models, n8n state, and workflows. | MUST |
| INF-8 | Provision **Amazon API Gateway** as the HTTPS webhook ingress. | MUST |
| INF-9 | Provision **AWS Lambda** functions for the webhook handler and the idle-shutdown routine. | MUST |
| INF-10 | Provision **Amazon SQS** (with a dead-letter queue) as the durable event buffer. | MUST |
| INF-11 | Provision **Amazon EventBridge** to drive the idle-shutdown timer. | MUST |
| INF-12 | Provision **Amazon CloudWatch** for logs, metrics, and alarms. | MUST |
| INF-13 | The EC2 host MUST run **n8n**, **OpenClaw**, and **Ollama** via **Docker Compose**. | MUST |
| INF-14 | CloudFormation templates MUST be **modular and reusable** (separate network, serverless, compute, and observability stacks). | MUST |
| INF-15 | No resource MAY be created manually outside CloudFormation (no console drift). | MUST |

---

## 6. Security Requirements

See [Security](./security.md) for full detail.

| ID | Requirement | Priority |
| --- | --- | --- |
| SEC-1 | All IAM roles and policies MUST follow **least privilege** (the handler Lambda may only enqueue and start the instance; the shutdown Lambda may only stop it). | MUST |
| SEC-2 | GitHub webhook deliveries MUST be verified with **HMAC SHA-256** before processing ([WH-8](#22-signature-validation)). | MUST |
| SEC-3 | The webhook endpoint MUST be **HTTPS only**. | MUST |
| SEC-4 | **Security groups** MUST restrict inbound traffic; the n8n and Ollama ports MUST NOT be publicly exposed. | MUST |
| SEC-5 | Sensitive configuration MUST be provided via **environment variables** / a secrets store, never committed to source. | MUST |
| SEC-6 | Secrets (webhook secret, tokens) MUST be managed through a **secrets management** mechanism, not stored in plaintext. | MUST |
| SEC-7 | The EC2 instance MUST use **SSH key authentication**; password authentication MUST be disabled. | MUST |
| SEC-8 | There MUST be **no hardcoded credentials** in code, images, or CloudFormation templates/parameters. | MUST |
| SEC-9 | Data MUST be encrypted **at rest** (EBS, SQS, and any content storage) and **in transit** (TLS 1.2+). | MUST |
| SEC-10 | All access and activity MUST be logged to **CloudWatch**; logs MUST NOT contain secret material. | MUST |

---

## 7. Cost Optimisation Requirements

See [Cost Optimisation](./cost-optimization.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| COST-1 | Compute MUST be **event-driven**: the EC2 instance MUST NOT run continuously. | MUST |
| COST-2 | The compute host MUST be an **EC2 Spot Instance** to minimise cost. | MUST |
| COST-3 | The instance MUST be **started automatically** by the webhook handler Lambda when a valid event arrives. | MUST |
| COST-4 | The instance MUST be **stopped automatically** after a configurable **idle timeout** via EventBridge and the idle-shutdown Lambda. | MUST |
| COST-5 | The idle timeout MUST be configurable via CloudFormation parameter / environment variable. | MUST |
| COST-6 | Model weights and state MUST persist on **EBS** so the system pays only cheap storage while stopped and avoids re-downloading models. | MUST |
| COST-7 | **Amazon SQS** MUST buffer webhooks during cold start so no event is lost while the instance boots ([WH-7](#21-delivery--processing)). | MUST |
| COST-8 | Because inference is **local via Ollama**, the system MUST incur **no per-token inference charges**. | MUST |
| COST-9 | CloudWatch log retention MUST be bounded to limit storage cost. | MUST |
| COST-10 | The system SHOULD support falling back to On-Demand when Spot capacity is unavailable. | SHOULD |

---

## 8. Non-Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-1 | **Scalability** — the platform MUST process many repository events, buffered and drained through SQS. | MUST |
| NFR-2 | **Availability** — the webhook front door (API Gateway + Lambda + SQS) MUST remain available even when the EC2 instance is stopped. | MUST |
| NFR-3 | **Reliability** — runs MUST be idempotent and resumable after a Spot interruption. | MUST |
| NFR-4 | **Performance** — a typical repository SHOULD complete a generation run within a bounded time once the instance is warm. | SHOULD |
| NFR-5 | **Cost efficiency** — an idle system MUST cost only persistent storage (EBS), with no always-on compute. | MUST |
| NFR-6 | **Maintainability** — all infrastructure MUST be defined as code (CloudFormation); services MUST run reproducibly via Docker Compose. | MUST |
| NFR-7 | **Extensibility** — new content types, events, and workflow stages SHOULD be addable without rewriting existing ones. | SHOULD |
| NFR-8 | **Observability** — all components MUST emit structured logs and key metrics to CloudWatch. | MUST |
| NFR-9 | **Portability** — n8n workflows MUST be importable/exportable as versioned JSON; the AI stack MUST be provider-independent (local). | MUST |
| NFR-10 | **Usability** — a new contributor MUST be able to deploy the platform and process a repository from the documentation alone. | MUST |

---

## 9. Future Enhancements

Planned capabilities (see [Roadmap](./roadmap.md)). These are **future work** and not part of the current compliant release.

**Compute & resilience**

- On-Demand fallback when Spot capacity is unavailable
- Multi-Availability-Zone Spot placement
- GPU auto-detection and model right-sizing

**AI & content**

- Multi-model support (different local models per content type)
- Fine-tuned local models for documentation style
- Expanded content types (video/short/podcast scripts, diagrams)
- Multi-language content generation

**Publishing & platform**

- Direct publishing integrations (Dev.to, Medium, Hashnode)
- Web dashboard for run history and content review
- Multi-repository batch processing
- Expanded webhook event support ([WH-14](#23-supported-events))
