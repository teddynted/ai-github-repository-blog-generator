# Local Development

This guide explains how to run and develop the **GitHub AI Blog Generator** on your machine. The whole AI stack — n8n, OpenClaw, and Ollama — runs locally in Docker Compose, so you can develop the pipeline without any cloud inference cost.

Related: [Deployment](./deployment.md) · [Workflows](./workflows.md) · [Contributing](./contributing.md).

---

## 1. Required Software

| Tool | Minimum version | Purpose |
| --- | --- | --- |
| [Git](https://git-scm.com/) | 2.30 | Clone the repo; the app itself clones target repos |
| [Docker](https://www.docker.com/) | 24 | Run n8n + OpenClaw + Ollama locally |
| [Docker Compose](https://docs.docker.com/compose/) | v2 | Local stack |
| [Ollama](https://ollama.com/) | latest | Local LLM server (can also run in Compose) |
| [AWS CLI](https://docs.aws.amazon.com/cli/) | v2 | Build/deploy CloudFormation; test SQS locally |
| [cfn-lint](https://github.com/aws-cloudformation/cfn-lint) | latest | Lint CloudFormation templates |
| [Go](https://go.dev/dl/) | 1.25+ | Build and test (the build toolchain is pinned in `go.mod`) |
| Make | any | Convenience targets (optional) |

Verify:

```bash
git --version && docker --version && docker compose version
aws --version && cfn-lint --version && go version
```

> A machine with a **GPU** is recommended for reasonable inference speed with larger Qwen models; smaller models run acceptably on CPU for development.

---

## 2. Docker Compose (local AI stack)

The `instance/` directory contains the same Docker Compose stack that runs on the EC2 host, so local and production topology match.

```bash
cp .env.example .env      # fill in values (see Section 4)
docker compose -f instance/docker-compose.yml up -d
docker compose -f instance/docker-compose.yml logs -f n8n
```

- **n8n** → http://localhost:5678
- **Ollama** → http://localhost:11434

Stop with `docker compose -f instance/docker-compose.yml down` (add `-v` to remove volumes).

Example `instance/docker-compose.yml` shape:

```yaml
services:
  n8n:
    image: n8nio/n8n:latest
    ports:
      - "5678:5678"
    environment:
      - N8N_ENCRYPTION_KEY=${N8N_ENCRYPTION_KEY}
      - OLLAMA_BASE_URL=http://ollama:11434
      - QUEUE_URL=${QUEUE_URL}
      - AWS_REGION=${AWS_REGION}
    volumes:
      - n8n_data:/home/node/.n8n
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_models:/root/.ollama
  openclaw:
    image: openclaw/openclaw:latest
    depends_on: [ollama]
volumes:
  n8n_data:
  ollama_models:
```

---

## 3. Pull the Local Model

Download the default Qwen model into Ollama once (it is then cached in the `ollama_models` volume — the local analogue of the persistent EBS volume in production):

```bash
docker compose -f instance/docker-compose.yml exec ollama ollama pull qwen2.5:7b
docker compose -f instance/docker-compose.yml exec ollama ollama list
```

Set `OLLAMA_MODEL` in `.env` to match. No external API key is needed — inference is entirely local.

---

## 4. Environment Configuration

Local configuration lives in `.env` (git-ignored). Start from `.env.example`:

| Variable | Description | Example |
| --- | --- | --- |
| `AWS_REGION` | Region for SQS testing | `us-east-1` |
| `AWS_PROFILE` | Local credentials profile | `blog-dev` |
| `OLLAMA_MODEL` | Local model to run | `qwen2.5:7b` |
| `N8N_ENCRYPTION_KEY` | n8n credential encryption key | random 32+ chars |
| `QUEUE_URL` | Dev SQS queue URL (optional) | `https://sqs…/blog-gen-dev-events` |
| `WEBHOOK_SECRET` | Secret for local HMAC tests | random 32+ chars |
| `PUBLISH_TRIGGER` | Commit-message trigger that gates generation | `blog:` |
| `GITHUB_TOKEN` | Optional; for cloning private test repos | `github_pat_xxx` |

> Secrets in `.env` are for **local dev only**. In AWS, secrets are provided via environment/secret configuration — see [Security](./security.md).

---

## 5. Building & Testing the Lambdas

The code is a **single Go module** (monorepo): shared logic lives under `internal/`, and each of the four functions — `registration` (validate + create webhook + store metadata/secret), `webhook-handler` (verify + trigger gate + publish), `instance-starter` (start the Spot host), and `idle-shutdown` (stop it) — is an entry point under `lambdas/<fn>/`. See [Development Plan](./development-plan.md).

```bash
make check          # gofmt check + go vet + go test -race -cover
```

> The handler's trigger logic is the highest-value unit to test: assert that `blog:` commits publish an event and that routine commits return `200` without publishing.

Build deployable artifacts (Linux, arm64) for every implemented function:

```bash
make build          # → dist/<fn>/bootstrap
```

---

## 6. Running End-to-End Locally

1. Start the stack (`docker compose -f instance/docker-compose.yml up -d`) and pull the model (§3).
2. Import the workflow JSON from `workflows/` (see [Workflows → Importing](./workflows.md#11-importing-workflows)).
3. Configure n8n credentials (SQS/AWS, GitHub) in the editor.
4. **Enqueue a test event** — post a fixture message representing a matched (`blog:`) event to your dev SQS queue, or run the **Event Ingestion** workflow with a sample payload directly.
5. Watch the pipeline: clone → analysis (OpenClaw) → Repository Memory → generation (Ollama) → quality review → (optional approval) → publish → notify.
6. Confirm Markdown output appears at your configured local destination.
7. Separately, exercise the handler's trigger gate against a `blog:` fixture and a routine-commit fixture (§5) to confirm only the former publishes.

Iterate directly in the n8n editor; export changes back to `workflows/` and commit them.

---

## 7. Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| n8n editor unreachable | Container not up / port clash | `docker compose ps`; check port `5678`; view logs |
| Ollama returns model-not-found | Model not pulled | `ollama pull $OLLAMA_MODEL` (§3) |
| Inference very slow | Running a large model on CPU | Use a smaller Qwen model, or a GPU host |
| n8n loses credentials on restart | `N8N_ENCRYPTION_KEY` changed | Keep the key stable; persist the `n8n_data` volume |
| Go build fails for Lambda | Wrong target arch | Set `GOOS=linux GOARCH=arm64 CGO_ENABLED=0` |
| `NoCredentialProviders` (SQS test) | AWS creds not loaded | `aws sts get-caller-identity`; set `AWS_PROFILE` |
| HMAC test rejected (401) | `WEBHOOK_SECRET` mismatch | Ensure the signer and handler use the same secret |
| Stack stuck `UPDATE_IN_PROGRESS` / rollback | Interrupted or failed deploy | Inspect `aws cloudformation describe-stack-events`; wait for auto-rollback |

If you're stuck, open an issue with logs and reproduction steps — see [Contributing → Issue Reporting](./contributing.md#8-issue-reporting).
