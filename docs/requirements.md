# Requirements

This document defines the requirements for the **AI GitHub Repository Blog Generator** — an AI-powered developer content engine that analyzes GitHub repositories and generates complete, publication-ready content packages using Amazon Bedrock.

Requirements use the following convention:

- **MUST** — mandatory for a compliant release.
- **SHOULD** — strongly recommended; may be deferred with justification.
- **MAY** — optional or future work.

Each requirement has a stable ID so it can be referenced from issues, pull requests, and tests:

| Prefix | Category |
| --- | --- |
| `FR-*` | Functional |
| `NFR-*` | Non-functional |
| `INF-*` | Infrastructure |
| `SEC-*` | Security |
| `AI-*` | AI |
| `WF-*` | Workflow |
| `ST-*` | Storage |
| `MON-*` | Monitoring |
| `COST-*` | Cost optimization |

Related: [Architecture](./architecture.md) · [Workflows](./workflows.md) · [Infrastructure](./infrastructure.md) · [Roadmap](./roadmap.md).

---

## 1. Functional Requirements

### 1.1 Repository Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-1.1 | The system MUST accept a GitHub repository as input to a generation run. | MUST |
| FR-1.2 | The system MUST automatically clone the target repository for analysis. | MUST |
| FR-1.3 | The system SHOULD perform a shallow clone to minimize time and storage. | SHOULD |
| FR-1.4 | The system MUST analyze the repository structure (file tree, modules, layout). | MUST |
| FR-1.5 | The system MUST locate and analyze the README to extract purpose and usage. | MUST |
| FR-1.6 | The system MUST analyze representative source code files within a token budget. | MUST |
| FR-1.7 | The system MUST analyze configuration files (`package.json`, `go.mod`, `Dockerfile`, `*.tf`, CI configs, …). | MUST |
| FR-1.8 | The system MUST detect the technology stack (languages, frameworks, build tools, infrastructure). | MUST |
| FR-1.9 | The system MUST synthesize an understanding of the project architecture from the above. | MUST |
| FR-1.10 | The system MUST respect ignore rules (`.gitignore`, binaries, vendored dirs, size caps). | MUST |

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
| FR-3.1 | Each long-form article MUST include an engaging title (and subtitle where the platform supports one). | MUST |
| FR-3.2 | Each long-form article MUST include an introduction, well-organized sections, and a conclusion. | MUST |
| FR-3.3 | Each long-form article SHOULD include a table of contents. | SHOULD |
| FR-3.4 | Each long-form article SHOULD include code examples where appropriate. | SHOULD |
| FR-3.5 | Each long-form article SHOULD cover architecture explanations, best practices, and challenges/trade-offs. | SHOULD |
| FR-3.6 | Each long-form article MUST include a call to action. | MUST |
| FR-3.7 | Each long-form article MUST include platform-specific tags and estimated reading time. | MUST |
| FR-3.8 | Generated content MUST be valid Markdown (articles) or valid JSON (`seo.json`, `metadata.json`). | MUST |

### 1.4 Platform Targeting

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-4.1 | The system MUST produce platform-specific content optimized for Medium, Dev.to, and Hashnode. | MUST |
| FR-4.2 | The system MUST produce content suitable for personal blogs, company engineering blogs, and any Markdown-compatible platform. | MUST |

### 1.5 GitHub Event Triggers

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-5.1 | The system MUST support triggering on a **push** event. | MUST |
| FR-5.2 | The system MUST support triggering on a **pull request** event. | MUST |
| FR-5.3 | The system MUST support triggering on a **release** event. | MUST |
| FR-5.4 | The system SHOULD support triggering on **repository creation**. | SHOULD |
| FR-5.5 | The system MUST support **workflow dispatch** (from GitHub Actions). | MUST |
| FR-5.6 | The system MUST support **manual** on-demand triggering. | MUST |

### 1.6 Error Handling & Retries

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-6.1 | Every stage MUST handle failures explicitly and surface actionable errors. | MUST |
| FR-6.2 | The system MUST retry transient failures (GitHub, Bedrock, S3) with exponential backoff. | MUST |
| FR-6.3 | A failed run MUST NOT overwrite or corrupt a previously generated package. | MUST |

---

## 2. Non-Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-1 | **Scalability** — compute-heavy work MUST be able to scale horizontally or process repositories independently. | MUST |
| NFR-2 | **Availability** — storage and AI stages MUST rely on managed, highly-available AWS services. | MUST |
| NFR-3 | **Reliability** — runs MUST be idempotent; re-running a failed job MUST NOT corrupt output. | MUST |
| NFR-4 | **Performance** — a single content package SHOULD complete within a bounded time for a typical repository, enforced by token/file budgets. | SHOULD |
| NFR-5 | **Maintainability** — all infrastructure MUST be defined as code; Go code MUST pass `gofmt`, `go vet`, and tests in CI. | MUST |
| NFR-6 | **Extensibility** — new content types and workflow stages SHOULD be addable without rewriting existing ones. | SHOULD |
| NFR-7 | **Observability** — all components MUST emit structured logs and key metrics. | MUST |
| NFR-8 | **Portability** — the workflows MUST be importable/exportable as versioned JSON. | MUST |
| NFR-9 | **Beginner-friendliness** — a new contributor MUST be able to run the system locally from documentation alone. | MUST |

---

## 3. Infrastructure Requirements

All infrastructure MUST be provisioned with **Terraform** (see [Infrastructure](./infrastructure.md)).

| ID | Requirement | Priority |
| --- | --- | --- |
| INF-1 | Provision an **Amazon VPC** with **public** and **private** subnets. | MUST |
| INF-2 | Provision an **Internet Gateway** and **route tables** for controlled connectivity. | MUST |
| INF-3 | Provision **security groups** restricting traffic to only what is required. | MUST |
| INF-4 | Provision **IAM roles** and **IAM policies** following least privilege. | MUST |
| INF-5 | Provision **Amazon EC2** to host the n8n orchestrator in a private subnet. | MUST |
| INF-6 | Provide access to **Amazon Bedrock** for content generation. | MUST |
| INF-7 | Provision **Amazon S3** for generated content (and Terraform remote state). | MUST |
| INF-8 | Provision **AWS Secrets Manager** for GitHub tokens and credentials. | MUST |
| INF-9 | Provision **Amazon CloudWatch** for logs, metrics, and alarms. | MUST |
| INF-10 | Provision **Amazon EventBridge** and **EventBridge Scheduler** for scheduling. | MUST |
| INF-11 | Provision **AWS Lambda** (Go) for scheduled EC2 start/stop. | MUST |
| INF-12 | Terraform state MUST be stored remotely (S3) with locking (DynamoDB). | MUST |
| INF-13 | No resource MAY be created manually outside Terraform (no console drift). | MUST |

---

## 4. Security Requirements

See [Security](./security.md) for full detail.

| ID | Requirement | Priority |
| --- | --- | --- |
| SEC-1 | All IAM roles and policies MUST follow **least privilege**. | MUST |
| SEC-2 | GitHub tokens MUST be stored in **AWS Secrets Manager**. | MUST |
| SEC-3 | There MUST be **no hardcoded credentials** in code, images, or state. | MUST |
| SEC-4 | **Amazon Bedrock** access MUST be scoped by IAM to specific model ARNs. | MUST |
| SEC-5 | **Amazon S3** access MUST be least-privilege; buckets MUST block public access and enforce TLS. | MUST |
| SEC-6 | Data MUST be encrypted at rest (S3, EBS, Secrets Manager) and in transit (TLS 1.2+). | MUST |
| SEC-7 | All access and activity MUST be logged to **CloudWatch**; logs MUST NOT contain secrets. | MUST |
| SEC-8 | Administrative access to EC2 SHOULD use SSM Session Manager (no public SSH). | SHOULD |

---

## 5. AI Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| AI-1 | The system MUST use **Amazon Bedrock** foundation models for content generation. | MUST |
| AI-2 | The model ID/inference profile, `max_tokens`, and `temperature` MUST be configurable (Terraform variables). | MUST |
| AI-3 | The system MUST assemble structured prompts from repository analysis (structure, README, code, config, tech stack, architecture). | MUST |
| AI-4 | Prompts MUST enforce a maximum token budget and truncate deterministically when exceeded. | MUST |
| AI-5 | The system MUST use distinct, platform-tuned prompts for each content type. | MUST |
| AI-6 | The system MUST retry Bedrock throttling/transient errors with exponential backoff. | MUST |
| AI-7 | The system SHOULD record token usage per invocation for cost tracking. | SHOULD |
| AI-8 | Prompt templates SHOULD be configurable without code changes. | SHOULD |

---

## 6. Workflow Requirements

Orchestration is implemented with **n8n** (see [Workflows](./workflows.md)).

| ID | Requirement | Priority |
| --- | --- | --- |
| WF-1 | All stages MUST be orchestrated through n8n workflows. | MUST |
| WF-2 | Workflows MUST pass structured data between stages. | MUST |
| WF-3 | Workflows MUST be triggerable by GitHub events and by manual/scheduled invocation. | MUST |
| WF-4 | Each workflow MUST have explicit success and failure paths. | MUST |
| WF-5 | Workflows MUST be exportable/importable as versioned JSON, free of embedded secret values. | MUST |
| WF-6 | Each content type SHOULD be generatable independently for testing. | SHOULD |

---

## 7. Storage Requirements

See [Architecture → Storage](./architecture.md#7-storage-architecture).

| ID | Requirement | Priority |
| --- | --- | --- |
| ST-1 | Generated content MUST be stored in **Amazon S3**. | MUST |
| ST-2 | Content MUST be organized by repository and date: `generated-content/<repository-name>/<YYYY-MM-DD>/`. | MUST |
| ST-3 | Each package MUST contain the full set of generated assets (articles, social posts, `seo.json`, `metadata.json`, image prompts). | MUST |
| ST-4 | The content bucket MUST enforce encryption at rest and block public access. | MUST |
| ST-5 | The content bucket SHOULD use versioning and lifecycle policies to bound cost. | SHOULD |
| ST-6 | Terraform state MUST be stored in a separate, versioned, encrypted S3 bucket with DynamoDB locking. | MUST |

---

## 8. Monitoring Requirements

See [Monitoring](./monitoring.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| MON-1 | All components MUST emit **structured logs** to CloudWatch. | MUST |
| MON-2 | Key metrics (runs started/succeeded/failed, latency, token usage) MUST be published to CloudWatch. | MUST |
| MON-3 | A run ID MUST correlate logs across all stages of a single run. | MUST |
| MON-4 | Failure alarms (workflow failures, Bedrock failures, infra health) MUST notify operators. | MUST |
| MON-5 | An operational dashboard SHOULD summarize run health at a glance. | SHOULD |
| MON-6 | Log retention MUST be bounded (default 14 days). | MUST |

---

## 9. Cost Optimization Requirements

See [Cost Optimization](./cost-optimization.md).

| ID | Requirement | Priority |
| --- | --- | --- |
| COST-1 | The EC2 host MUST be automatically **started and stopped** to avoid running continuously. | MUST |
| COST-2 | Start/stop MUST be driven by **EventBridge Scheduler** invoking a **Go AWS Lambda**. | MUST |
| COST-3 | The default schedule MUST **start EC2 at 19:00** and **stop EC2 at 21:00**. | MUST |
| COST-4 | The start/stop schedule MUST be configurable via Terraform variables. | MUST |
| COST-5 | S3 lifecycle policies MUST bound stored-content cost. | MUST |
| COST-6 | CloudWatch log retention MUST be bounded. | MUST |
| COST-7 | Bedrock cost SHOULD be controlled via token budgets and capped `max_tokens`. | SHOULD |

---

## 10. Future Enhancements

Planned capabilities (see [Roadmap](./roadmap.md)). These are **future work** and not part of the current compliant release.

**Content breadth**

- Multi-language article generation
- AI-generated architecture diagrams
- AI-generated sequence diagrams
- AI-generated flowcharts
- AI-generated release notes
- Documentation generation
- API documentation generation
- YouTube video scripts
- TikTok scripts
- Shorts scripts
- Podcast summaries

**Intelligence & quality**

- Multi-model AI support
- Content quality scoring
- Duplicate content detection

**Automation**

- Scheduled repository rescans
- Automatic publishing to blogging platforms
- CMS integrations
- WordPress integration
- Ghost CMS integration
- Hashnode publishing API
- Medium publishing API
- Dev.to publishing API
