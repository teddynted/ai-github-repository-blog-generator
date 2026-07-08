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
| Go ≥ 1.22 | To build the Lambda binaries |
| Docker | For local build/testing (optional) |

> **No Amazon Bedrock, OpenAI, or Anthropic access is required.** All inference runs locally via Ollama on the instance.

Verify access:

```bash
aws sts get-caller-identity
aws ec2 describe-key-pairs --key-names "$KEY_PAIR_NAME"
```

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
| `INSTANCE_TYPE` | EC2 instance type for the Spot request | `g4dn.xlarge` |
| `SPOT_MAX_PRICE` | Maximum Spot price you will pay | `0.20` |
| `IDLE_TIMEOUT_MINUTES` | Minutes of inactivity before auto-shutdown | `15` |
| `OLLAMA_MODEL` | Local model to run | `qwen2.5:7b` |
| `EBS_VOLUME_SIZE_GB` | Size of the persistent gp3 volume | `100` |
| `KEY_PAIR_NAME` | EC2 key pair for SSH | `blog-generator-key` |
| `LogRetentionDays` | CloudWatch retention | `14` |
| `OperatorCidr` | CIDR allowed to SSH to the instance | `203.0.113.10/32` |

> **Never commit secrets.** `WEBHOOK_SECRET` and any tokens are provided via environment/parameter input at deploy time, not stored in the repository ([Security](./security.md)).

---

## 3. Build the Lambda Functions

```bash
# Webhook handler
( cd lambdas/webhook-handler && \
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lambda && \
  zip webhook-handler.zip bootstrap )

# Idle shutdown
( cd lambdas/idle-shutdown && \
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lambda && \
  zip idle-shutdown.zip bootstrap )
```

Upload the ZIPs to a deployment bucket (or reference them inline), depending on your pipeline.

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

# 2. Serverless layer (API Gateway, Lambda, SQS + DLQ, EventBridge)
aws cloudformation deploy \
  --template-file infrastructure/serverless.yaml \
  --stack-name blog-gen-serverless \
  --parameter-overrides WebhookSecret=$WEBHOOK_SECRET \
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

In the n8n editor: **Workflows → Import from File**, select each JSON under `workflows/`, configure credentials, and activate. See [Workflows → Importing](./workflows.md#8-importing-workflows).

---

## 6. Configure the GitHub Webhook

In your repository: **Settings → Webhooks → Add webhook**.

1. **Payload URL:** the `WebhookUrl` stack output.
2. **Content type:** `application/json`.
3. **Secret:** the same value as `WEBHOOK_SECRET`.
4. **Events:** *push*, *release*, *pull request* (or "Send me everything" and let the handler filter).
5. Save and confirm a green **✓** under **Recent Deliveries**.

> The webhook front door (API Gateway + Lambda + SQS) is **always available**, even when the EC2 instance is stopped — the handler enqueues the event and starts the instance on demand.

---

## 7. Deployment Verification

```bash
# Stack outputs
aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs" --output table

# Queue exists
aws sqs get-queue-attributes --queue-url "$QUEUE_URL" \
  --attribute-names ApproximateNumberOfMessages

# Handler Lambda is deployed
aws lambda get-function --function-name blog-gen-webhook-handler \
  --query 'Configuration.State'

# EC2 instance is registered (likely 'stopped' until a webhook arrives)
aws ec2 describe-instances --instance-ids "$INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].State.Name'
```

**Smoke test:** push a commit (or use GitHub's **Recent Deliveries → Redeliver**). Expected sequence:

| Check | Expected |
| --- | --- |
| GitHub Recent Deliveries | Green ✓ (HTTP 200) |
| SQS | Message count increments, then drains |
| EC2 | Transitions `stopped` → `running` after the webhook |
| CloudWatch | Handler logs show validate → enqueue → start; n8n logs show the run |
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
