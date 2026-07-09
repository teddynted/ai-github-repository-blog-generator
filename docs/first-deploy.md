# First Deploy — Runbook

A single linear checklist to take the platform from an empty AWS account to a
live, triggered blog generation. Each step links to deeper docs. Deploy is
**opt-in**: nothing provisions until you arm it.

> **Time:** ~30–45 min (most of it AWS provisioning + the first model pull).
> **Deeper reference:** [deployment.md](./deployment.md) · [ci-cd.md](./ci-cd.md) · [security.md](./security.md)

## 0. Prerequisites

- [ ] AWS account + AWS CLI configured (`aws sts get-caller-identity` works).
- [ ] **GPU Spot quota** for the instance family (default `g4dn.xlarge`). Request an increase for *All G and VT Spot Instance Requests* if needed, or set `EnableGpu=false` to smoke-test CPU-only first.
- [ ] An **EC2 key pair** in your region (for SSH): `aws ec2 create-key-pair --key-name blog-gen-key ...`.
- [ ] A **GitHub PAT** for the repo you'll onboard (fine-grained: Contents read, Webhooks read/write).
- [ ] Local tools: `go`, `git`, `gh` (optional), and the repo cloned.

## 1. Bootstrap (OIDC role + artifacts bucket)

```bash
scripts/bootstrap.sh --region us-east-1 --owner <you> --repo ai-github-repository-blog-generator
```

This deploys `blog-gen-bootstrap` (artifacts bucket + GitHub OIDC provider +
least-privilege deploy role, **including the Secrets Manager permissions the
compute stack needs**) and prints the exact GitHub values to set next.

> Already bootstrapped before the SMTP work landed? Re-run this — the deploy role
> gained `secretsmanager` permissions it now needs.

## 2. Set GitHub repository values

**Settings → Secrets and variables → Actions.** Use the values bootstrap printed:

| Kind | Name | Value |
| --- | --- | --- |
| Secret | `AWS_DEPLOY_ROLE_ARN` | `DeployRoleArn` output |
| Variable | `ARTIFACTS_BUCKET` | `ArtifactsBucketName` output |
| Variable | `AWS_REGION` | e.g. `us-east-1` |
| Variable | `KEY_PAIR_NAME` | your EC2 key pair |
| Variable | `OPERATOR_CIDR` | (optional) your SSH source CIDR |
| Variable | `DEPLOY_ENABLED` | `true` ← arms the deploy workflow |

Optional — email notifications ([Turbo SMTP](./local-workflow.md)):

| Kind | Name | Value |
| --- | --- | --- |
| Variable | `NOTIFY_EMAIL_FROM` / `NOTIFY_EMAIL_TO` | from / comma-separated recipients |
| Variable | `SMTP_USERNAME` | Turbo SMTP account |
| Secret | `SMTP_PASSWORD` | (optional) auto-seeds Secrets Manager each deploy |

## 3. Deploy the stacks

Merge to `main` (or **Actions → deploy → Run workflow**). `deploy.yml` builds and
uploads the Lambdas + worker, then deploys **network → serverless → compute →
observability**.

```bash
gh run watch    # or watch it in the Actions tab
```

CPU-only smoke test instead? Deploy compute manually with `EnableGpu=false`
(see [deployment.md §4](./deployment.md)).

## 4. Set the SMTP password (skip if not using email, or if you set the `SMTP_PASSWORD` GitHub secret)

```bash
SMTP_SECRET_ARN=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='NotificationsSecretArn'].OutputValue" --output text)
aws secretsmanager put-secret-value --secret-id "$SMTP_SECRET_ARN" --secret-string 'YOUR_TURBO_SMTP_PASSWORD'
# then: sudo systemctl restart blog-gen-worker   (on the instance, if it was already running)
```

## 5. Register the repository

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

curl -sS -X POST "$REGISTRATION_URL" -H "Content-Type: application/json" \
  -d '{"repository_url":"https://github.com/<you>/<repo>","pat":"github_pat_xxx"}'
```

This validates access, stores the PAT + webhook secret in Secrets Manager, writes
DynamoDB metadata, and **creates the GitHub webhook automatically**. Confirm a
green ✓ under the repo's **Settings → Webhooks → Recent Deliveries**.

## 6. Trigger a generation

Push a commit whose message starts with the trigger (default `blog:`):

```bash
git commit --allow-empty -m "blog: first end-to-end test" && git push
```

A normal commit (no `blog:` prefix) is acknowledged and **ignored** — that's the
opt-in gate working.

## 7. Verify

| Where | Check |
| --- | --- |
| GitHub → Webhooks | Delivery shows ✓ (HTTP 200) |
| CloudWatch (handler) | verify → trigger match → PutEvents |
| EC2 | instance transitions `stopped` → `running` |
| Instance (SSH) | `docker ps` shows `ollama`; `curl -s localhost:11434/api/tags` lists the model; `nvidia-smi` (GPU); `systemctl status blog-gen-worker` active |
| Instance (SSH) | `journalctl -u blog-gen-worker -f` — process → generate → review → publish |
| Output | new Markdown at the destination (S3 `OUTPUT_S3_BUCKET`, else `/data/generated-content`) |
| Email | notification arrives (if configured) |
| EC2 | returns to `stopped` after the idle timeout |

The generated package includes the blog, README/docs suggestions, an
architecture summary, release notes, and the evidence-grounded **AWS
architecture diagrams**.

## 8. First-run gotchas

- **First model pull is slow** (`qwen2.5:7b` ≈ 4.7 GB) — the worker's readiness
  wait + SQS redelivery cover it; the first generation may lag a few minutes.
- **`nvidia-smi` fails / no GPU in container** — the most likely first-launch
  fix; Ollama auto-falls back to CPU so the pipeline still runs. See the Ollama
  notes in [development-plan.md](./development-plan.md).
- **Spot interruption** — the instance stops; the gp3 volume (models, memory)
  persists, and the next matched event starts it again.

## 9. Teardown

Delete the app stacks (reverse order), then bootstrap. The persistent volume is
`Retain` by design — delete it explicitly if you want the models gone.

```bash
for s in observability compute serverless network; do
  aws cloudformation delete-stack --stack-name "blog-gen-$s"
  aws cloudformation wait stack-delete-complete --stack-name "blog-gen-$s"
done
```
