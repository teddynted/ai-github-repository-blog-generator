# Security

Security model and controls for the **GitHub AI Blog Generator**. The platform is designed to be secure by default: GitHub PATs and the Anthropic key isolated in AWS Secrets Manager, least-privilege IAM, API-key-gated endpoints, restricted security groups, HTTPS-only ingress, SSH key authentication, and encryption in transit and at rest.

Related: [Infrastructure](./infrastructure.md) · [Deployment](./deployment.md) · [Monitoring](./monitoring.md).

---

## 1. Identity & Access Management (IAM)

- Every compute identity — each Lambda and the EC2 instance — has its **own least-privilege role**; no shared, over-broad roles.
- Human/CI access uses **short-lived credentials**: SSO for humans, **GitHub OIDC** federation for CI (no long-lived access keys). See [CI/CD](./ci-cd.md#5-aws-authentication-oidc).
- The registration Lambda may write secrets/metadata; the manual-trigger Lambda may only start a Step Functions execution; the state machine may start the host and enqueue jobs; the instance consumes the queue, calls Bedrock, and reads the PAT only when cloning.

---

## 2. GitHub PAT & Secret Storage

GitHub Personal Access Tokens are the most sensitive material in the MVP, so they are handled with care ([Requirements §14](./requirements.md#14-secure-credential-storage-requirements)).

| Control | Implementation |
| --- | --- |
| **Secrets Manager only** | All repositories' PATs live in **one shared AWS Secrets Manager secret** (JSON keyed by owner/name); the **Anthropic API key** is a separate secret (Bedrock uses IAM, no key) |
| **Never in plain text** | PATs and the Anthropic key are never stored in source, config files, environment variables, container images, or the metadata database |
| **Reference, not value** | DynamoDB holds only the **secret reference** (ARN/name), never the token |
| **Retrieved only when needed** | The PAT is fetched only when a run clones the repo — never by the request Lambdas |
| **Least privilege** | IAM grants `GetSecretValue` scoped to specific secret ARNs; the registration Lambda alone may put the repos secret |
| **Never logged** | PATs and the Anthropic key are never written to logs or surfaced in errors |
| **Encryption** | Secrets are KMS-encrypted at rest; retrieved over TLS |

**Rotation** of a repository's PAT is supported: re-`POST /repositories` with `"update": true` and the new value. The Anthropic key is rotated with `put-secret-value` on its secret; restart the worker to pick it up.

---

## 3. Endpoint Authentication (API key)

There is no public webhook ingress. Every API Gateway route — registration,
`POST /process`, and `POST /release-context` — is gated by an **API key**.

| Control | Implementation |
| --- | --- |
| **API key required** | All routes set `ApiKeyRequired: true`; a request without a valid `x-api-key` gets `403` from API Gateway **before** any Lambda runs |
| **Usage plan** | Keys are bound to a shared usage plan on the stage (rate/quota can be tuned) |
| **Least-privilege trigger** | The `manual-trigger` Lambda can only `states:StartExecution` on the orchestration state machine — no EC2, no secrets |
| **HTTPS only** | The API Gateway endpoints accept TLS traffic only |
| **Audit logging** | Every request is logged with its outcome (never the payload or a secret) |

The API key id is a stack output (`RegistrationApiKeyId`); resolve its value with
`aws apigateway get-api-key --include-value`. Rotate by creating a new key on the
usage plan and retiring the old one.

---

## 4. Least Privilege

Policies specify concrete actions and resource ARNs; wildcards are avoided wherever an ARN can be named.

**Registration Lambda (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue"],
      "Resource": "arn:aws:secretsmanager:us-east-1:<acct>:secret:blog-gen/github/repositories-*" },
    { "Effect": "Allow",
      "Action": ["dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:GetItem", "dynamodb:DeleteItem"],
      "Resource": "arn:aws:dynamodb:us-east-1:<acct>:table/blog-gen-repositories" }
  ]
}
```

**manual-trigger Lambda + state machine (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Sid": "ManualTrigger", "Effect": "Allow", "Action": ["states:StartExecution"],
      "Resource": "arn:aws:states:us-east-1:<acct>:stateMachine:blog-gen-orchestration" },
    { "Sid": "StateMachineEc2", "Effect": "Allow", "Action": ["ec2:StartInstances"],
      "Resource": "arn:aws:ec2:us-east-1:<acct>:instance/*",
      "Condition": { "StringEquals": { "aws:ResourceTag/Project": "blog-gen" } } },
    { "Sid": "StateMachineEnqueue", "Effect": "Allow", "Action": ["sqs:SendMessage"],
      "Resource": "arn:aws:sqs:us-east-1:<acct>:blog-gen-events" }
  ]
}
```

| Principal | May do | May NOT do |
| --- | --- | --- |
| `registration` (Lambda) | Read/write the repos secret, write metadata | Read the queue, start/stop the instance, run inference |
| `manual-trigger` (Lambda) | `states:StartExecution` only | Access EC2, secrets, the queue, or inference |
| `StateMachineRole` | Describe + tag-scoped start instances, SSM describe, `sqs:SendMessage` | Stop the instance, read secrets/PAT, run inference |
| `scheduled-start` / `scheduled-stop` (Lambda) | Start/stop the target instance (by ID) | Publish events, read the queue |
| EC2 instance | Consume the queue, call Bedrock, read the PAT (to clone), R/W the content bucket, write logs | Modify IAM, alter infrastructure, create secrets |

---

## 5. Encryption

| Data | At rest | In transit |
| --- | --- | --- |
| Secrets (PATs, Anthropic key, SMTP password) | KMS-encrypted (Secrets Manager) | TLS |
| DynamoDB (metadata) | Encryption at rest | TLS |
| EBS (n8n + PostgreSQL state, Repository Memory) | Encrypted EBS | TLS to AWS APIs |
| Amazon S3 (artifacts) | SSE-KMS at rest | TLS |
| Amazon SQS | SSE at rest | TLS |
| Registration / `/process` endpoints | — | HTTPS only (TLS 1.2+) |
| GitHub API, Bedrock, Anthropic | — | HTTPS only |

Where SSE-KMS is used, keys have rotation enabled and key policies restrict use to the intended roles.

---

## 6. Network Security (Security Groups & HTTPS)

- **Ingress to the instance** is limited to **SSH (22) from the operator CIDR(s)** only. The **n8n (5678)** port is **never** exposed to the internet (SSH tunnel only).
- **Registration and `/process` ingress** terminate at **API Gateway (HTTPS only)**, not the instance.
- Restrict `OperatorCidr` to known addresses; avoid `0.0.0.0/0`.
- Egress allows the instance to clone repositories, pull container images, and reach AWS APIs, Bedrock, and the Anthropic API.

---

## 7. Environment Variables & Secrets Management

- Sensitive values (PATs, the Anthropic API key, the SMTP password) live in **Secrets Manager** — **never committed** to source, images, or CloudFormation templates ([SEC-5](./requirements.md#9-security-requirements)). Bedrock uses IAM, so there is no key to store for the primary leg.
- Non-secret configuration (`PublishTrigger` default, `BEDROCK_MODEL_ID`, `ANTHROPIC_MODEL`, timeouts) may be passed as environment/CloudFormation parameters.
- `.env` and any local parameter files are git-ignored.
- No long-lived AWS keys in code, CI, or images — CI uses **OIDC**; compute uses **instance/Lambda roles**.
- **Repository Memory** stores analysis and topic metadata only — never secrets ([MEM-5](./requirements.md#5-repository-memory-requirements)).

---

## 8. SSH Key Authentication

- The EC2 instance uses **key-pair (public-key) authentication**; **password authentication is disabled** (`PasswordAuthentication no`) ([SEC-7](./requirements.md#9-security-requirements)).
- The private key never leaves the operator's machine; store it securely with correct permissions.
- SSH is reachable only from the restricted `OperatorCidr`.
- Rotate the key pair periodically and on suspected exposure.

---

## 9. Logging & Audit Trails

- **AWS CloudTrail** records control-plane API activity (recommended: org-level trail to a dedicated, locked log bucket).
- **CloudWatch Logs** capture registration, handler, scheduled-start, scheduled-stop, and n8n logs with bounded retention.
- Structured logs **must not** contain PATs, secrets, or full source contents — log references (delivery IDs, run IDs), not payloads.

See [Monitoring](./monitoring.md) for alerting on suspicious or failed activity.

---

## 10. Data Handling & Privacy

- Only **repository content the operator has rights to** should be processed; access is via the scoped PAT.
- **Inference is managed Claude** — repository-derived context is sent to **Amazon Bedrock** (in your AWS account, IAM-authenticated) and, on fallback, the **Anthropic API**, under those services' data-use terms. No other third-party model provider is used.
- Cloned repositories are transient — they live on the instance's disk during a run and are not persisted.

---

## 11. Generated Content Safety & Prompt Injection

The content pipeline feeds **untrusted repository text** — release notes, commit
messages, README, and documentation — into LLM prompts. A crafted commit message
or release body could attempt a **prompt-injection** ("ignore previous
instructions and …") to steer a generated artifact.

**Mitigations in place today:**

- **Grounding, not free generation.** Every generator is instructed to ground its
  output strictly in the Release Context and *not invent* facts, versions,
  services, or features; deterministic fallbacks produce output with no model at
  all. This bounds what injected text can achieve — the generators are built to
  refuse fabrication (and, per the QA edge-case work, to skip rather than invent
  when there is nothing groundable).
- **Human review + approval gate.** No artifact is published without passing the
  Review & Approval Workflow ([governance](./governance.md)); the publishing layer
  **refuses content that is not approved**. A successful injection therefore
  cannot auto-publish — a human sees the output first.
- **Managed inference, in-account first.** Generation runs on Amazon Bedrock
  (your account, IAM) with an Anthropic API fallback — not an arbitrary
  third-party endpoint — so injected text has a bounded, audited egress path.
- **Authenticated trigger limits initiation.** Runs fire only on an authenticated
  `POST /process` (API key), so an outside party cannot freely trigger generation
  on arbitrary input.

**Residual risk & recommended hardening (fast-follow, not yet implemented):**

- Treat repository text as **data, not instructions**: wrap untrusted content in
  explicit delimiters in each prompt and instruct the model to ignore any
  instructions found inside those delimiters.
- Keep the **human-approval gate mandatory** for any public destination — do not
  add a fully-unattended publish path for externally-sourced content.
- Consider a lightweight content check (denylist for injected directives, links to
  unexpected domains) before the review stage for defense in depth.

> The current posture (grounding + mandatory review/approval + managed in-account
> inference) keeps injection from producing an auto-published outcome.
> The hardening above reduces the chance of injected text degrading an artifact's
> *quality* before a human catches it.

---

## 12. Reporting a Vulnerability

Please report security issues privately (e.g. GitHub Security Advisories or a maintainer email) rather than opening a public issue. Include reproduction steps and impact. Do not include exploit details in public channels until a fix is released.
