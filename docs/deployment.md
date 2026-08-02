# Deployment

This guide covers deploying the **GitHub AI Blog Generator** to AWS using **CloudFormation**, then registering a repository and triggering a run with `POST /process`.

Related: [Local Development](./local-development.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [CI/CD](./ci-cd.md).

---

## 1. AWS Prerequisites

| Requirement | Notes |
| --- | --- |
| AWS account | With permissions to create VPC, EC2, Lambda, API Gateway, Step Functions, SQS, IAM, and CloudWatch |
| Region | Any Region where these services are available (e.g. `us-east-1`) |
| AWS CLI v2 | Configured with credentials (`aws configure` or SSO) |
| EC2 key pair | For SSH access to the instance |
| `cfn-lint` | Template linting ([Install](https://github.com/aws-cloudformation/cfn-lint)) |
| Go ≥ 1.25 | To build the Lambda binaries (build toolchain pinned in `go.mod`) |
| Docker | For local build/testing (optional) |

> **Amazon Bedrock access** to a Claude model (`us.anthropic.claude-opus-4-8`) **and/or** an **Anthropic API key** is required — the Provider Router uses Bedrock first and falls back to the Anthropic API.
>
> The instance only runs n8n + the API-calling worker, so the default is a small **t4g.small (arm64/Graviton)** — no GPU. The host is **On-Demand**, so a start does not depend on Spot capacity.

Verify access:

```bash
aws sts get-caller-identity
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
| Variable | `OPERATOR_CIDR` | your SSH CIDR (optional) |
| Variable | `DEPLOY_ENABLED` | `true` |

With those set, merging to `main` runs [`deploy.yml`](./ci-cd.md#enabling-deployyml), which packages the Lambdas and deploys **network → serverless → compute → scheduler → observability**. For a manual deploy instead, set `ARTIFACTS_BUCKET` locally and follow Sections 3–4.

> **`… bucket … already exists`** — the artifacts bucket has `DeletionPolicy: Retain`, so a previously deleted `blog-gen-bootstrap` stack leaves the physical bucket behind. Re-creating then collides with it (often leaving the stack in `REVIEW_IN_PROGRESS`). Re-run with `IMPORT_EXISTING=1`, which **adopts** the existing bucket (keeping its contents — it is versioned) via a change set with `--import-existing-resources`, and first clears any leftover `REVIEW_IN_PROGRESS`/failed stack automatically. Requires AWS CLI ≥ 2.22.
>
> ```bash
> IMPORT_EXISTING=1 scripts/bootstrap.sh --region us-east-1 --owner <you> --repo <repo>
> ```

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
| `BEDROCK_MODEL_ID` | Provider Router primary — Bedrock Claude model id | `us.anthropic.claude-opus-4-8` |
| `ANTHROPIC_MODEL` | Provider Router fallback — Anthropic API model id | `claude-opus-4-8` |
| `PUBLISH_TRIGGER` | Optional trigger pattern stored in metadata | `blog:` |
| `INSTANCE_TYPE` | EC2 instance type (arm64/Graviton) | `t4g.small` |
| `StartExpression` | Scheduler cron for the daily START (scheduler stack) | `cron(0 18 ? * * *)` |
| `StopExpression` | Scheduler cron for the daily STOP (scheduler stack) | `cron(0 20 ? * * *)` |
| `ScheduleTimezone` | IANA timezone the crons evaluate in (scheduler stack) | `Etc/UTC` |
| `EBS_VOLUME_SIZE_GB` | Size of the persistent gp3 volume | `20` |
| `KeyPairName` | (optional) existing key pair; blank = stack-managed | (managed) |
| `REQUIRE_HUMAN_APPROVAL` | Require manual approval before publishing | `false` |
| `LogRetentionDays` | CloudWatch retention | `14` |
| `OperatorCidr` | CIDR allowed to SSH to the instance | `203.0.113.10/32` |

> **Never commit secrets.** The Anthropic API key and any GitHub tokens live in Secrets Manager, provided out-of-band at deploy time, not stored in the repository ([Security](./security.md)).

---

## 3. Build the Lambda Functions

```bash
# Single Go module — build every implemented function to dist/<fn>/bootstrap
make build

# Package each for Lambda (provided.al2023, arm64)
for fn in registration manual-trigger release-context scheduled-start scheduled-stop idle-stop; do
  [ -f "dist/$fn/bootstrap" ] && ( cd dist/$fn && zip $fn.zip bootstrap )
done
```

- `registration` — validate repo + PAT, store metadata + secret.
- `manual-trigger` — the authenticated `POST /process` endpoint; validates the request and **starts a Step Functions execution** (202 accepted). See [Manual Trigger](./manual-trigger.md).
- `release-context` — the authenticated `POST /release-context` endpoint; builds the structured Release Context.
- `scheduled-start` — start the On-Demand instance at 18:00 daily (scheduler stack).
- `scheduled-stop` — stop the instance at 20:00 daily (scheduler stack).

Upload the ZIPs to the artifacts bucket (from the [bootstrap](#first-time-bootstrap-automated-deploy)); the serverless/scheduler templates' default code keys are `<fn>.zip` at the bucket root:

```bash
for fn in registration manual-trigger release-context scheduled-start scheduled-stop idle-stop; do
  aws s3 cp "dist/$fn/$fn.zip" "s3://$ARTIFACTS_BUCKET/$fn.zip"
done
```

---

## 4. Deploy the Stacks

Deploy in order: **network → serverless → compute → scheduler → observability**. Templates are modular and reusable.

```bash
# 1. Network layer
aws cloudformation deploy \
  --template-file infrastructure/network.yaml \
  --stack-name blog-gen-network \
  --parameter-overrides OperatorCidr=$OperatorCidr \
  --capabilities CAPABILITY_NAMED_IAM

# 2. Serverless layer (API Gateway, Lambdas, Step Functions, SQS + DLQ)
aws cloudformation deploy \
  --template-file infrastructure/serverless.yaml \
  --stack-name blog-gen-serverless \
  --parameter-overrides ArtifactsBucket=$ARTIFACTS_BUCKET PublishTrigger=$PUBLISH_TRIGGER \
  --capabilities CAPABILITY_NAMED_IAM

# 3. Compute layer (On-Demand EC2 + persistent EBS + worker service)
#    Build & upload the worker binary first (linux/arm64 — the t4g host is Graviton):
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/worker/worker ./cmd/worker
aws s3 cp dist/worker/worker "s3://$ARTIFACTS_BUCKET/worker"
aws cloudformation deploy \
  --template-file infrastructure/compute.yaml \
  --stack-name blog-gen-compute \
  --parameter-overrides \
      InstanceType=$INSTANCE_TYPE \
      BedrockModelId=$BEDROCK_MODEL_ID \
      AnthropicModel=$ANTHROPIC_MODEL \
      EbsVolumeSizeGb=$EBS_VOLUME_SIZE_GB \
      ArtifactsBucket=$ARTIFACTS_BUCKET \
      WorkerCodeKey=worker \
      NotifyEmailFrom=$NOTIFY_EMAIL_FROM \
      NotifyEmailTo=$NOTIFY_EMAIL_TO \
      SmtpUsername=$SMTP_USERNAME \
  --capabilities CAPABILITY_NAMED_IAM

# Set the Anthropic API key (Provider Router fallback leg) out-of-band:
ANTHROPIC_SECRET_ARN=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='AnthropicApiKeySecretArn'].OutputValue" --output text 2>/dev/null || echo "blog-gen/anthropic/api-key")
aws secretsmanager put-secret-value --secret-id "$ANTHROPIC_SECRET_ARN" --secret-string 'sk-ant-...'

# 4. Scheduler layer — owns instance power (start 18:00 / stop 20:00, daily).
#    Wire it to the compute stack's InstanceId:
INSTANCE_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='InstanceId'].OutputValue" --output text)
aws cloudformation deploy \
  --template-file infrastructure/scheduler.yaml \
  --stack-name blog-gen-scheduler \
  --parameter-overrides \
      InstanceId=$INSTANCE_ID \
      ArtifactsBucket=$ARTIFACTS_BUCKET \
  --capabilities CAPABILITY_NAMED_IAM
# (Override ScheduleTimezone/StartExpression/StopExpression to change the window.)

# 5. Observability layer
aws cloudformation deploy \
  --template-file infrastructure/observability.yaml \
  --stack-name blog-gen-observability \
  --capabilities CAPABILITY_NAMED_IAM
```

Set the SMTP password (email notifications). The compute stack creates the
secret container; the value is set out-of-band so it never appears in the
template or parameter history:

```bash
# The stack exports the secret ARN as an output:
SMTP_SECRET_ARN=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='NotificationsSecretArn'].OutputValue" --output text)

aws secretsmanager put-secret-value \
  --secret-id "$SMTP_SECRET_ARN" \
  --secret-string 'YOUR_TURBO_SMTP_PASSWORD'
```

The worker reads it at startup via `SMTP_PASSWORD_SECRET`; restart the service
(or the instance) to pick up a changed value: `sudo systemctl restart blog-gen-worker`.
Email is optional — leave `NotifyEmail*`/`SmtpUsername` unset to disable it.

Retrieve the `/process` endpoint URL:

```bash
aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='ProcessUrl'].OutputValue" --output text
```

---

## 5. Import the n8n Workflows

The instance is stopped outside its scheduled window. To import workflows, start it once (manually or wait for the 18:00 window) and open n8n through an SSH tunnel (the UI is not publicly exposed):

```bash
INSTANCE_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='InstanceId'].OutputValue" --output text)

aws ec2 start-instances --instance-ids "$INSTANCE_ID"

# Retrieve the stack-managed private key from SSM, then tunnel the n8n UI (5678)
PARAM=$(aws cloudformation describe-stacks --stack-name blog-gen-compute \
  --query "Stacks[0].Outputs[?OutputKey=='ManagedKeyPrivateKeyParam'].OutputValue" --output text)
aws ssm get-parameter --name "$PARAM" --with-decryption --query Parameter.Value --output text > blog-gen-key.pem
chmod 400 blog-gen-key.pem
ssh -i blog-gen-key.pem -L 5678:localhost:5678 ubuntu@<instance-public-ip>
# then open http://localhost:5678
```

In the n8n editor: **Workflows → Import from File**, select each JSON under `workflows/`, configure credentials, and activate. See [Workflows → Importing](./workflows.md#11-importing-workflows).

---

## 6. Register a Repository

Registration validates access and stores the repo's PAT in the **shared** Secrets Manager secret (`blog-gen/github/repositories`). GitHub webhook ingress has been removed, so registration no longer creates a GitHub webhook. Full endpoint spec (payload, responses, errors, delete): [Registration](./registration.md).

**1. Create the GitHub PAT** (in GitHub, not via the endpoint): **Settings → Developer settings → Personal access tokens → Fine-grained tokens**, scoped to the target repo with **Contents: Read**, **Pull requests: Read and write**, **Metadata: Read**. Copy the `github_pat_…` value.

**2. POST it to the registration endpoint** with `owner`, `repository`, and `pat`.

> ⚠️ **The registration and `/process` routes require an API key** (`ApiKeyRequired: true`). Send the `x-api-key` header or you get **403 Forbidden**.

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

API_KEY_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationApiKeyId'].OutputValue" --output text)
API_KEY=$(aws apigateway get-api-key --api-key "$API_KEY_ID" --include-value --query value --output text)

curl -sS -X POST "$REGISTRATION_URL" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d '{"owner":"acme","repository":"widget","pat":"github_pat_xxx"}'

# Rotate the PAT for an existing repo (add "update": true):
#   -d '{"owner":"acme","repository":"widget","pat":"github_pat_new","update":true}'
# Deregister a repo:
#   curl -sS -X DELETE "$REGISTRATION_URL" -H "x-api-key: $API_KEY" \
#     -H "Content-Type: application/json" -d '{"owner":"acme","repository":"widget"}'
```

On success the platform validates access + token permissions, adds the PAT to the shared secret keyed by `acme/widget`, and writes metadata to DynamoDB.

### Trigger a run

`POST /process` starts the pipeline. It returns **202** and starts a Step Functions execution that boots the host and enqueues the job:

```bash
PROCESS_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='ProcessUrl'].OutputValue" --output text)

curl -sS -X POST "$PROCESS_URL" -H "x-api-key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"owner":"acme","repository":"widget","releaseTag":"v1.2.0"}'   # omit releaseTag for a repo run
```

---

## 7. Deployment Verification

```bash
# Stack outputs
aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs" --output table

# Queue exists
aws sqs get-queue-attributes --queue-url "$QUEUE_URL" \
  --attribute-names ApproximateNumberOfMessages

# Manual-trigger + registration Lambdas are deployed
aws lambda get-function --function-name blog-gen-manual-trigger --query 'Configuration.State'
aws lambda get-function --function-name blog-gen-registration --query 'Configuration.State'

# Metadata table is active
aws dynamodb describe-table --table-name blog-gen-repositories --query 'Table.TableStatus'

# EC2 instance is registered (likely 'stopped' until a /process call arrives)
aws ec2 describe-instances --instance-ids "$INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].State.Name'
```

On the instance (via SSH), confirm the orchestration plane and worker are up:

```bash
docker compose -f /opt/blog-gen/docker-compose.yml ps   # n8n + postgres + redis running
curl -s http://127.0.0.1:5678/healthz                   # n8n ready
systemctl status blog-gen-worker                        # active (running)
journalctl -u blog-gen-worker -n 50                     # recent worker logs
```

**Smoke test:** call `POST /process` (see [Trigger a run](#trigger-a-run)) and follow the execution.

Expected sequence:

| Check | Expected |
| --- | --- |
| Registration | `RegistrationUrl` returns success; DynamoDB item present |
| `POST /process` | HTTP 202; a Step Functions execution starts |
| Step Functions console | Execution: FindInstance → StartInstance → WaitForBoot → CheckSSM → EnqueueJob |
| EC2 | Transitions `stopped` → `running` (started by the state machine) |
| SQS | Job enqueued; count increments then drains once the worker is up |
| CloudWatch (worker) | Run logs: analyze → memory → generate (Bedrock → Anthropic) → S3 → review |
| Published output | New Markdown in the content S3 bucket (reused on re-runs) |
| EC2 (scheduled-stop / idle-stop) | Transitions back to `stopped` |

---

## 8. Rollback

CloudFormation-managed infrastructure makes rollback deterministic. A **failed stack update rolls back automatically** to the last good state.

**Infrastructure rollback** — re-deploy an earlier known-good template revision:

```bash
git checkout <previous-good-sha> -- infrastructure/
aws cloudformation deploy \
  --template-file infrastructure/compute.yaml \
  --stack-name blog-gen-compute \
  --parameter-overrides InstanceType=$INSTANCE_TYPE \
  --capabilities CAPABILITY_NAMED_IAM
```

CloudFormation computes and applies only the diff. Inspect changes with `aws cloudformation describe-stack-events`.

**Lambda rollback** — deploy a previous artifact (use published versions/aliases for instant revert).

**Persistent data** — the gp3 EBS volume is retained across instance replacement, so n8n + PostgreSQL state and Repository Memory survive a compute rollback.

> **Guardrail:** never run `aws cloudformation delete-stack` against a shared environment as a rollback mechanism. Prefer a targeted re-deploy from a known-good commit.
