# First Deploy — Runbook

A single linear checklist to take the platform from an empty AWS account to a
live, triggered blog generation. Each step links to deeper docs. Deploy is
**opt-in**: nothing provisions until you arm it.

> **Time:** ~30–45 min (most of it AWS provisioning + the first model pull).
> **Deeper reference:** [deployment.md](./deployment.md) · [ci-cd.md](./ci-cd.md) · [security.md](./security.md)

## 0. Prerequisites

- [ ] AWS account + AWS CLI configured (`aws sts get-caller-identity` works).
- [ ] **GPU Spot quota** for the instance family (default `g4dn.xlarge`). Request an increase for *All G and VT Spot Instance Requests* if needed, or set `EnableGpu=false` to smoke-test CPU-only first.
- [ ] *(Optional)* An **EC2 key pair** — only if you want to reuse an existing one. Otherwise leave `KEY_PAIR_NAME` unset and the compute stack **creates and manages** one for you (private key in SSM).
- [ ] A **GitHub PAT** for the repo you'll onboard (fine-grained: Contents read, Webhooks read/write).
- [ ] Local tools: `go`, `git`, `gh` (optional), and the repo cloned.

## 1. Bootstrap (OIDC role + artifacts bucket)

```bash
scripts/bootstrap.sh --region us-east-1 --owner <you> --repo ai-github-repository-blog-generator
```

This deploys `blog-gen-bootstrap` (artifacts bucket + GitHub OIDC provider +
least-privilege deploy role, **including the Secrets Manager permissions the
compute stack needs**) and prints the exact GitHub values to set next.

An account may have only one OIDC provider for `token.actions.githubusercontent.com`;
the script **auto-detects an existing one and reuses it**, so you don't need
`--existing-oidc-arn` (it stays available as an override).

> **Why this step is manual (not in `deploy.yml`).** `deploy.yml` authenticates
> by assuming the `blog-gen-deploy` role via OIDC — but that role is *created
> here*. CI can't create its own login role (chicken-and-egg), and wiring it in
> would mean storing long-lived **admin** AWS keys in GitHub, which OIDC exists to
> avoid. So bootstrap runs **once per account**, with admin credentials, by you.
> Everything after it is fully automated.

### Run it in AWS CloudShell (no local AWS setup)

The easiest way — CloudShell is a browser terminal in the AWS console that
already has your credentials, with `git` and the AWS CLI preinstalled:

1. AWS Console → click the **CloudShell** icon in the top navigation bar.
2. In the shell:
   ```bash
   git clone https://github.com/teddynted/ai-github-repository-blog-generator.git
   cd ai-github-repository-blog-generator
   bash scripts/bootstrap.sh --region <your-region> \
     --owner teddynted --repo ai-github-repository-blog-generator
   ```
3. Copy the printed values into **GitHub → Settings → Secrets and variables →
   Actions** (see step 2 below), then re-run your workflow.

Prefer your laptop? Same command, provided you have the AWS CLI configured with
credentials that can create IAM roles, an OIDC provider, and an S3 bucket.

> Already bootstrapped before the SMTP work landed? Re-run this — the deploy role
> gained `secretsmanager` permissions it now needs.

## 2. Set GitHub repository values

**Settings → Secrets and variables → Actions.** Use the values bootstrap printed:

| Kind | Name | Value |
| --- | --- | --- |
| Secret | `AWS_DEPLOY_ROLE_ARN` | `DeployRoleArn` output |
| Variable | `AWS_REGION` | e.g. `us-east-1` |
| Variable | `KEY_PAIR_NAME` | *(optional)* existing key pair — **leave unset** and the stack creates one (private key in SSM at `/ec2/keypair/<id>`) |
| Variable | `OPERATOR_CIDR` | (optional) your SSH source CIDR |
| Variable | `DEPLOY_ENABLED` | `true` ← arms the deploy workflow |

`ARTIFACTS_BUCKET` is **not** a variable — `deploy.yml` derives it as
`blog-gen-artifacts-<account>-<region>` (matching bootstrap), so it can't drift.

> **Environment override gotcha:** the deploy job runs in the `production`
> environment. A variable set under **Settings → Environments → production**
> overrides the repository-level one of the same name — check there too if a
> value doesn't seem to take effect.

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

Registration is a **two-part** step: create a GitHub PAT (in GitHub), then POST it
to the registration API Gateway endpoint.

### 5a. Create the GitHub PAT (in GitHub — not via the endpoint)

GitHub → **Settings → Developer settings → Personal access tokens → Fine-grained
tokens → Generate new token**:

- **Repository access:** the repo you want to onboard.
- **Permissions:** **Contents** = Read · **Webhooks** = Read and write · **Metadata** = Read (auto).
- Generate and copy the `github_pat_…` value.

### 5b. Register via the endpoint (requires the API key)

> [!IMPORTANT]
> The registration route has **`ApiKeyRequired: true`** — you **must** send an
> `x-api-key` header, or the call returns **403 Forbidden**. (The webhook route is
> different: it's public and secured by an HMAC signature, no API key.)

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

# The registration API key (retrieve its value from the key id output)
API_KEY_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationApiKeyId'].OutputValue" --output text)
API_KEY=$(aws apigateway get-api-key --api-key "$API_KEY_ID" --include-value \
  --query value --output text)

curl -sS -X POST "$REGISTRATION_URL" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d '{
        "repository_url": "https://github.com/<you>/<repo>",
        "pat": "github_pat_xxx",
        "trigger_pattern": "blog:"
      }'
```

`trigger_pattern` is optional (defaults to `blog:`; supports a literal prefix or
`regex:`). This validates access with the PAT, stores the **PAT + a generated
webhook secret in Secrets Manager** (never plaintext), writes DynamoDB metadata,
and **creates the GitHub webhook automatically** (subscribed to `push` +
`release`). Confirm a green ✓ under the repo's **Settings → Webhooks → Recent
Deliveries**.

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

- **`Could not assume role with OIDC: Not authorized to perform sts:AssumeRoleWithWebIdentity`** —
  the deploy workflow can't assume the role. Almost always one of:
  - **`AWS_DEPLOY_ROLE_ARN` points at the wrong role.** It must be **this**
    project's role, `arn:aws:iam::<account>:role/blog-gen-deploy` — *not* a role
    left over from another project (e.g. an n8n deploy role). A different
    project's role trusts a different repo, so STS refuses.
  - **Bootstrap never completed** in the target account → the role doesn't exist
    (STS reports the same "Not authorized" either way). Run step 1.
  - **Owner/repo mismatch** — bootstrap was run with different `--owner`/`--repo`
    than the repo running the workflow. Re-run with the exact values.

  Diagnose: `aws iam get-role --role-name blog-gen-deploy --query 'Role.AssumeRolePolicyDocument'`
  — the `sub` must be `repo:teddynted/ai-github-repository-blog-generator:*`, and
  the account must match the ARN in the secret.
- **First model pull is slow** (`qwen2.5:7b` ≈ 4.7 GB) — the worker's readiness
  wait + SQS redelivery cover it; the first generation may lag a few minutes.
- **`nvidia-smi` fails / no GPU in container** — the most likely first-launch
  fix; Ollama auto-falls back to CPU so the pipeline still runs. See the Ollama
  notes in [development-plan.md](./development-plan.md).
- **Spot interruption** — the instance stops; the gp3 volume (models, memory)
  persists, and the next matched event starts it again.
- **`InsufficientInstanceCapacity` / "do not have sufficient g4dn.xlarge capacity
  in <az>"** — that AZ is momentarily out of GPU Spot capacity. Set the
  `SUBNET_AZ` variable to an AZ the error lists as available (e.g. `us-east-1b`)
  and redeploy the network + compute stacks. (Changing a subnet's AZ replaces it,
  so delete `blog-gen-compute` and `blog-gen-network` first, then re-run deploy.)
- **`not eligible for Free Tier`** — the account is on the new AWS Free Plan,
  which blocks non-free instance types (the GPU host). Upgrade to a Paid Plan in
  Billing, then redeploy.

## 9. Teardown

Delete the app stacks (reverse order), then bootstrap. The persistent volume is
`Retain` by design — delete it explicitly if you want the models gone.

```bash
for s in observability compute serverless network; do
  aws cloudformation delete-stack --stack-name "blog-gen-$s"
  aws cloudformation wait stack-delete-complete --stack-name "blog-gen-$s"
done
```
