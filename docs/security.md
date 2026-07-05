# Security

Security model and controls for the **AI GitHub Repository Blog Generator**. The platform is designed to be secure by default: least-privilege IAM, secrets in AWS Secrets Manager, encryption in transit and at rest, no public ingress, and auditable access.

Related: [Infrastructure](./infrastructure.md) · [Deployment](./deployment.md) · [Monitoring](./monitoring.md).

---

## 1. Identity & Access Management (IAM)

- Every compute identity (each Lambda, the EC2 instance) has its **own role** — no shared, over-broad roles.
- Human/CI access uses **short-lived credentials**: SSO for humans, **GitHub OIDC** federation for CI (no long-lived access keys). See [CI/CD](./ci-cd.md#5-aws-authentication-oidc).
- Administrative access to the n8n host is via **SSM Session Manager** — there is **no SSH key and no inbound SSH**.

---

## 2. Secrets Management

- All secrets live in **AWS Secrets Manager**; nothing sensitive is committed to git or baked into images/AMIs.
- Terraform creates the secret *resources*; **values are populated out of band** ([Deployment §4](./deployment.md#4-secrets)).
- Access is granted per-consumer, scoped to specific secret ARNs.

| Secret | Consumer | Access |
| --- | --- | --- |
| `blog-generator/repos/<owner>/<name>/pat` | n8n (EC2) | `secretsmanager:GetSecretValue` on that ARN (per repo) |
| `blog-generator/repos/<owner>/<name>/webhook-secret` | n8n (EC2) | `GetSecretValue` on that ARN (per repo) |
| `blog-generator/n8n-credentials` | n8n (EC2) | `GetSecretValue` on that ARN |
| `blog-generator/notification-webhook` | n8n (EC2) | `GetSecretValue` on that ARN |

Per-repository PATs and webhook secrets are created **at registration** under the `blog-generator/repos/*` prefix; n8n is granted `secretsmanager:CreateSecret`/`PutSecretValue` scoped to that prefix and `GetSecretValue` to read them. **The PAT is stored only here — never in DynamoDB, code, or plaintext** ([SEC-4](./requirements.md#7-security-requirements)); DynamoDB holds only the secret ARN.

**Rotation:** rotate PATs and webhook secrets on a schedule and immediately on suspected exposure. When rotating a webhook secret, update it in the GitHub webhook configuration in the same change window to avoid rejected deliveries. The n8n encryption key must remain stable (rotating it invalidates stored credentials) — treat its change as a planned migration.

---

## 3. Least Privilege

Policies specify concrete actions and resource ARNs; wildcards are avoided wherever an ARN can be named.

**Example — `ec2-scheduler` Lambda policy (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "StartStopN8nHost",
      "Effect": "Allow",
      "Action": ["ec2:StartInstances", "ec2:StopInstances"],
      "Resource": "arn:aws:ec2:us-east-1:<acct>:instance/<n8n-instance-id>"
    },
    {
      "Sid": "Logs",
      "Effect": "Allow",
      "Action": ["logs:CreateLogStream", "logs:PutLogEvents"],
      "Resource": "arn:aws:logs:us-east-1:<acct>:log-group:/aws/lambda/blog-generator-ec2-scheduler:*"
    }
  ]
}
```

**Example — n8n instance role: write the content package (illustrative):**

```json
{
  "Effect": "Allow",
  "Action": ["s3:PutObject", "s3:GetObject"],
  "Resource": "arn:aws:s3:::blog-generator-generated-content/generated-content/*"
}
```

**Bedrock access** is scoped to the specific model ARN(s):

```json
{
  "Effect": "Allow",
  "Action": ["bedrock:InvokeModel"],
  "Resource": "arn:aws:bedrock:us-east-1::foundation-model/<model-id>"
}
```

| Principal | May do | May NOT do |
| --- | --- | --- |
| n8n (EC2) | Read secrets, invoke Bedrock, write generated-content, publish SNS | Delete buckets, modify IAM, start/stop EC2 |
| `ec2-scheduler` (Lambda) | Start/stop the n8n instance, write its own logs | Read secrets, access content, invoke Bedrock |

---

## 4. Encryption

| Data | At rest | In transit |
| --- | --- | --- |
| S3 objects (generated content, state) | SSE (SSE-S3/SSE-KMS) | TLS enforced via bucket policy (`aws:SecureTransport`) |
| DynamoDB (`repositories`, `tf-locks`) | Encryption at rest | TLS |
| Secrets (PATs, webhook secrets) | KMS-encrypted | TLS |
| EC2 storage | Encrypted EBS | TLS to AWS APIs |
| Bedrock calls | — | TLS |
| GitHub API & clone | — | HTTPS only |
| Registration & webhook endpoints | — | HTTPS only |

Where SSE-KMS is used, keys have rotation enabled and key policies restrict use to the intended roles.

---

## 5. HTTPS / Transport Security

- All AWS API traffic, all GitHub interactions, and the **inbound webhook endpoint** use **TLS 1.2+** — the endpoint accepts **HTTPS only**.
- S3 bucket policies **deny non-TLS requests**.
- Only the **webhook path** is publicly reachable (443, restricted by security group to GitHub's IP ranges); the **n8n management UI is not exposed publicly** and is reached via SSM port-forwarding over an encrypted channel ([Deployment §7](./deployment.md#7-deployment-verification)). A managed TLS front door (ALB + ACM, or API Gateway) is a documented hardening option.

---

## 6. Credential Management

- No long-lived AWS keys in code, CI, or images — CI uses **OIDC**; compute uses **instance/Lambda roles**.
- Local development uses a **low-privilege dev profile**, never production credentials ([Local Development §3](./local-development.md#3-aws-cli-configuration)).
- Exported n8n workflow JSON references credentials **by ID**, never by value.
- `.env`, `*.tfvars`, and state files are git-ignored.

---

## 7. Logging & Audit Trails

- **AWS CloudTrail** records control-plane API activity (recommended: org-level trail to a dedicated, locked log bucket).
- **CloudWatch Logs** capture application/workflow logs with bounded retention.
- **S3 access logging** and **versioning** on the generated-content bucket provide tamper-evidence and recovery.
- Structured logs **must not** contain secrets or full source contents — log references (keys, IDs), not payloads.

See [Monitoring](./monitoring.md) for alerting on suspicious or failed activity.

---

## 8. Data Handling & Privacy

- Only **repository content the operator has rights to** should be processed. For private repos, access is via a scoped GitHub token.
- Cloned repositories are transient — they live on the EC2 host's ephemeral disk during a run and are not persisted to S3.
- Generated content contains summaries/excerpts of source repos; treat the generated-content bucket according to the sensitivity of the analyzed repositories.

---

## 9. Reporting a Vulnerability

Please report security issues privately (e.g. GitHub Security Advisories or a maintainer email) rather than opening a public issue. Include reproduction steps and impact. Do not include exploit details in public channels until a fix is released.

---

## 10. Webhook Security

GitHub Webhooks are the primary entry point, so the delivery path is hardened end to end ([WH-7…WH-10](./requirements.md#42-signature-validation)).

| Control | Implementation |
| --- | --- |
| **Secret validation** | Every delivery is validated against the repository's `.../webhook-secret` (resolved via its DynamoDB record) from Secrets Manager |
| **HMAC SHA-256** | The `X-Hub-Signature-256` header is recomputed over the raw body and compared |
| **Constant-time comparison** | Signature comparison uses a constant-time function to prevent timing attacks |
| **Reject invalid signatures** | Missing/invalid signatures are rejected with `401` and **not processed** |
| **HTTPS only** | The endpoint accepts TLS traffic only; plaintext is refused |
| **Network restriction** | Inbound 443 is limited to GitHub's published webhook IP ranges |
| **Least privilege** | n8n may only read the webhook secret; no other component can |
| **Audit logging** | Every delivery — accepted **and** rejected — is logged to CloudWatch |

**Validation contract (illustrative):**

```text
signature = "sha256=" + HMAC_SHA256(secret, raw_request_body)
if not constant_time_equals(signature, header["X-Hub-Signature-256"]):
    respond 401 and log rejection
else:
    process event
```

**Operational notes:**

- Each repository has its **own** webhook signing secret, generated at registration; a leak is contained to one repository.
- Rotate the webhook secret in Secrets Manager and GitHub together to avoid rejected deliveries ([§2](#2-secrets-management)).
- GitHub retries failed deliveries; inspect **Settings → Webhooks → Recent Deliveries** to replay or debug.
- Never log the raw payload, the PAT, or the secret — log the delivery ID, event type, and outcome only.

### Registration endpoint

The registration endpoint receives a PAT, so it is held to the same bar: **HTTPS only**, access-controlled (admin token / basic auth), and it immediately writes the PAT to Secrets Manager without logging it. See [Repository Registration Requirements](./requirements.md#2-repository-registration-requirements).
