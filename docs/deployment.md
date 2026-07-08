# Deployment

This guide covers deploying the **GitHub AI Blog Generator** to AWS using **CloudFormation**, then wiring up the GitHub webhook.

Related: [Local Development](./local-development.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [CI/CD](./ci-cd.md).

---

## 1. AWS Prerequisites

| Requirement | Notes |
| --- | --- |
| AWS account | With permissions to create VPC, EC2, Lambda, API Gateway, SQS, EventBridge, IAM, and CloudWatch |
| Region | Any Region where these services are available (e.g. `us-east-1`) |
| AWS CLI v2 | Configured with credentials (`aws configure` or SSO) |
| EC2 key pair | For SSH access to the instance |
| `cfn-lint` | Template linting ([Install](https://github.com/aws-cloudformation/cfn-lint)) |
| Go ≥ 1.25 | To build the Lambda binaries (build toolchain pinned in `go.mod`) |
| Docker | For local build/testing (optional) |

> **No Amazon Bedrock, OpenAI, or Anthropic access is required.** All inference runs locally via Ollama on the instance.
>
> The instance runs Ollama, so choose a **GPU instance type** (e.g. `g4dn.xlarge`) and a Region/AZ with Spot capacity for it.

Verify access:

```bash
aws sts get-caller-identity
aws ec2 describe-key-pairs --key-names "$KEY_PAIR_NAME"
```

---

## First-Time Bootstrap (automated deploy)

Two ways to deploy: **manually** (Sections 3–4 below) or via the **`deploy.yml` GitHub Actions workflow** (recommended for repeatable deploys). Both need a one-time bootstrap that creates the **Lambda artifacts bucket**, and the workflow additionally needs a **GitHub OIDC deploy role**.

Run once, with admin credentials:

```bash
scripts/bootstrap.sh --region us-east-1 --owner <you> --repo <repo>
# If the account already has a GitHub OIDC provider:
#   scripts/bootstrap.sh --no-oidc-provider --existing-oidc-arn <arn>
```

This deploys [`infrastructure/bootstrap.yaml`](../infrastructure/bootstrap.yaml) (artifacts bucket, OIDC provider, deploy role) and prints the values to set on the repository (**Settings → Secrets and variables → Actions**):

| Kind | Name | Value |
| --- | --- | --- |
| Secret | `AWS_DEPLOY_ROLE_ARN` | `DeployRoleArn` output |
| Variable | `ARTIFACTS_BUCKET` | `ArtifactsBucketName` output |
| Variable | `AWS_REGION` | your Region |
| Variable | `KEY_PAIR_NAME` | your EC2 key pair |
| Variable | `OPERATOR_CIDR` | your SSH CIDR (optional) |
| Variable | `DEPLOY_ENABLED` | `true` |

With those set, merging to `main` runs [`deploy.yml`](./ci-cd.md#enabling-deployyml), which packages the Lambdas and deploys **network → serverless → compute → observability**. For a manual deploy instead, set `ARTIFACTS_BUCKET` locally and follow Sections 3–4.

---

## 2. Configuration

Runtime configuration is provided through **CloudFormation parameters** and **environment variables**. Copy the example and edit:

```bash
cp .env.example .env
```

| Parameter / Variable | Description | Example |
| --- | --- | --- |
| `AWS_REGION` | Deployment region | `us-east-1` |
| `ProjectName` | Resource name prefix | `blog-gen` |
| `WEBHOOK_SECRET` | Shared secret for GitHub HMAC validation | random 32+ chars |
| `PUBLISH_TRIGGER` | Commit-message trigger that gates generation | `blog:` |
| `INSTANCE_TYPE` | EC2 instance type for the Spot request | `g4dn.xlarge` |
| `SPOT_MAX_PRICE` | Maximum Spot price you will pay | `0.20` |
| `IDLE_TIMEOUT_MINUTES` | Minutes of inactivity before auto-shutdown | `15` |
| `OLLAMA_MODEL` | Local model to run | `qwen2.5:7b` |
| `EBS_VOLUME_SIZE_GB` | Size of the persistent gp3 volume | `100` |
| `KEY_PAIR_NAME` | EC2 key pair for SSH | `blog-generator-key` |
| `REQUIRE_HUMAN_APPROVAL` | Require manual approval before publishing | `false` |
| `LogRetentionDays` | CloudWatch retention | `14` |
| `OperatorCidr` | CIDR allowed to SSH to the instance | `203.0.113.10/32` |

> **Never commit secrets.** `WEBHOOK_SECRET` and any tokens are provided via environment/parameter input at deploy time, not stored in the repository ([Security](./security.md)).

---

## 3. Build the Lambda Functions

```bash
# Single Go module — build every implemented function to dist/<fn>/bootstrap
make build

# Package each for Lambda (provided.al2023, arm64)
for fn in registration webhook-handler instance-starter idle-shutdown; do
  [ -f "dist/$fn/bootstrap" ] && ( cd dist/$fn && zip $fn.zip bootstrap )
done
```

- `registration` — validate repo + PAT, create the webhook, store metadata + secrets.
- `webhook-handler` — resolve metadata, verify signature, validate the commit-message trigger, publish matched events.
- `instance-starter` — start the Spot Instance on a matched event.
- `idle-shutdown` — stop the Spot Instance after the idle timeout.

Upload the ZIPs to the artifacts bucket (from the [bootstrap](#first-time-bootstrap-automated-deploy)); the serverless template's default code keys are `<fn>.zip` at the bucket root:

```bash
for fn in registration webhook-handler instance-starter idle-shutdown; do
  aws s3 cp "dist/$fn/$fn.zip" "s3://$ARTIFACTS_BUCKET/$fn.zip"
done
```

---

## 4. Deploy the Stacks

Deploy in order: **network → serverless → compute → observability**. Templates are modular and reusable.

```bash
# 1. Network layer
aws cloudformation deploy \
  --template-file infrastructure/network.yaml \
  --stack-name blog-gen-network \
  --parameter-overrides OperatorCidr=$OperatorCidr \
  --capabilities CAPABILITY_NAMED_IAM

# 2. Serverless layer (API Gateway, Lambdas, EventBridge, SQS + DLQ)
aws cloudformation deploy \
  --template-file infrastructure/serverless.yaml \
  --stack-name blog-gen-serverless \
  --parameter-overrides ArtifactsBucket=$ARTIFACTS_BUCKET PublishTrigger=$PUBLISH_TRIGGER \
  --capabilities CAPABILITY_NAMED_IAM

# 3. Compute layer (EC2 Spot + persistent EBS)
aws cloudformation deploy \
  --template-file infrastructure/compute.yaml \
  --stack-name blog-gen-compute \
  --parameter-overrides \
      InstanceType=$INSTANCE_TYPE \
      SpotMaxPrice=$SPOT_MAX_PRICE \
      KeyPairName=$KEY_PAIR_NAME \
      OllamaModel=$OLLAMA_MODEL \
      EbsVolumeSizeGb=$EBS_VOLUME_SIZE_GB \
      IdleTimeoutMinutes=$IDLE_TIMEOUT_MINUTES \
  --capabilities CAPABILITY_NAMED_IAM

# 4. Observability layer
aws cloudformation deploy \
  --template-file infrastructure/observability.yaml \
  --stack-name blog-gen-observability \
  --capabilities CAPABILITY_NAMED_IAM
```

Retrieve the webhook URL:

```bash
aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='WebhookUrl'].OutputValue" --output text
```

---

## 5. Import the n8n Workflows

The instance is stopped by default; it starts on the first webhook. To import workflows, start it once and open n8n through an SSH tunnel (the UI is not publicly exposed):

```bash
INSTANCE_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='InstanceId'].OutputValue" --output text)

aws ec2 start-instances --instance-ids "$INSTANCE_ID"

# Tunnel the n8n UI (5678) over SSH
ssh -i ~/.ssh/$KEY_PAIR_NAME.pem -L 5678:localhost:5678 ubuntu@<instance-public-ip>
# then open http://localhost:5678
```

In the n8n editor: **Workflows → Import from File**, select each JSON under `workflows/`, configure credentials, and activate. See [Workflows → Importing](./workflows.md#11-importing-workflows).

---

## 6. Register a Repository

Registration is the primary way to onboard a repo — it validates access, stores the PAT in Secrets Manager, writes metadata to DynamoDB, and **creates the GitHub webhook automatically**.

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

curl -sS -X POST "$REGISTRATION_URL" \
  -H "Content-Type: application/json" \
  -d '{"repository_url":"https://github.com/acme/widget","pat":"github_pat_xxx"}'

# Optional: a custom per-repo trigger (literal prefix or "regex:"):
#   -d '{"repository_url":"...","pat":"...","trigger_pattern":"regex:^(blog|post):"}'
```

On success the platform validates access + token permissions, creates the webhook (pointing at `WebhookUrl`) with a generated secret, stores the PAT + webhook secret in Secrets Manager, and writes metadata (including the secret reference and default trigger pattern `blog:`) to DynamoDB. Confirm a green **✓** under the repo's **Settings → Webhooks → Recent Deliveries**.

> The webhook front door (API Gateway + Lambda + EventBridge + SQS) is **always available**, even when the EC2 instance is stopped. The handler acknowledges every push; it only publishes an event — and starts the instance — when the commit message matches the repository's trigger pattern (default `blog:`).

### Manual webhook setup (fallback)

If you prefer to create the webhook yourself: **Settings → Webhooks → Add webhook** → Payload URL = `WebhookUrl`, Content type = `application/json`, Secret = the repository's webhook secret, Events = **push** and **release** (registration subscribes to both automatically).

---

## 7. Deployment Verification

```bash
# Stack outputs
aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs" --output table

# Queue exists
aws sqs get-queue-attributes --queue-url "$QUEUE_URL" \
  --attribute-names ApproximateNumberOfMessages

# Handler + registration Lambdas are deployed
aws lambda get-function --function-name blog-gen-webhook-handler --query 'Configuration.State'
aws lambda get-function --function-name blog-gen-registration --query 'Configuration.State'

# Metadata table is active
aws dynamodb describe-table --table-name blog-gen-repositories --query 'Table.TableStatus'

# EC2 instance is registered (likely 'stopped' until a webhook arrives)
aws ec2 describe-instances --instance-ids "$INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].State.Name'
```

**Smoke test:** verify both paths of the trigger gate.

1. **Ignored path** — push a normal commit (e.g. `docs: tweak README`). Expect a green ✓ in GitHub, `WebhookIgnored` in CloudWatch, and **no** instance start.
2. **Triggered path** — push a commit whose message starts with `blog:` (e.g. `blog: smoke test`), or **Redeliver** such a delivery.

Expected sequence for the triggered path:

| Check | Expected |
| --- | --- |
| Registration | `RegistrationUrl` returns success; DynamoDB item present; webhook created in GitHub |
| GitHub Recent Deliveries | Green ✓ (HTTP 200) |
| CloudWatch (handler) | Logs show verify → trigger match → PutEvents |
| EventBridge / SQS | Event published; SQS count increments, then drains |
| EC2 | Transitions `stopped` → `running` after the matched event |
| CloudWatch (n8n) | Run logs: analyze → memory → generate → review → publish |
| Published output | New Markdown content at the configured destination |
| EC2 (after idle timeout) | Transitions back to `stopped` |

---

## 8. Rollback

CloudFormation-managed infrastructure makes rollback deterministic. A **failed stack update rolls back automatically** to the last good state.

**Infrastructure rollback** — re-deploy an earlier known-good template revision:

```bash
git checkout <previous-good-sha> -- infrastructure/
aws cloudformation deploy \
  --template-file infrastructure/compute.yaml \
  --stack-name blog-gen-compute \
  --parameter-overrides InstanceType=$INSTANCE_TYPE SpotMaxPrice=$SPOT_MAX_PRICE KeyPairName=$KEY_PAIR_NAME \
  --capabilities CAPABILITY_NAMED_IAM
```

CloudFormation computes and applies only the diff. Inspect changes with `aws cloudformation describe-stack-events`.

**Lambda rollback** — deploy a previous artifact (use published versions/aliases for instant revert).

**Persistent data** — the gp3 EBS volume is retained across instance replacement, so models and n8n state survive a compute rollback.

> **Guardrail:** never run `aws cloudformation delete-stack` against a shared environment as a rollback mechanism. Prefer a targeted re-deploy from a known-good commit.
