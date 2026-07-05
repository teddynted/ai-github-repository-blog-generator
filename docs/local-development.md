# Local Development

This guide explains how to run and develop the **AI GitHub Repository Blog Generator** on your machine.

Related: [Deployment](./deployment.md) · [Workflows](./workflows.md) · [Contributing](./contributing.md).

---

## 1. Required Software

| Tool | Minimum version | Purpose |
| --- | --- | --- |
| [Git](https://git-scm.com/) | 2.30 | Clone the repo; the app itself clones target repos |
| [Docker](https://www.docker.com/) | 24 | Run n8n locally |
| [Docker Compose](https://docs.docker.com/compose/) | v2 | Local n8n stack |
| [Terraform](https://developer.hashicorp.com/terraform/downloads) | 1.6 | Infrastructure as Code |
| [AWS CLI](https://docs.aws.amazon.com/cli/) | v2 | AWS access, Bedrock testing |
| [Go](https://go.dev/dl/) | 1.22 | Build and test Lambda functions |
| Make | any | Convenience targets (optional) |

Verify:

```bash
git --version && docker --version && docker compose version
terraform version && aws --version && go version
```

---

## 2. Docker & Docker Compose (local n8n)

The `docker/` directory contains a self-contained n8n stack for local development.

```bash
cd docker
cp .env.example .env      # fill in values (see Section 5)
docker compose up -d
docker compose logs -f n8n
```

n8n is then available at **http://localhost:5678**. Stop with `docker compose down` (add `-v` to remove volumes).

Example `docker/docker-compose.yml` shape:

```yaml
services:
  n8n:
    image: n8nio/n8n:latest
    ports:
      - "5678:5678"
    environment:
      - N8N_BASIC_AUTH_ACTIVE=true
      - N8N_BASIC_AUTH_USER=${N8N_USER}
      - N8N_BASIC_AUTH_PASSWORD=${N8N_PASSWORD}
      - N8N_ENCRYPTION_KEY=${N8N_ENCRYPTION_KEY}
      - AWS_REGION=${AWS_REGION}
    volumes:
      - n8n_data:/home/node/.n8n
volumes:
  n8n_data:
```

---

## 3. AWS CLI Configuration

Local runs still call **Amazon Bedrock**, **S3**, and **Secrets Manager**, so configure credentials:

```bash
aws configure          # or: aws configure sso
aws sts get-caller-identity
```

Use a low-privilege developer profile scoped to a **dev** environment. Never use production credentials for local experimentation.

---

## 4. Building & Testing the Go Lambda

The content pipeline itself runs inside **n8n workflows** (no application Lambda). The only Go function is `ec2-scheduler`, which starts/stops the EC2 host on a schedule ([Cost Optimization](./cost-optimization.md#1-ec2-scheduling)).

```bash
cd lambdas/ec2-scheduler

go mod download
go build ./...
go vet ./...
gofmt -l .            # should print nothing
go test ./... -race -cover
```

Build a deployable artifact (Linux, ARM64):

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap ./cmd/lambda
zip function.zip bootstrap
```

Run it locally against a fixture event (e.g. a `start` or `stop` action):

```bash
go run ./cmd/local --event ./testdata/start.json
```

---

## 5. Environment Configuration

Local configuration lives in `docker/.env` (git-ignored). Start from `.env.example`:

| Variable | Description | Example |
| --- | --- | --- |
| `AWS_REGION` | Region for Bedrock/S3 | `us-east-1` |
| `AWS_PROFILE` | Local credentials profile | `blog-dev` |
| `BEDROCK_MODEL_ID` | Foundation model / inference profile | `us.anthropic.claude-sonnet-4-...` |
| `N8N_USER` / `N8N_PASSWORD` | Local n8n basic auth | `admin` / `change-me` |
| `N8N_ENCRYPTION_KEY` | n8n credential encryption key | random 32+ chars |
| `GENERATED_CONTENT_BUCKET` | Dev content bucket | `blog-generator-dev-generated-content` |
| `GITHUB_TOKEN` | Optional; for private/API access | `ghp_xxx` |

> Secrets in `.env` are for **local dev only**. In AWS, all secrets come from Secrets Manager — see [Security](./security.md).

---

## 6. Running End-to-End Locally

1. Start n8n (`docker compose up -d`).
2. Import the workflow JSON from `workflows/n8n/` (see [Workflows → Importing](./workflows.md#8-importing-workflows)).
3. Configure n8n credentials (AWS, GitHub) in the editor.
4. Manually execute the **Repository Ingestion** workflow with a test repo URL.
5. Confirm the content package appears in your dev generated-content bucket.

Iterate on the pipeline directly in the n8n editor; changes are exported back to `workflows/n8n/` and committed ([Workflows → Exporting](./workflows.md#8-importing-workflows)).

---

## 7. Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `AccessDeniedException` calling Bedrock | Model access not granted, or wrong Region | Request model access in the Bedrock console; confirm `AWS_REGION` |
| `ValidationException: model id` | Wrong/incomplete `BEDROCK_MODEL_ID` | Use an exact model ID or inference profile from `aws bedrock list-foundation-models` |
| n8n editor unreachable | Container not up / port clash | `docker compose ps`; check port `5678`; view `docker compose logs n8n` |
| `ThrottlingException` from Bedrock | Rate limits | Rely on backoff/retry; lower concurrency; request a quota increase |
| Go build fails for Lambda | Wrong target arch | Set `GOOS=linux GOARCH=arm64 CGO_ENABLED=0` |
| `NoCredentialProviders` | AWS creds not loaded | `aws sts get-caller-identity`; set `AWS_PROFILE` |
| n8n loses credentials on restart | `N8N_ENCRYPTION_KEY` changed | Keep the key stable; persist the `n8n_data` volume |
| Terraform state lock error | Interrupted run | `terraform force-unlock <lock-id>` (only if you own the lock) |

If you're stuck, open an issue with logs and reproduction steps — see [Contributing → Issue Reporting](./contributing.md#8-issue-reporting).
