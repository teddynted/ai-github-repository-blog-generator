# Deployment

This guide covers deploying the **AI GitHub Repository Blog Generator** to AWS using CloudFormation.

Related: [Local Development](./local-development.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [CI/CD](./ci-cd.md).

---

## 1. AWS Prerequisites

| Requirement | Notes |
| --- | --- |
| AWS account | With permissions to create VPC, EC2, Lambda, S3, IAM, EventBridge, Secrets Manager, CloudWatch |
| **Amazon Bedrock model access** | Request access to the target foundation model in your Region (Bedrock → Model access) |
| Region | A Region where Bedrock and all services are available (e.g. `us-east-1`) |
| AWS CLI v2 | Configured with credentials (`aws configure` or SSO) |
| `cfn-lint` | Template linting ([Install](https://github.com/aws-cloudformation/cfn-lint)) |
| Go ≥ 1.22 | To build Lambda binaries |
| Docker | To build/package artifacts locally if needed |

Verify access:

```bash
aws sts get-caller-identity
aws bedrock list-foundation-models --region us-east-1 --query "modelSummaries[].modelId" --output table
```

---

## 2. Bootstrap the Artifacts Bucket

CloudFormation tracks stack state itself, so there is no remote state or lock table to manage. You only need an S3 **artifacts bucket** that `aws cloudformation package` uses to upload nested-stack templates and the Lambda deployment ZIP. Create it **once** per account/region:

```bash
./scripts/bootstrap.sh --region us-east-1 --artifacts-bucket <your-artifacts-bucket>
```

This creates a versioned, encrypted S3 bucket. `package` uploads the local templates and Lambda artifact here and rewrites the template to reference the uploaded objects before `deploy`.

---

## 3. Stack Parameters & Configuration

Runtime configuration is provided through **CloudFormation parameters** (`cloudformation/parameters.json`) and **environment variables** consumed by n8n and the Lambdas.

Copy and edit the example parameters file:

```bash
cp cloudformation/parameters.example.json cloudformation/parameters.json
```

| Parameter | Description | Example |
| --- | --- | --- |
| `AwsRegion` | Deployment region | `us-east-1` |
| `ProjectName` | Resource name prefix | `blog-generator` |
| `Environment` | Environment tag | `prod` |
| `BedrockModelId` | Foundation model ID / inference profile | `us.anthropic.claude-sonnet-4-...` |
| `BedrockMaxTokens` | Max output tokens | `4096` |
| `BedrockTemperature` | Sampling temperature | `0.3` |
| `Ec2InstanceType` | n8n host size | `t3.small` |
| `N8nOperatingSchedule` | Cron for EC2 start/stop | see [Cost Optimization](./cost-optimization.md) |
| `ArtifactsRetentionDays` | Artifact bucket expiry | `7` |
| `LogRetentionDays` | CloudWatch retention | `14` |
| `NotificationEmail` | SNS subscription target | `ops@example.com` |

`parameters.json` uses the CLI `--parameter-overrides` format (`Key=Value` pairs or a JSON list). Non-secret environment variables passed to n8n/Lambdas are set by the stack; secrets are injected from Secrets Manager (next section).

---

## 4. Secrets

Never commit secrets. Store them in **AWS Secrets Manager**.

**Deployment-level secrets** — CloudFormation provisions the containers; you populate the values out of band:

| Secret | Purpose |
| --- | --- |
| `blog-generator/n8n-credentials` | n8n encryption key and basic-auth credentials |
| `blog-generator/notification-webhook` | Slack/webhook URL (if used) |
| `blog-generator/registration-token` | Admin token protecting the registration endpoint |

**Per-repository secrets** — created **automatically at registration** under the `blog-generator/repos/<owner>/<name>/` prefix (`pat` and `webhook-secret`). You do **not** create these manually; the registration workflow does, and their ARNs are recorded in DynamoDB (never the values).

Populate a deployment-level secret after the stack deploys:

```bash
aws secretsmanager put-secret-value \
  --secret-id blog-generator/n8n-credentials \
  --secret-string '{"encryptionKey":"...","user":"admin","password":"..."}' \
  --region us-east-1
```

See [Security](./security.md) for rotation and access policy.

---

## 5. IAM Permissions

The deploying principal needs permissions to manage the resource set. For CI, use a dedicated role (OIDC-federated GitHub Actions role — see [CI/CD](./ci-cd.md)) scoped to this project's resources.

At runtime, CloudFormation creates **least-privilege roles**:

| Role | Key permissions |
| --- | --- |
| n8n EC2 instance role | `bedrock:InvokeModel`; `secretsmanager:CreateSecret/PutSecretValue/GetSecretValue` on `blog-generator/repos/*`; `dynamodb:GetItem/PutItem/UpdateItem/Query` on the `repositories` table; `s3:PutObject/GetObject` (generated-content); `sns:Publish`; CloudWatch |
| `ec2-scheduler` Lambda role | `ec2:StartInstances` / `ec2:StopInstances` (the n8n instance only), CloudWatch Logs |

Full policy detail: [Security → Least Privilege](./security.md#3-least-privilege).

---

## 6. Deploy

```bash
# Lint templates
cfn-lint cloudformation/**/*.yaml

# Package: upload nested-stack templates + Lambda artifact to the artifacts bucket
aws cloudformation package \
  --template-file cloudformation/main.yaml \
  --s3-bucket <your-artifacts-bucket> \
  --output-template-file packaged.yaml

# Deploy (creates or updates the stack via a change set)
aws cloudformation deploy \
  --template-file packaged.yaml \
  --stack-name blog-generator \
  --parameter-overrides file://cloudformation/parameters.json \
  --capabilities CAPABILITY_NAMED_IAM
```

> To review changes before they apply, use a change set instead: `aws cloudformation create-change-set … --change-set-name review`, inspect it with `describe-change-set`, then `execute-change-set`. CI does exactly this (see [CI/CD](./ci-cd.md)).

Then import the n8n workflows (see [Workflows](./workflows.md#9-importing-workflows)) and populate secrets (Section 4).

---

## 6a. Register a Repository

Registration is the primary way to onboard a repo — it stores metadata, secures the PAT, and **creates the webhook automatically**.

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-generator \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

curl -sS -X POST "$REGISTRATION_URL" \
  -H "Authorization: Bearer $REGISTRATION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"repository_url":"https://github.com/acme/widget","pat":"github_pat_xxx"}'
```

On success the platform validates access, stores the PAT in Secrets Manager, writes metadata to DynamoDB, creates the webhook (if the PAT permits), and triggers an initial analysis. Confirm the item in DynamoDB and a green **✓** under the repo's **Settings → Webhooks → Recent Deliveries**.

### Manual webhook setup (fallback)

If the PAT lacks webhook permission, registration returns manual instructions — add a webhook yourself:

1. **Payload URL:** `https://<webhook-host>/webhook/github` (the `WebhookUrl` stack output)
2. **Content type:** `application/json`
3. **Secret:** the value the registration response reports (stored at `blog-generator/repos/<owner>/<name>/webhook-secret`).
4. **Events:** *push*, *release*, *pull request*, *repository*.
5. Save and confirm a green **✓** under **Recent Deliveries**.

> The endpoint is only reachable while the EC2 host is running (its 19:00–21:00 window). GitHub retries failed deliveries; you can widen the window via the `Ec2StartCron` / `Ec2StopCron` parameters.

---

## 7. Deployment Verification

After the stack reaches `CREATE_COMPLETE`/`UPDATE_COMPLETE`, verify each layer:

```bash
# Outputs (bucket names, instance id, function arns)
aws cloudformation describe-stacks --stack-name blog-generator \
  --query "Stacks[0].Outputs" --output table

# n8n management UI via SSM port-forward (UI is not publicly exposed)
aws ssm start-session --target <instance-id> \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["5678"],"localPortNumber":["5678"]}'
# then open http://localhost:5678

# Webhook endpoint responds over HTTPS (while EC2 is running)
WEBHOOK_URL=$(aws cloudformation describe-stacks --stack-name blog-generator \
  --query "Stacks[0].Outputs[?OutputKey=='WebhookUrl'].OutputValue" --output text)
curl -sSf -o /dev/null -w '%{http_code}\n' "$WEBHOOK_URL/health" || true

# Scheduler Lambda is deployed
aws lambda get-function --function-name blog-generator-ec2-scheduler --query 'Configuration.State'

# Repository metadata table exists and is active
aws dynamodb describe-table --table-name blog-generator-repositories --query 'Table.TableStatus'

# Bedrock access works
aws bedrock-runtime invoke-model --model-id "$BEDROCK_MODEL_ID" \
  --body '{"anthropic_version":"bedrock-2023-05-31","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}' \
  --cli-binary-format raw-in-base64-out /dev/stdout
```

**Smoke test:** register a test repository (§6a), then **Redeliver** a webhook (or push a commit) and confirm a content package appears in the generated-content bucket and a notification is received.

| Check | Expected |
| --- | --- |
| Stack outputs | All outputs populated |
| DynamoDB `repositories` | Item present after registration; `webhook_status = active` |
| GitHub Recent Deliveries | Green ✓ (2xx) response from the webhook |
| Generated-content bucket | New `generated-content/<repo>/<YYYY-MM-DD>/` package after a run |
| CloudWatch dashboard | Run/success metrics increment |
| Notification | Success message received |

---

## 8. Rollback

CloudFormation-managed infrastructure and versioned artifacts make rollback deterministic. A **failed stack update rolls back automatically** to the last good state; the steps below cover a deliberate rollback to an earlier template revision.

**Infrastructure rollback**

```bash
# Roll back to a previous known-good commit of the IaC, then re-deploy
git checkout <previous-good-sha> -- cloudformation/
aws cloudformation package \
  --template-file cloudformation/main.yaml \
  --s3-bucket <your-artifacts-bucket> \
  --output-template-file packaged.yaml
aws cloudformation deploy \
  --template-file packaged.yaml \
  --stack-name blog-generator \
  --parameter-overrides file://cloudformation/parameters.json \
  --capabilities CAPABILITY_NAMED_IAM
```

CloudFormation computes and applies only the diff. If an update fails, the stack returns to `UPDATE_ROLLBACK_COMPLETE` on its own; use `aws cloudformation describe-stack-events` to see what changed.

**Content rollback** — generated packages use S3 versioning; retrieve any prior version:

```bash
aws s3api list-object-versions --bucket <generated-content-bucket> --prefix generated-content/<repo>/
aws s3api get-object --bucket <generated-content-bucket> --key <key> --version-id <id> restored.md
```

**Lambda rollback** — for `ec2-scheduler`, deploy a previous artifact/alias, or re-deploy the stack with a prior code version. Use published versions/aliases for instant revert.

> **Guardrail:** never run `aws cloudformation delete-stack` against a shared environment as a rollback mechanism. Prefer a targeted re-deploy from a known-good commit.
