# Security

Security model and controls for the **GitHub AI Blog Generator**. The platform is designed to be secure by default: least-privilege IAM, HMAC-validated webhooks, a lightweight handler that gates on the commit trigger, restricted security groups, HTTPS-only ingress, secrets kept out of source, SSH key authentication, and encryption in transit and at rest.

Related: [Infrastructure](./infrastructure.md) · [Deployment](./deployment.md) · [Monitoring](./monitoring.md).

---

## 1. Identity & Access Management (IAM)

- Every compute identity — each Lambda and the EC2 instance — has its **own least-privilege role**; no shared, over-broad roles.
- Human/CI access uses **short-lived credentials**: SSO for humans, **GitHub OIDC** federation for CI (no long-lived access keys). See [CI/CD](./ci-cd.md#5-aws-authentication-oidc).
- The webhook handler may only publish events; the instance starter may only start; the idle-shutdown function may only stop; the instance may only consume the queue.

---

## 2. Webhook Signature Validation & Trigger Gate

GitHub Webhooks are the entry point, so the delivery path is hardened end to end ([WH-7…WH-10](./requirements.md#72-signature-validation)). The **Webhook Handler Lambda** validates the signature **before** it evaluates the commit-message trigger, and publishes an event **only** on a match.

| Control | Implementation |
| --- | --- |
| **HMAC SHA-256** | The `X-Hub-Signature-256` header is recomputed over the raw request body using `WEBHOOK_SECRET` |
| **Constant-time comparison** | Signatures are compared with a constant-time function to prevent timing attacks |
| **Reject invalid signatures** | Missing/invalid signatures are rejected with `401` and **never evaluated or published** |
| **Trigger gate** | Only commits matching the publish trigger (default `blog:`) are published to EventBridge; all others return `200` and stop |
| **HTTPS only** | The API Gateway endpoint accepts TLS traffic only |
| **Audit logging** | Every delivery — published, ignored, and rejected — is logged to CloudWatch |

**Validation + gate contract (illustrative):**

```text
signature = "sha256=" + HMAC_SHA256(WEBHOOK_SECRET, raw_request_body)
if not constant_time_equals(signature, header["X-Hub-Signature-256"]):
    respond 401 and log rejection        # nothing is evaluated or published
elif not commit_message.startswith(PUBLISH_TRIGGER):
    respond 200 and log "ignored"        # acknowledged, no processing
else:
    events.put(bus, "blog.publish.requested", payload)
    respond 200
```

**Operational notes:**

- Rotate `WEBHOOK_SECRET` on a schedule and immediately on suspected exposure; update it in the GitHub webhook configuration in the same change window.
- Never log the raw payload or the secret — log the delivery ID, event type, and outcome only.

---

## 3. Least Privilege

Policies specify concrete actions and resource ARNs; wildcards are avoided wherever an ARN can be named.

**Webhook Handler Lambda (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    { "Sid": "PublishMatchedEvents", "Effect": "Allow",
      "Action": ["events:PutEvents"],
      "Resource": "arn:aws:events:us-east-1:<acct>:event-bus/blog-gen-bus" },
    { "Sid": "Logs", "Effect": "Allow",
      "Action": ["logs:CreateLogStream", "logs:PutLogEvents"],
      "Resource": "arn:aws:logs:us-east-1:<acct>:log-group:/aws/lambda/blog-gen-webhook-handler:*" }
  ]
}
```

**Instance Starter Lambda (illustrative):**

```json
{
  "Effect": "Allow",
  "Action": ["ec2:StartInstances", "ec2:DescribeInstances"],
  "Resource": "arn:aws:ec2:us-east-1:<acct>:instance/<instance-id>"
}
```

**Idle Shutdown Lambda (illustrative):**

```json
{
  "Effect": "Allow",
  "Action": ["ec2:StopInstances", "ec2:DescribeInstances"],
  "Resource": "arn:aws:ec2:us-east-1:<acct>:instance/<instance-id>"
}
```

**EC2 instance role (illustrative):**

```json
{
  "Effect": "Allow",
  "Action": ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"],
  "Resource": "arn:aws:sqs:us-east-1:<acct>:blog-gen-events"
}
```

| Principal | May do | May NOT do |
| --- | --- | --- |
| `webhook-handler` (Lambda) | Publish matched events to EventBridge, write its own logs | Start/stop the instance, read the queue, run inference |
| `instance-starter` (Lambda) | Start the instance, write its own logs | Stop the instance, publish events, read the queue |
| `idle-shutdown` (Lambda) | Stop the instance, write its own logs | Start the instance, publish events, read the queue |
| EC2 instance | Consume the queue, write logs | Modify IAM, start/stop itself, alter infrastructure |

---

## 4. Encryption

| Data | At rest | In transit |
| --- | --- | --- |
| EBS (models, n8n state, Repository Memory) | Encrypted EBS | TLS to AWS APIs |
| Amazon SQS | SSE at rest | TLS |
| Webhook endpoint | — | HTTPS only (TLS 1.2+) |
| GitHub API & clone | — | HTTPS only |
| Repository clones | On encrypted instance disk (transient) | — |

Where SSE-KMS is used, keys have rotation enabled and key policies restrict use to the intended roles.

---

## 5. Network Security (Security Groups & HTTPS)

- **Ingress to the instance** is limited to **SSH (22) from the operator CIDR(s)** only. The **n8n (5678)** and **Ollama (11434)** ports are **never** exposed to the internet — reach them through an SSH tunnel.
- **Webhook ingress** terminates at **API Gateway (HTTPS only)**, not the instance; the instance never receives inbound webhook traffic.
- Restrict `OperatorCidr` to known addresses; avoid `0.0.0.0/0`.
- Egress allows the instance to clone repositories, pull container images, and reach AWS APIs.

---

## 6. Environment Variables & Secrets Management

- Sensitive values (`WEBHOOK_SECRET`, `PUBLISH_TRIGGER` config, any repository tokens) are supplied via **environment variables / a secrets store** and injected at deploy or runtime — **never committed** to source, images, or CloudFormation templates ([SEC-5](./requirements.md#9-security-requirements), [SEC-6](./requirements.md#9-security-requirements)).
- `.env` and any local parameter files are git-ignored.
- No long-lived AWS keys in code, CI, or images — CI uses **OIDC**; compute uses **instance/Lambda roles**.
- Exported n8n workflow JSON references credentials **by ID**, never by value.
- **Repository Memory** stores analysis and topic metadata only — never secrets ([MEM-5](./requirements.md#5-repository-memory-requirements)).

---

## 7. SSH Key Authentication

- The EC2 instance uses **key-pair (public-key) authentication**; **password authentication is disabled** (`PasswordAuthentication no`) ([SEC-7](./requirements.md#9-security-requirements)).
- The private key never leaves the operator's machine; store it securely with correct permissions.
- SSH is reachable only from the restricted `OperatorCidr`.
- Rotate the key pair periodically and on suspected exposure.

---

## 8. Logging & Audit Trails

- **AWS CloudTrail** records control-plane API activity (recommended: org-level trail to a dedicated, locked log bucket).
- **CloudWatch Logs** capture handler, starter, idle-shutdown, and n8n logs with bounded retention.
- Structured logs **must not** contain secrets or full source contents — log references (delivery IDs, run IDs), not payloads.

See [Monitoring](./monitoring.md) for alerting on suspicious or failed activity.

---

## 9. Data Handling & Privacy

- Only **repository content the operator has rights to** should be processed. For private repos, access is via a scoped token.
- **All inference is local** (Ollama) — repository content is **never sent to a third-party model provider**, a core privacy property of the design.
- Cloned repositories are transient — they live on the instance's disk during a run and are not persisted.

---

## 10. Reporting a Vulnerability

Please report security issues privately (e.g. GitHub Security Advisories or a maintainer email) rather than opening a public issue. Include reproduction steps and impact. Do not include exploit details in public channels until a fix is released.
