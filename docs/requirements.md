# Requirements

This document defines the requirements for the **AI GitHub Repository Blog Generator** — an AI-powered Developer Content Engine. Users **register repositories** (URL + Personal Access Token); the platform stores metadata, auto-configures **GitHub Webhooks**, analyzes repositories, and generates complete, publication-ready content packages using Amazon Bedrock.

Requirements use the following convention:

- **MUST** — mandatory for a compliant release.
- **SHOULD** — strongly recommended; may be deferred with justification.
- **MAY** — optional or future work.

Each requirement has a stable ID so it can be referenced from issues, pull requests, and tests:

| Prefix | Category |
| --- | --- |
| `FR-*` | Functional |
| `REG-*` | Repository registration |
| `GH-*` | GitHub API |
| `WH-*` | GitHub webhook |
| `AI-*` | AI |
| `INF-*` | Infrastructure |
| `SEC-*` | Security |
| `WF-*` | Workflow |
| `ST-*` | Storage |
| `MON-*` | Monitoring |
| `COST-*` | Cost optimization |
| `NFR-*` | Non-functional |

Related: [Architecture](./architecture.md) · [Workflows](./workflows.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [Roadmap](./roadmap.md).

---

## 1. Functional Requirements

### 1.1 Repository Analysis

The system MUST analyze the repository to build context for generation, covering:

| Area | Includes |
| --- | --- |
| Documentation | README, docs, API definitions |
| Code | Source code, folder structure, project architecture |
| Dependencies | Dependencies, package managers |
| Infrastructure as Code | Terraform, CloudFormation |
| Containers & orchestration | Docker files, Kubernetes manifests |
| CI/CD | Pipelines, GitHub Actions |
| Configuration | Configuration files |

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-1.1 | The system MUST clone (or update) the registered repository for analysis. | MUST |
| FR-1.2 | The system MUST analyze the README, source code, folder structure, and project architecture. | MUST |
| FR-1.3 | The system MUST analyze dependencies and package managers. | MUST |
| FR-1.4 | The system MUST analyze Infrastructure as Code (Terraform, CloudFormation), Docker files, and Kubernetes manifests. | MUST |
| FR-1.5 | The system MUST analyze CI/CD pipelines and GitHub Actions. | MUST |
| FR-1.6 | The system MUST analyze documentation, API definitions, and configuration files. | MUST |
| FR-1.7 | The system MUST detect the technology stack from the above. | MUST |
| FR-1.8 | The system MUST respect ignore rules (`.gitignore`, binaries, vendored dirs, size caps). | MUST |
| FR-1.9 | The analysis MUST be used to provide context to Amazon Bedrock. | MUST |

### 1.2 Content Generation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-2.1 | The system MUST generate a **complete content package**, not a single article. | MUST |
| FR-2.2 | The system MUST generate a generic technical blog article (`blog.md`). | MUST |
| FR-2.3 | The system MUST generate a Medium article (`medium.md`). | MUST |
| FR-2.4 | The system MUST generate a Dev.to article (`devto.md`). | MUST |
| FR-2.5 | The system MUST generate a Hashnode article (`hashnode.md`). | MUST |
| FR-2.6 | The system MUST generate a newsletter article (`newsletter.md`). | MUST |
| FR-2.7 | The system MUST generate a LinkedIn post (`linkedin.md`). | MUST |
| FR-2.8 | The system MUST generate an X (Twitter) thread (`twitter-thread.md`). | MUST |
| FR-2.9 | The system MUST generate a Reddit post (`reddit.md`). | MUST |
| FR-2.10 | The system MUST generate an FAQ (`faq.md`). | MUST |
| FR-2.11 | The system SHOULD generate README improvement suggestions (`readme-suggestions.md`). | SHOULD |
| FR-2.12 | The system MUST generate a cover-image prompt for AI image generation (`image-prompts.md`). | MUST |
| FR-2.13 | The system MUST generate SEO metadata: title, description, keywords (`seo.json`). | MUST |
| FR-2.14 | The system MUST generate package metadata: tags, reading time, social captions, call-to-action suggestions (`metadata.json`). | MUST |

### 1.3 Article Quality

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-3.1 | Each long-form article MUST include a title and, where supported, a subtitle. | MUST |
| FR-3.2 | Each long-form article MUST include an introduction, structured sections, and a conclusion. | MUST |
| FR-3.3 | Each long-form article SHOULD include a table of contents. | SHOULD |
| FR-3.4 | Each long-form article SHOULD include code examples and architecture explanations. | SHOULD |
| FR-3.5 | Each long-form article SHOULD cover best practices, challenges, and trade-offs. | SHOULD |
| FR-3.6 | Each long-form article MUST include a call to action. | MUST |
| FR-3.7 | Each long-form article MUST include platform-specific tags and estimated reading time. | MUST |
| FR-3.8 | Generated content MUST be valid Markdown (articles) or valid JSON (`seo.json`, `metadata.json`). | MUST |

### 1.4 Platform Targeting

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-4.1 | The system MUST produce platform-specific content optimized for Medium, Dev.to, and Hashnode. | MUST |
| FR-4.2 | The system MUST produce content suitable for personal blogs, company engineering blogs, and any Markdown-compatible platform. | MUST |

### 1.5 Event-to-Content Mapping

The content produced SHOULD be tailored to the triggering event.

| Event | Expected content |
| --- | --- |
| `push` | Updated technical article, updated documentation, LinkedIn update, X thread |
| `release` | Release blog, newsletter, LinkedIn article, release summary |
| `pull_request` | Feature article, architecture update, technical summary |
| `repository` (created) | Initial project overview, technology stack article, repository introduction |
| `workflow_dispatch` | On-demand full content package |

### 1.6 Error Handling & Retries

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-6.1 | Every stage MUST handle failures explicitly and surface actionable errors. | MUST |
| FR-6.2 | The system MUST retry transient failures (GitHub, Bedrock, S3, DynamoDB) with exponential backoff. | MUST |
| FR-6.3 | A failed run MUST NOT overwrite or corrupt a previously generated package. | MUST |

---

## 2. Repository Registration Requirements

Registration is the onboarding entry point (see [Workflows](./workflows.md#3-repository-registration)).

| ID | Requirement | Priority |
| --- | --- | --- |
| REG-1 | The system MUST accept a **repository URL** and a **GitHub Personal Access Token (PAT)** to register a repository. | MUST |
| REG-2 | The system MUST validate repository access before completing registration. | MUST |
| REG-3 | The system MUST store the PAT in **AWS Secrets Manager** and MUST NOT persist it in plaintext or in DynamoDB. | MUST |
| REG-4 | The system MUST store repository metadata in **DynamoDB** ([ST-7](#9-storage-requirements)). | MUST |
| REG-5 | The system MUST automatically create a GitHub Webhook **when the PAT grants permission**. | MUST |
| REG-6 | If webhook creation is not permitted, the system MUST complete registration and return manual webhook setup instructions. | MUST |
| REG-7 | The system MUST trigger an **initial repository analysis** after registration. | MUST |
| REG-8 | The registration endpoint MUST be **HTTPS only** and access-controlled. | MUST |
| REG-9 | The system SHOULD support re-registration/update of an existing repository (e.g. token rotation). | SHOULD |

---

## 3. GitHub API Requirements

The PAT is used to call the GitHub API and to clone repositories.

| ID | Requirement | Priority |
| --- | --- | --- |
| GH-1 | The system MUST use the PAT to access **private repositories** and clone repositories. | MUST |
| GH-2 | The system MUST read repository **contents**. | MUST |
| GH-3 | The system MUST read **releases**, **pull requests**, and **branches**. | MUST |
| GH-4 | The system MUST be able to **create GitHub Webhooks** via the API when permitted. | MUST |
| GH-5 | The system MUST handle GitHub API errors (rate limits, 401/403/404) gracefully and surface actionable messages. | MUST |
| GH-6 | The system SHOULD respect GitHub API rate limits and back off on `403`/`429`. | SHOULD |
| GH-7 | The system MUST request the **least-privilege** token scopes required (fine-grained tokens preferred). | MUST |

**Recommended token permissions**

| Fine-grained permission | Access | Classic scope |
| --- | --- | --- |
| Contents | Read | `repo` |
| Metadata | Read | `repo` |
| Pull requests | Read | `repo` |
| Webhooks | Read & write | `admin:repo_hook` |

---

## 4. GitHub Webhook Requirements

GitHub Webhooks are the **primary trigger mechanism** (see [Security → Webhook Security](./security.md#10-webhook-security)).

### 4.1 Delivery & Processing

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-1 | The system MUST expose a single **HTTPS** webhook endpoint to receive GitHub events. | MUST |
| WH-2 | The endpoint MUST accept the GitHub JSON payload (`application/json`). | MUST |
| WH-3 | On a valid delivery, the system MUST resolve the repository record from DynamoDB and extract event/ref info. | MUST |
| WH-4 | The system MUST start the analysis/generation pipeline from validated webhook data. | MUST |
| WH-5 | The system MUST respond promptly to GitHub (e.g. `202 Accepted`) and process asynchronously. | SHOULD |
| WH-6 | The system MUST log every delivery (accepted and rejected) to CloudWatch. | MUST |

### 4.2 Signature Validation

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-7 | The system MUST validate the `X-Hub-Signature-256` header using **HMAC SHA-256** with the repository's webhook secret. | MUST |
| WH-8 | The comparison MUST be constant-time to prevent timing attacks. | MUST |
| WH-9 | The system MUST **reject** deliveries with a missing or invalid signature (`401`) and MUST NOT process them. | MUST |
| WH-10 | The webhook secret MUST be stored in AWS Secrets Manager, never in code. | MUST |

### 4.3 Supported Events

| ID | Requirement | Priority |
| --- | --- | --- |
| WH-11 | The system MUST support the **primary** events: `push`, `release`, `pull_request`, `workflow_dispatch`, `repository`. | MUST |
| WH-12 | The system SHOULD ignore/skip events it does not handle without error. | SHOULD |
| WH-13 | The system MAY support **future** events: `issues`, `issue_comment`, `discussion`, `discussion_comment`, `deployment`, `deployment_status`, `package`, `registry_package`, `milestone`, `fork`, `watch` (star). | MAY |

---

## 5. AI Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| AI-1 | The system MUST use **Amazon Bedrock** foundation models for content generation. | MUST |
| AI-2 | The model ID/inference profile, `max_tokens`, and `temperature` MUST be configurable (CloudFormation parameters). | MUST |
| AI-3 | The system MUST assemble structured prompts from the repository analysis context. | MUST |
| AI-4 | Prompts MUST enforce a maximum token budget and truncate deterministically when exceeded. | MUST |
| AI-5 | The system MUST use distinct, platform-tuned prompts for each content type. | MUST |
| AI-6 | The system MUST retry Bedrock throttling/transient errors with exponential backoff. | MUST |
| AI-7 | The system SHOULD record token usage per invocation for cost tracking. | SHOULD |
| AI-8 | Prompt templates SHOULD be configurable without code changes. | SHOULD |

---

## 6. Infrastructure Requirements

All infrastructure MUST be provisioned with **AWS CloudFormation** (see [Infrastructure](./infrastructure.md)).

| ID | Requirement | Priority |
| --- | --- | --- |
| INF-1 | Provision an **Amazon VPC** with **public** and **private** subnets. | MUST |
| INF-2 | Provision an **Internet Gateway** and **route tables** for controlled connectivity. | MUST |
| INF-3 | Provision **security groups** restricting inbound 443 to GitHub webhook IP ranges and allowing SSM. | MUST |
| INF-4 | Provision **IAM roles** and **IAM policies** following least privilege. | MUST |
| INF-5 | Provision **Amazon EC2** to host the n8n orchestrator and the registration/webhook endpoint. | MUST |
| INF-6 | Provide access to **Amazon Bedrock** for content generation. | MUST |
| INF-7 | Provision **Amazon S3** for generated content (and a deployment artifacts bucket for CloudFormation packaging). | MUST |
| INF-8 | Provision **Amazon DynamoDB** for repository metadata. | MUST |
| INF-9 | Provision **AWS Secrets Manager** for per-repository PATs and webhook secrets. | MUST |
| INF-10 | Provision **Amazon CloudWatch** for logs, metrics, and alarms. | MUST |
| INF-11 | Provision **Amazon EventBridge** and **EventBridge Scheduler** for scheduling. | MUST |
| INF-12 | Provision **AWS Lambda** (Go) for scheduled EC2 start/stop. | MUST |
| INF-13 | CloudFormation packaging artifacts MUST be stored in a versioned, encrypted S3 bucket; stack state is managed by CloudFormation itself. | MUST |
| INF-14 | No resource MAY be created manually outside CloudFormation (no console drift). | MUST |

---

## 7. Security Requirements

See [Security](./security.md) for full detail.

| ID | Requirement | Priority |
| --- | --- | --- |
| SEC-1 | All IAM roles and policies MUST follow **least privilege**. | MUST |
| SEC-2 | The registration and webhook endpoints MUST be **HTTPS only**. | MUST |
| SEC-3 | GitHub webhook deliveries MUST be verified with **HMAC SHA-256** before processing ([WH-7](#42-signature-validation)). | MUST |
| SEC-4 | GitHub Personal Access Tokens MUST be stored in **AWS Secrets Manager** — never in DynamoDB, code, or plaintext. | MUST |
| SEC-5 | Data MUST be encrypted **at rest** (S3, EBS, DynamoDB, Secrets Manager). | MUST |
| SEC-6 | Data MUST be encrypted **in transit** (TLS 1.2+). | MUST |
| SEC-7 | There MUST be **no hardcoded credentials** in code, images, or state. | MUST |
| SEC-8 | Compute MUST use **IAM roles**, not static credentials; CI MUST use OIDC. | MUST |
| SEC-9 | All access and activity MUST be logged to **CloudWatch** (audit logging); logs MUST NOT contain secrets. | MUST |
| SEC-10 | **Amazon Bedrock** access MUST be scoped by IAM to specific model ARNs. | MUST |
| SEC-11 | **Amazon S3** access MUST be least-privilege; buckets MUST block public access and enforce TLS. | MUST |

---

## 8. Workflow Requirements

Orchestration is implemented with **n8n** (see [Workflows](./workflows.md)).

| ID | Requirement | Priority |
| --- | --- | --- |
| WF-1 | All stages (registration, ingestion, analysis, generation, publishing, notifications) MUST be orchestrated through n8n workflows. | MUST |
| WF-2 | Workflows MUST pass structured data between stages. | MUST |
| WF-3 | Workflows MUST be triggered by GitHub webhooks and MUST also support manual invocation. | MUST |
| WF-4 | Each workflow MUST have explicit success and failure paths. | MUST |
| WF-5 | Workflows MUST be exportable/importable as versioned JSON, free of embedded secret values. | MUST |
| WF-6 | Each content type SHOULD be generatable independently for testing. | SHOULD |

---

## 9. Storage Requirements

See [Architecture → Storage](./architecture.md#7-storage-architecture).

| ID | Requirement | Priority |
| --- | --- | --- |
| ST-1 | Generated content MUST be stored in **Amazon S3**. | MUST |
| ST-2 | Content MUST be organized by repository and date: `generated-content/<repository-name>/<YYYY-MM-DD>/`. | MUST |
| ST-3 | Each package MUST contain the full set of generated assets (articles, social posts, `seo.json`, `metadata.json`, image prompts). | MUST |
| ST-4 | The content bucket MUST enforce encryption at rest and block public access. | MUST |
| ST-5 | The content bucket SHOULD use versioning and lifecycle policies to bound cost. | SHOULD |
| ST-6 | CloudFormation packaging artifacts MUST be stored in a separate, versioned, encrypted S3 bucket. | MUST |
| ST-7 | Repository metadata MUST be stored in **DynamoDB**: repository URL, owner, name, default branch, registration timestamp, webhook status, webhook ID, last processed commit, last successful generation, generation status. | MUST |
| ST-8 | DynamoDB MUST NOT store GitHub Personal Access Tokens (store references/ARNs only). | MUST |
| ST-9 | The DynamoDB table MUST have encryption at rest enabled. | MUST |

---

## 10. Monitoring Requirements

See [Monitoring](./monitoring.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| MON-1 | All components MUST emit **structured logs** to CloudWatch. | MUST |
| MON-2 | Key metrics (registrations, webhook deliveries, runs started/succeeded/failed, latency, token usage) MUST be published to CloudWatch. | MUST |
| MON-3 | A run ID MUST correlate logs across all stages of a single run. | MUST |
| MON-4 | Failure alarms (webhook rejections, workflow failures, Bedrock failures, infra health) MUST notify operators. | MUST |
| MON-5 | An operational dashboard SHOULD summarize registration and run health at a glance. | SHOULD |
| MON-6 | Log retention MUST be bounded (default 14 days). | MUST |

---

## 11. Cost Optimization Requirements

See [Cost Optimization](./cost-optimization.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| COST-1 | The EC2 host MUST be automatically **started and stopped** to avoid running continuously. | MUST |
| COST-2 | Start/stop MUST be driven by **EventBridge Scheduler** invoking a **Go AWS Lambda**. | MUST |
| COST-3 | The default schedule MUST **start EC2 every day at 19:00** and **stop EC2 every day at 21:00**. | MUST |
| COST-4 | The start/stop schedule MUST be configurable via CloudFormation parameters. | MUST |
| COST-5 | S3 lifecycle policies MUST bound stored-content cost. | MUST |
| COST-6 | CloudWatch log retention MUST be bounded. | MUST |
| COST-7 | DynamoDB SHOULD use on-demand (pay-per-request) capacity to avoid idle cost. | SHOULD |
| COST-8 | Bedrock cost SHOULD be controlled via token budgets and capped `max_tokens`. | SHOULD |

---

## 12. Non-Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-1 | **Scalability** — the platform MUST support many registered repositories and process runs independently. | MUST |
| NFR-2 | **Availability** — storage, metadata, and AI stages MUST rely on managed, highly-available AWS services. | MUST |
| NFR-3 | **Reliability** — runs MUST be idempotent; re-running a failed job MUST NOT corrupt output. | MUST |
| NFR-4 | **Performance** — a single content package SHOULD complete within a bounded time for a typical repository. | SHOULD |
| NFR-5 | **Maintainability** — all infrastructure MUST be defined as code; Go code MUST pass `gofmt`, `go vet`, and tests in CI. | MUST |
| NFR-6 | **Extensibility** — new content types, events, and workflow stages SHOULD be addable without rewriting existing ones. | SHOULD |
| NFR-7 | **Observability** — all components MUST emit structured logs and key metrics. | MUST |
| NFR-8 | **Portability** — the workflows MUST be importable/exportable as versioned JSON. | MUST |
| NFR-9 | **Usability** — a new contributor MUST be able to run the system locally and register a repository from documentation alone. | MUST |

---

## 13. Future Enhancements

Planned capabilities (see [Roadmap](./roadmap.md)). These are **future work** and not part of the current compliant release.

**Authentication & multi-user**

- GitHub App authentication
- OAuth login
- Team workspaces
- Multi-user support

**Automatic publishing & CMS**

- Automatic publishing to Medium
- Automatic publishing to Dev.to
- Automatic publishing to Hashnode
- WordPress integration
- Ghost CMS integration

**Content breadth**

- Multi-language content generation
- AI-generated diagrams
- AI-generated release notes
- API documentation generation
- YouTube scripts
- TikTok scripts
- Shorts scripts
- Podcast summaries

**Intelligence & quality**

- Multi-model AI support
- Content quality scoring
- Duplicate content detection

**Automation & platform**

- Scheduled rescans
- Expanded webhook event support ([WH-13](#43-supported-events))
- Always-on webhook ingestion buffer (24/7 event capture)
