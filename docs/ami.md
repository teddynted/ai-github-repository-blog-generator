# Custom AMI — Build & Update

The worker instance boots from a **pre-baked AMI** so a Spot launch is ready to
process jobs in well under a minute instead of the ~10–15 min a stock image
needs to install NVIDIA drivers, Docker, the Ollama image, and pull the model.

## What's baked vs. what happens at boot

| Baked into the AMI (once, at build) | Done at boot (fast, instance-specific) |
| --- | --- |
| Docker Engine | Mount the persistent `/data` volume |
| NVIDIA driver + container toolkit | Seed the model onto `/data` (copy, no download) |
| `ollama/ollama` image | Write `/etc/blog-gen/worker.env` from stack params |
| The LLM model (as a seed) | Download the small worker binary from S3 |
| systemd units + health/GPU scripts | `systemctl enable --now` the two services |

Everything baked lives in **one script — [`scripts/ami/provision.sh`](../scripts/ami/provision.sh)** — which Packer runs at build time and which a stock AMI runs at boot as a fallback. One source, so the two paths never drift.

## Prerequisites

- [Packer](https://developer.hashicorp.com/packer/install) ≥ 1.9
- AWS credentials with EC2/AMI build permissions
- Enough EBS/GPU quota to launch the build instance (`g4dn.xlarge` by default)

## Build (or rebuild)

**In CI (recommended):** Actions → **build-ami** → *Run workflow* (inputs: region,
model, GPU, bake-model). It runs Packer on a hosted runner using the deploy OIDC
role and **writes the new AMI id to SSM** (`/blog-gen/worker-ami`). Then just
redeploy `blog-gen-compute` (merge to main / run `deploy.yml`) — deploy reads the
AMI id from SSM automatically. **No variable to set.**

**Locally:**

```bash
scripts/build-ami.sh --region us-east-1            # defaults: model qwen2.5:7b, GPU on, model baked
# options:
scripts/build-ami.sh --model llama3.1:8b           # different model
scripts/build-ami.sh --no-bake-model               # smaller AMI; model pulls once at first boot
scripts/build-ami.sh --no-gpu                       # CPU-only image (cheaper test instances)
```

Packer prints the new **AMI id** at the end. Wire it in:

- **CI (automatic):** the `build-ami` workflow already saves the id to SSM `/blog-gen/worker-ami`; `deploy.yml` reads it on the next run. Nothing to set. (Building locally instead? Save it yourself: `aws ssm put-parameter --name /blog-gen/worker-ami --type String --value <ami-id> --overwrite`.)
- **Pin a specific image:** set the repository variable `CUSTOM_AMI=<ami-id>` — it overrides the SSM value.
- **Manual deploy:** add `CustomAmi=<ami-id>` to the compute stack's `--parameter-overrides`.

Then redeploy `blog-gen-compute`. New Spot instances launch from the baked AMI; the launch template picks up the new image on the next instance replacement.

## When to rebuild

Rebuild whenever the **runtime dependencies** change — not on every app change:

- the model (`OllamaModel`) changes
- Docker / NVIDIA / Ollama versions you want pinned change
- `scripts/ami/provision.sh` changes

The **worker binary is *not* baked** — it changes every deploy and is downloaded
from S3 at boot (a few MB). So ordinary code changes need no AMI rebuild.

## Reproducibility

The AMI is defined entirely in code: [`packer/blog-gen.pkr.hcl`](../packer/blog-gen.pkr.hcl) (the image) + `scripts/ami/provision.sh` (its contents). Rebuilding is a single command and always reflects the current repo. Keep the Packer `ollama_model` default in sync with `config.DefaultOllamaModel`.

## Operational considerations

- **AMI size / cost.** Baking the model adds ~5 GB to the image (EBS snapshot storage cost — cents/month). Use `--no-bake-model` to trade image size for a one-time model download on first boot.
- **GPU portability.** The image runs on GPU *or* CPU: `start-ollama.sh` detects the GPU at runtime and adds `--gpus all` only when present. So a GPU-baked AMI still boots on a CPU instance (slower inference).
- **Fallback safety.** If `CustomAmi` is unset, the stock Ubuntu AMI still works — it fetches and runs `provision.sh` at boot. Slower, but nothing breaks.
- **Region-scoped.** AMIs are per-region; rebuild (or copy) per region you deploy to.
- **Health.** `/opt/blog-gen/health.sh` reports readiness (Docker + Ollama + model + worker). The worker's `ExecStartPre` waits for the model before it pulls any job.

See the README → **Optimizing Spot Instance Startup** for the end-to-end workflow and expected timings.
