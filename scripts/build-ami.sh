#!/usr/bin/env bash
# build-ami.sh — build (or rebuild) the pre-baked worker AMI with Packer.
#
#   scripts/build-ami.sh [--region us-east-1] [--model qwen2.5:7b] \
#                        [--no-gpu] [--no-bake-model]
#
# Rebuild whenever the runtime dependencies change (Docker/NVIDIA/Ollama versions
# or the model). The AMI id it prints goes to the compute stack as CustomAmi.
set -euo pipefail

HERE="$(cd "$(dirname "$0")/.." && pwd)"
REGION="us-east-1"
MODEL="qwen2.5:7b"
ENABLE_GPU="true"
BAKE_MODEL="true"

while [ $# -gt 0 ]; do
	case "$1" in
		--region) REGION="$2"; shift 2 ;;
		--model) MODEL="$2"; shift 2 ;;
		--no-gpu) ENABLE_GPU="false"; shift ;;
		--no-bake-model) BAKE_MODEL="false"; shift ;;
		-h|--help) sed -n '2,10p' "$0"; exit 0 ;;
		*) echo "unknown argument: $1" >&2; exit 1 ;;
	esac
done

command -v packer >/dev/null 2>&1 || {
	echo "packer not found — install from https://developer.hashicorp.com/packer/install" >&2
	exit 1
}

cd "$HERE"
packer init packer/
echo "Building AMI (region=$REGION model=$MODEL gpu=$ENABLE_GPU bake_model=$BAKE_MODEL)…"
packer build \
	-var "region=$REGION" \
	-var "ollama_model=$MODEL" \
	-var "enable_gpu=$ENABLE_GPU" \
	-var "bake_model=$BAKE_MODEL" \
	packer/blog-gen.pkr.hcl

echo
echo "Done. Set the printed AMI id as the compute stack's CustomAmi parameter"
echo "(deploy.yml: repository variable CUSTOM_AMI), then redeploy blog-gen-compute."
