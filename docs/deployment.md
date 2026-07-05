# Deployment

This guide covers deploying the **AI GitHub Repository Blog Generator** to AWS using Terraform.

Related: [Local Development](./local-development.md) · [Infrastructure](./infrastructure.md) · [Security](./security.md) · [CI/CD](./ci-cd.md).

---

## 1. AWS Prerequisites

| Requirement | Notes |
| --- | --- |
| AWS account | With permissions to create VPC, EC2, Lambda, S3, IAM, EventBridge, Secrets Manager, CloudWatch |
| **Amazon Bedrock model access** | Request access to the target foundation model in your Region (Bedrock → Model access) |
| Region | A Region where Bedrock and all services are available (e.g. `us-east-1`) |
| AWS CLI v2 | Configured with credentials (`aws configure` or SSO) |
| Terraform ≥ 1.6 | [Install](https://developer.hashicorp.com/terraform/downloads) |
| Go ≥ 1.22 | To build Lambda binaries |
| Docker | To build/package artifacts locally if needed |

Verify access:

```bash
aws sts get-caller-identity
aws bedrock list-foundation-models --region us-east-1 --query "modelSummaries[].modelId" --output table
```

---

## 2. Bootstrap Remote State

Terraform state is stored in S3 with a DynamoDB lock table. Create the backend **once** per account/region before the first `init`:

```bash
./scripts/bootstrap.sh --region us-east-1 --state-bucket <your-tfstate-bucket> --lock-table tf-locks
```

This creates a versioned, encrypted S3 bucket and a DynamoDB table. Then configure `terraform/backend.tf`:

```hcl
terraform {
  backend "s3" {
    bucket         = "<your-tfstate-bucket>"
    key            = "blog-generator/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "tf-locks"
    encrypt        = true
  }
}
```

---

## 3. Environment Variables & Configuration

Runtime configuration is provided through **Terraform variables** (`terraform/terraform.tfvars`) and **environment variables** consumed by n8n and the Lambdas.

Copy and edit the example tfvars:

```bash
cp terraform/terraform.tfvars.example terraform/terraform.tfvars
```

| Variable | Description | Example |
| --- | --- | --- |
| `aws_region` | Deployment region | `us-east-1` |
| `project_name` | Resource name prefix | `blog-generator` |
| `environment` | Environment tag | `prod` |
| `bedrock_model_id` | Foundation model ID / inference profile | `us.anthropic.claude-sonnet-4-...` |
| `bedrock_max_tokens` | Max output tokens | `4096` |
| `bedrock_temperature` | Sampling temperature | `0.3` |
| `ec2_instance_type` | n8n host size | `t3.small` |
| `n8n_operating_schedule` | Cron for EC2 start/stop | see [Cost Optimization](./cost-optimization.md) |
| `artifacts_retention_days` | Artifact bucket expiry | `7` |
| `log_retention_days` | CloudWatch retention | `14` |
| `notification_email` | SNS subscription target | `ops@example.com` |

Environment variables passed to n8n/Lambdas (non-secret) are set by Terraform; secrets are injected from Secrets Manager (next section).

---

## 4. Secrets

Never commit secrets. Store them in **AWS Secrets Manager**; Terraform provisions the secret *containers*, and you populate the *values* out of band.

| Secret | Purpose |
| --- | --- |
| `blog-generator/github-token` | GitHub PAT for cloning private repos / API metadata |
| `blog-generator/n8n-credentials` | n8n encryption key and basic-auth credentials |
| `blog-generator/notification-webhook` | Slack/webhook URL (if used) |

Populate a secret value after `apply`:

```bash
aws secretsmanager put-secret-value \
  --secret-id blog-generator/github-token \
  --secret-string '{"token":"ghp_xxx"}' \
  --region us-east-1
```

See [Security](./security.md) for rotation and access policy.

---

## 5. IAM Permissions

The deploying principal needs permissions to manage the resource set. For CI, use a dedicated role (OIDC-federated GitHub Actions role — see [CI/CD](./ci-cd.md)) scoped to this project's resources.

At runtime, Terraform creates **least-privilege roles**:

| Role | Key permissions |
| --- | --- |
| `repo-cloner` Lambda role | `s3:PutObject` (artifacts), `secretsmanager:GetSecretValue` (github-token), CloudWatch Logs |
| `repo-analyzer` Lambda role | `s3:GetObject/PutObject` (artifacts), CloudWatch Logs |
| `blog-publisher` Lambda role | `s3:PutObject` (posts), `sns:Publish`, CloudWatch metrics/logs |
| n8n EC2 instance role | `bedrock:InvokeModel`, `lambda:InvokeFunction`, `secretsmanager:GetSecretValue`, S3 read/write, CloudWatch |

Full policy detail: [Security → Least Privilege](./security.md#3-least-privilege).

---

## 6. Deploy

```bash
cd terraform
terraform init
terraform fmt -check
terraform validate
terraform plan -out tfplan
terraform apply tfplan
```

Then import the n8n workflows (see [Workflows](./workflows.md#8-importing-workflows)) and populate secrets (Section 4).

---

## 7. Deployment Verification

After `apply`, verify each layer:

```bash
# Outputs (bucket names, instance id, function arns)
terraform output

# n8n reachable via SSM port-forward (no public ingress)
aws ssm start-session --target <instance-id> \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["5678"],"localPortNumber":["5678"]}'
# then open http://localhost:5678

# Lambdas are deployed
aws lambda get-function --function-name blog-generator-repo-cloner --query 'Configuration.State'

# Bedrock access works
aws bedrock-runtime invoke-model --model-id "$BEDROCK_MODEL_ID" \
  --body '{"anthropic_version":"bedrock-2023-05-31","max_tokens":16,"messages":[{"role":"user","content":"ping"}]}' \
  --cli-binary-format raw-in-base64-out /dev/stdout
```

**Smoke test:** trigger the ingestion workflow with a known public repo and confirm a `.md` object appears in the posts bucket and a notification is received.

| Check | Expected |
| --- | --- |
| `terraform output` | All outputs populated |
| Posts bucket | New `posts/.../<timestamp>.md` after a run |
| CloudWatch dashboard | Run/success metrics increment |
| Notification | Success message received |

---

## 8. Rollback

Terraform-managed infrastructure and versioned artifacts make rollback deterministic.

**Infrastructure rollback**

```bash
# Roll back to a previous known-good commit of the IaC
git checkout <previous-good-sha> -- terraform/
cd terraform
terraform plan -out rollback.plan
terraform apply rollback.plan
```

Because state is remote and versioned, you can also inspect prior state versions in the state bucket if needed.

**Content rollback** — generated posts use S3 versioning; retrieve any prior version:

```bash
aws s3api list-object-versions --bucket <posts-bucket> --prefix posts/<owner>/<repo>/
aws s3api get-object --bucket <posts-bucket> --key <key> --version-id <id> restored.md
```

**Lambda rollback** — deploy a previous artifact/alias, or `terraform apply` a prior code version. Use published versions/aliases for instant revert.

> **Guardrail:** never run `terraform destroy` against a shared environment as a rollback mechanism. Prefer targeted re-apply from a known-good commit.
