# Security

Security model and controls for the **GitHub AI Blog Generator**. The platform is designed to be secure by default: GitHub PATs isolated in AWS Secrets Manager, least-privilege IAM, HMAC-validated webhooks, restricted security groups, HTTPS-only ingress, SSH key authentication, and encryption in transit and at rest.

Related: [Infrastructure](./infrastructure.md) · [Deployment](./deployment.md) · [Monitoring](./monitoring.md).

---

## 1. Identity & Access Management (IAM)

- Every compute identity — each Lambda and the EC2 instance — has its **own least-privilege role**; no shared, over-broad roles.
- Human/CI access uses **short-lived credentials**: SSO for humans, **GitHub OIDC** federation for CI (no long-lived access keys). See [CI/CD](./ci-cd.md#5-aws-authentication-oidc).
- The registration Lambda may write secrets/metadata; the webhook handler may read metadata and the webhook secret and publish events; the instance consumes the queue and reads the PAT only when cloning.

---

## 2. GitHub PAT & Secret Storage

GitHub Personal Access Tokens are the most sensitive material in the MVP, so they are handled with care ([Requirements §14](./requirements.md#14-secure-credential-storage-requirements)).

| Control | Implementation |
| --- | --- |
| **Secrets Manager only** | PATs and per-repo webhook secrets are stored as secrets in **AWS Secrets Manager** |
| **Never in plain text** | PATs are never stored in source, config files, environment variables, container images, or the metadata database |
| **Reference, not value** | DynamoDB holds only the **secret reference** (ARN/name), never the token |
| **Retrieved only when needed** | The PAT is fetched only for clone/webhook operations — not by the webhook handler on the hot path |
| **Least privilege** | IAM grants `GetSecretValue` scoped to specific secret ARNs; the registration Lambda alone may create/put secrets |
| **Never logged** | PATs (and secrets) are never written to logs or surfaced in errors |
| **Encryption** | Secrets are KMS-encrypted at rest; retrieved over TLS |

**Rotation** of PATs and webhook secrets is a **future enhancement** ([Roadmap](./roadmap.md)); when supported, a webhook-secret rotation must update the GitHub webhook configuration in the same change window to avoid rejected deliveries.

---

## 3. Webhook Signature Validation & Trigger Gate

The **Webhook Handler Lambda** resolves the repository record, validates the signature **before** evaluating the trigger, and publishes an event **only** on a match ([WH-7…WH-10](./requirements.md#72-signature-validation)).

| Control | Implementation |
| --- | --- |
| **Per-repo secret** | The repository's webhook secret is resolved from Secrets Manager (via the metadata record) |
| **HMAC SHA-256** | The `X-Hub-Signature-256` header is recomputed over the raw body |
| **Constant-time comparison** | Signatures are compared with a constant-time function |
| **Reject invalid signatures** | Missing/invalid signatures are rejected with `401` and **never evaluated or published** |
| **Trigger gate** | Only commits matching the repo's trigger pattern (default `blog:`) are published; all others return `200` and stop |
| **HTTPS only** | The API Gateway endpoint accepts TLS traffic only |
| **Audit logging** | Every delivery — published, ignored, and rejected — is logged (never the payload or secret) |

**Validation + gate contract (illustrative):**

```text
record   = metadata.lookup(repo_full_name)
secret   = secretsmanager.get(record.webhook_secret_ref)
expected = "sha256=" + HMAC_SHA256(secret, raw_request_body)
if not constant_time_equals(expected, header["X-Hub-Signature-256"]):
    respond 401 and log rejection
elif not matches(commit_message, record.trigger_pattern):   # default "blog:"
    respond 200 and log "ignored"
else:
    events.put(bus, "blog.publish.requested", payload); respond 200
```

---

## 4. Least Privilege

Policies specify concrete actions and resource ARNs; wildcards are avoided wherever an ARN can be named.

**Registration Lambda (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow",
      "Action": ["secretsmanager:CreateSecret", "secretsmanager:PutSecretValue"],
      "Resource": "arn:aws:secretsmanager:us-east-1:<acct>:secret:blog-gen/repos/*" },
    { "Effect": "Allow",
      "Action": ["dynamodb:PutItem", "dynamodb:UpdateItem"],
      "Resource": "arn:aws:dynamodb:us-east-1:<acct>:table/blog-gen-repositories" }
  ]
}
```

**Webhook Handler Lambda (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["dynamodb:GetItem"],
      "Resource": "arn:aws:dynamodb:us-east-1:<acct>:table/blog-gen-repositories" },
    { "Effect": "Allow", "Action": ["secretsmanager:GetSecretValue"],
      "Resource": "arn:aws:secretsmanager:us-east-1:<acct>:secret:blog-gen/repos/*/webhook-secret*" },
    { "Effect": "Allow", "Action": ["events:PutEvents"],
      "Resource": "arn:aws:events:us-east-1:<acct>:event-bus/blog-gen-bus" }
  ]
}
```

| Principal | May do | May NOT do |
| --- | --- | --- |
| `registration` (Lambda) | Create secrets, write metadata, create webhooks (via PAT) | Read the queue, start/stop the instance, run inference |
| `webhook-handler` (Lambda) | Read metadata, read the webhook secret, publish matched events, **describe** instances (read-only window gate) | **Read the PAT**, **start/stop** the instance, read the queue |
| `scheduled-start` (Lambda) | Start the target instance (by ID) | Stop it, publish events, read the queue |
| `scheduled-stop` (Lambda) | Stop the target instance (by ID) | Start it, publish events |
| EC2 instance | Consume the queue, read the PAT (to clone), write logs | Modify IAM, alter infrastructure, create secrets |

---

## 5. Encryption

| Data | At rest | In transit |
| --- | --- | --- |
| Secrets (PATs, webhook secrets) | KMS-encrypted (Secrets Manager) | TLS |
| DynamoDB (metadata) | Encryption at rest | TLS |
| EBS (models, n8n state, Repository Memory) | Encrypted EBS | TLS to AWS APIs |
| Amazon SQS | SSE at rest | TLS |
| Webhook & registration endpoints | — | HTTPS only (TLS 1.2+) |
| GitHub API & clone | — | HTTPS only |

Where SSE-KMS is used, keys have rotation enabled and key policies restrict use to the intended roles.

---

## 6. Network Security (Security Groups & HTTPS)

- **Ingress to the instance** is limited to **SSH (22) from the operator CIDR(s)** only. The **n8n (5678)** and **Ollama (11434)** ports are **never** exposed to the internet.
- **Webhook and registration ingress** terminate at **API Gateway (HTTPS only)**, not the instance.
- Restrict `OperatorCidr` to known addresses; avoid `0.0.0.0/0`.
- Egress allows the instance to clone repositories, pull container images, and reach AWS APIs.

---

## 7. Environment Variables & Secrets Management

- Sensitive values (webhook secrets, PATs) live in **Secrets Manager** — **never committed** to source, images, or CloudFormation templates ([SEC-5](./requirements.md#9-security-requirements)).
- Non-secret configuration (`PublishTrigger` default, `OLLAMA_MODEL`, timeouts) may be passed as environment/CloudFormation parameters.
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
- **All inference is local** (Ollama) — repository content is **never sent to a third-party model provider**.
- Cloned repositories are transient — they live on the instance's disk during a run and are not persisted.

---

## 11. Reporting a Vulnerability

Please report security issues privately (e.g. GitHub Security Advisories or a maintainer email) rather than opening a public issue. Include reproduction steps and impact. Do not include exploit details in public channels until a fix is released.
