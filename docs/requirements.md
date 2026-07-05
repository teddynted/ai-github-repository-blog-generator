# Requirements

This document defines the functional and non-functional requirements for the **AI GitHub Repository Blog Generator**.

Requirements use the following convention:

- **MUST** — mandatory for a compliant release.
- **SHOULD** — strongly recommended; may be deferred with justification.
- **MAY** — optional or future work.

Each requirement has a stable ID (`FR-*` for functional, `NFR-*` for non-functional) so it can be referenced from issues, pull requests, and tests.

Related: [Architecture](./architecture.md) · [Workflows](./workflows.md) · [Roadmap](./roadmap.md).

---

## 1. Functional Requirements

### 1.1 GitHub Repository Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-1.1 | The system MUST accept a public GitHub repository URL as input to a generation run. | MUST |
| FR-1.2 | The system MUST support authenticated access to private repositories via a GitHub token stored in Secrets Manager. | SHOULD |
| FR-1.3 | The system MUST validate the repository URL and reject malformed or unreachable inputs with a clear error. | MUST |

### 1.2 Repository Cloning

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-2.1 | The system MUST clone the target repository into ephemeral storage (`repo-cloner` Lambda). | MUST |
| FR-2.2 | The system SHOULD perform a shallow clone (`--depth 1`) to minimize time and storage. | SHOULD |
| FR-2.3 | The system MUST upload a normalized snapshot of the cloned repository to the artifacts S3 bucket. | MUST |
| FR-2.4 | The system MUST clean up ephemeral clone data after upload. | MUST |

### 1.3 Repository Metadata Extraction

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-3.1 | The system MUST extract repository metadata: name, description, default branch, topics, license, stars, and last-updated timestamp. | MUST |
| FR-3.2 | The system SHOULD retrieve metadata from the GitHub REST API where available and fall back to on-disk inspection. | SHOULD |

### 1.4 File Discovery

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-4.1 | The system MUST enumerate the repository file tree. | MUST |
| FR-4.2 | The system MUST rank files by relevance (README, manifests, entrypoints, configuration, docs) for inclusion in analysis. | MUST |
| FR-4.3 | The system MUST respect ignore rules (`.gitignore`, binary files, vendored/`node_modules` directories, size caps). | MUST |

### 1.5 README Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-5.1 | The system MUST locate and parse the repository README (any common casing/extension). | MUST |
| FR-5.2 | The system MUST extract the project summary, usage, and stated purpose from the README. | MUST |

### 1.6 Source Code Analysis

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-6.1 | The system MUST sample representative source files (entrypoints, core modules) within a configurable token budget. | MUST |
| FR-6.2 | The system SHOULD summarize directory structure and module responsibilities. | SHOULD |

### 1.7 Technology Detection

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-7.1 | The system MUST detect programming languages from file extensions and content. | MUST |
| FR-7.2 | The system MUST detect frameworks, build tools, and package managers from manifest files (e.g. `package.json`, `go.mod`, `requirements.txt`, `Dockerfile`, `*.tf`). | MUST |
| FR-7.3 | The system SHOULD detect infrastructure and deployment tooling (Terraform, Docker, CI configs). | SHOULD |

### 1.8 AI Prompt Generation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-8.1 | The system MUST assemble a structured prompt from metadata, README analysis, code samples, and detected technologies. | MUST |
| FR-8.2 | The system MUST enforce a maximum prompt token budget and truncate deterministically when exceeded. | MUST |
| FR-8.3 | The system SHOULD support configurable prompt templates. | SHOULD |

### 1.9 Amazon Bedrock Integration

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-9.1 | The system MUST invoke a configurable Amazon Bedrock foundation model to generate blog content. | MUST |
| FR-9.2 | The system MUST make the model ID, temperature, and max tokens configurable via Terraform variables. | MUST |
| FR-9.3 | The system MUST implement retry with exponential backoff on throttling and transient errors. | MUST |
| FR-9.4 | The system SHOULD record token usage per invocation for cost tracking. | SHOULD |

### 1.10 Blog Generation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-10.1 | The system MUST generate a coherent technical blog post covering purpose, architecture, technologies, and implementation. | MUST |
| FR-10.2 | The system SHOULD include a title, summary/abstract, and section headings. | SHOULD |

### 1.11 Markdown Generation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-11.1 | The system MUST output the blog post as valid Markdown. | MUST |
| FR-11.2 | The system MUST include YAML front matter (title, date, source repo, tags, model). | MUST |
| FR-11.3 | The system SHOULD produce fenced code blocks and, where useful, Mermaid diagrams. | SHOULD |

### 1.12 Blog Versioning

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-12.1 | The system MUST version every generated post (S3 object versioning and/or timestamped keys). | MUST |
| FR-12.2 | The system MUST retain prior versions and allow retrieval of any historical version. | MUST |

### 1.13 Content Storage

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-13.1 | The system MUST store generated posts in a dedicated, encrypted S3 bucket. | MUST |
| FR-13.2 | The system MUST use a deterministic key scheme: `posts/<repo-owner>/<repo-name>/<timestamp>.md`. | MUST |

### 1.14 Scheduling

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-14.1 | The system MUST support scheduled generation runs via Amazon EventBridge. | MUST |
| FR-14.2 | The system MUST support on-demand (manual) triggering. | MUST |

### 1.15 Workflow Automation

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-15.1 | The system MUST orchestrate all stages through n8n workflows. | MUST |
| FR-15.2 | The system MUST pass structured data between workflow stages. | MUST |
| FR-15.3 | The system SHOULD make workflows importable/exportable as versioned JSON. | SHOULD |

### 1.16 Error Handling

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-16.1 | Every workflow stage MUST handle failures explicitly and surface actionable errors. | MUST |
| FR-16.2 | The system MUST retry transient failures and stop after a configurable maximum. | MUST |
| FR-16.3 | Failed runs MUST NOT overwrite or corrupt previously generated content. | MUST |

### 1.17 Notifications

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-17.1 | The system MUST send a notification on run success and on run failure. | MUST |
| FR-17.2 | The system SHOULD support pluggable channels (email/SNS, Slack, webhook). | SHOULD |

---

## 2. Non-Functional Requirements

### 2.1 Scalability

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-1.1 | Compute-heavy stages MUST run in stateless Lambda functions that scale horizontally. | MUST |
| NFR-1.2 | The system SHOULD process multiple repositories concurrently without shared-state contention. | SHOULD |

### 2.2 Availability

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-2.1 | Storage and AI stages MUST rely on managed, highly-available AWS services (S3, Bedrock, Lambda). | MUST |
| NFR-2.2 | The n8n host SHOULD be recoverable from Infrastructure as Code within minutes. | SHOULD |

### 2.3 Reliability

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-3.1 | The system MUST be idempotent per run: re-running a failed job MUST NOT produce corrupt output. | MUST |
| NFR-3.2 | The system MUST use retries with backoff on all external calls (GitHub, Bedrock, S3). | MUST |

### 2.4 Performance

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-4.1 | A single blog generation run SHOULD complete within 5 minutes for a typical repository. | SHOULD |
| NFR-4.2 | Analysis MUST enforce token and file-size budgets to bound latency and cost. | MUST |

### 2.5 Security

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-5.1 | All IAM roles MUST follow least privilege. | MUST |
| NFR-5.2 | All secrets MUST be stored in AWS Secrets Manager — never in code or plaintext. | MUST |
| NFR-5.3 | Data MUST be encrypted at rest (S3, EBS) and in transit (TLS). | MUST |

See [Security](./security.md) for full detail.

### 2.6 Cost Optimization

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-6.1 | The EC2 n8n host SHOULD be stopped outside operating windows via scheduled EventBridge rules. | SHOULD |
| NFR-6.2 | S3 lifecycle policies MUST transition/expire artifacts to control storage cost. | MUST |
| NFR-6.3 | CloudWatch log retention MUST be bounded (default 14 days). | MUST |

See [Cost Optimization](./cost-optimization.md).

### 2.7 Maintainability

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-7.1 | All infrastructure MUST be defined as code (Terraform) with no manual console drift. | MUST |
| NFR-7.2 | Go code MUST pass `gofmt`, `go vet`, and unit tests in CI. | MUST |

### 2.8 Extensibility

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-8.1 | Prompt templates, models, and output templates MUST be configurable without code changes where practical. | MUST |
| NFR-8.2 | New workflow stages SHOULD be addable without rewriting existing ones. | SHOULD |

### 2.9 Observability

| ID | Requirement | Priority |
| --- | --- | --- |
| NFR-9.1 | All components MUST emit structured logs to CloudWatch. | MUST |
| NFR-9.2 | Key metrics (runs, successes, failures, latency, token usage) MUST be published to CloudWatch. | MUST |
| NFR-9.3 | Failure alarms MUST notify operators. | MUST |

See [Monitoring](./monitoring.md).
