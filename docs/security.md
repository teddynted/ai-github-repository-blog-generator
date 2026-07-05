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
| `blog-generator/github-token` | `repo-cloner`, n8n | `secretsmanager:GetSecretValue` on that ARN |
| `blog-generator/n8n-credentials` | EC2 (n8n) | `GetSecretValue` on that ARN |
| `blog-generator/notification-webhook` | `blog-publisher`, n8n | `GetSecretValue` on that ARN |

**Rotation:** enable Secrets Manager rotation where supported; rotate the GitHub token on a schedule and immediately on suspected exposure. The n8n encryption key must remain stable (rotating it invalidates stored credentials) — treat its change as a planned migration.

---

## 3. Least Privilege

Policies specify concrete actions and resource ARNs; wildcards are avoided wherever an ARN can be named.

**Example — `blog-publisher` policy (illustrative):**

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "WritePosts",
      "Effect": "Allow",
      "Action": ["s3:PutObject"],
      "Resource": "arn:aws:s3:::blog-generator-posts/posts/*"
    },
    {
      "Sid": "Notify",
      "Effect": "Allow",
      "Action": ["sns:Publish"],
      "Resource": "arn:aws:sns:us-east-1:<acct>:blog-generator-notifications"
    },
    {
      "Sid": "Logs",
      "Effect": "Allow",
      "Action": ["logs:CreateLogStream", "logs:PutLogEvents"],
      "Resource": "arn:aws:logs:us-east-1:<acct>:log-group:/aws/lambda/blog-generator-blog-publisher:*"
    }
  ]
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
| `repo-cloner` | Read github-token secret, write artifacts | Read posts, invoke Bedrock |
| `repo-analyzer` | Read/write artifacts | Write posts, read secrets |
| `blog-publisher` | Write posts, publish SNS | Read secrets, clone repos |
| n8n (EC2) | Invoke Lambdas + Bedrock, read secrets | Delete buckets, modify IAM |

---

## 4. Encryption

| Data | At rest | In transit |
| --- | --- | --- |
| S3 objects (artifacts, posts, state) | SSE (SSE-S3/SSE-KMS) | TLS enforced via bucket policy (`aws:SecureTransport`) |
| Secrets | KMS-encrypted | TLS |
| EC2 storage | Encrypted EBS | TLS to AWS APIs |
| Bedrock calls | — | TLS |
| GitHub calls | — | HTTPS only |

Where SSE-KMS is used, keys have rotation enabled and key policies restrict use to the intended roles.

---

## 5. HTTPS / Transport Security

- All AWS API traffic and all GitHub interactions use **TLS 1.2+**.
- S3 bucket policies **deny non-TLS requests**.
- The n8n editor is **not exposed publicly**; it is reached via SSM port-forwarding over an encrypted channel ([Deployment §7](./deployment.md#7-deployment-verification)). If ever exposed, it must sit behind TLS termination (ALB/ACM) with authentication.

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
- **S3 access logging** and **versioning** on the posts bucket provide tamper-evidence and recovery.
- Structured logs **must not** contain secrets or full source contents — log references (keys, IDs), not payloads.

See [Monitoring](./monitoring.md) for alerting on suspicious or failed activity.

---

## 8. Data Handling & Privacy

- Only **repository content the operator has rights to** should be processed. For private repos, access is via a scoped GitHub token.
- Cloned artifacts are transient and **expire quickly** (default 7 days) via S3 lifecycle.
- Generated posts contain summaries/excerpts of source repos; treat the posts bucket according to the sensitivity of the analyzed repositories.

---

## 9. Reporting a Vulnerability

Please report security issues privately (e.g. GitHub Security Advisories or a maintainer email) rather than opening a public issue. Include reproduction steps and impact. Do not include exploit details in public channels until a fix is released.
