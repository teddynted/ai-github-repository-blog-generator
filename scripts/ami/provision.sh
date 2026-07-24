#!/usr/bin/env bash
# provision.sh — the single source of truth for everything the instance needs
# pre-installed. Packer runs this to BAKE the custom AMI; UserData fetches and
# runs it only as a fallback on a stock AMI. Keeping one script means the AMI and
# the fallback never drift.
#
# Baked here (slow, network-heavy — done once at build time):
#   - Docker Engine
#   - NVIDIA driver + container toolkit (GPU passthrough)   [ENABLE_GPU=true]
#   - the ollama/ollama image
#   - the LLM model, as a seed copied to /data at first boot [BAKE_MODEL=true]
#   - the systemd units + helper scripts (auto-start, health, GPU detection)
#
# Left for boot (fast, instance-specific): mount /data, seed the model, write the
# worker env from stack params, drop in the worker binary, start the services.
#
# Env: OLLAMA_MODEL (default qwen2.5:7b), ENABLE_GPU (true), BAKE_MODEL (true).
set -euxo pipefail

OLLAMA_MODEL="${OLLAMA_MODEL:-qwen2.5:7b}"
ENABLE_GPU="${ENABLE_GPU:-true}"
BAKE_MODEL="${BAKE_MODEL:-true}"
export DEBIAN_FRONTEND=noninteractive

mkdir -p /opt/blog-gen

# --- Docker + base tools -------------------------------------------------
# A freshly-booted AMI can hit a transient apt/mirror skew (e.g. jq's dep
# libonig5 momentarily "not installable"); retry once after re-updating.
apt-get update -y
install_base() { apt-get install -y ca-certificates curl gnupg unzip jq; }
install_base || { sleep 5; apt-get update -y --fix-missing; install_base; }

# AWS CLI v2 — the `awscli` apt package was dropped on Ubuntu 22.04, so install
# the official bundle. The instance uses it at boot (aws s3 cp worker binary, ssm).
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o /tmp/awscliv2.zip
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install --update
rm -rf /tmp/aws /tmp/awscliv2.zip

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
	> /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable --now docker

# --- NVIDIA driver + container toolkit (GPU) -----------------------------
if [ "$ENABLE_GPU" = "true" ]; then
	apt-get install -y "linux-headers-$(uname -r)" nvidia-driver-535-server || true
	curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
		| gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
	curl -fsSL https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
		| sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
		> /etc/apt/sources.list.d/nvidia-container-toolkit.list
	apt-get update -y
	apt-get install -y nvidia-container-toolkit
	nvidia-ctk runtime configure --runtime=docker
	systemctl restart docker
fi

# --- Ollama image + baked model seed -------------------------------------
docker pull ollama/ollama:latest
if [ "$BAKE_MODEL" = "true" ]; then
	# Pull the model once into a seed dir baked into the image; first boot copies
	# it to the persistent volume, so no multi-GB download happens at launch.
	mkdir -p /opt/blog-gen/ollama-seed
	docker rm -f ollama-seed 2>/dev/null || true
	docker run -d --name ollama-seed -v /opt/blog-gen/ollama-seed:/root/.ollama ollama/ollama:latest
	for _ in $(seq 1 30); do
		docker exec ollama-seed ollama --version >/dev/null 2>&1 && break || sleep 2
	done
	docker exec ollama-seed ollama pull "$OLLAMA_MODEL"
	docker rm -f ollama-seed
fi

# --- Helper scripts + systemd units --------------------------------------
install -d /opt/blog-gen

# start-ollama.sh detects the GPU at runtime, so one AMI runs on GPU or CPU.
cat > /opt/blog-gen/start-ollama.sh <<'EOF'
#!/usr/bin/env bash
set -e
docker rm -f ollama 2>/dev/null || true
GPU=""
if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
	GPU="--gpus all"
fi
# shellcheck disable=SC2086
exec docker run --rm --name ollama $GPU \
	-p 127.0.0.1:11434:11434 -v /data/ollama:/root/.ollama ollama/ollama:latest
EOF

# wait-ollama.sh gates the worker until the API answers AND the model is loaded.
cat > /opt/blog-gen/wait-ollama.sh <<'EOF'
#!/usr/bin/env bash
[ -f /etc/blog-gen/worker.env ] && . /etc/blog-gen/worker.env
MODEL="${OLLAMA_MODEL:-qwen2.5:7b}"
for _ in $(seq 1 120); do
	if curl -sf http://127.0.0.1:11434/api/tags 2>/dev/null | grep -q "${MODEL%%:*}"; then
		exit 0
	fi
	sleep 5
done
exit 0
EOF

# health.sh: single readiness check across every service the worker needs.
cat > /opt/blog-gen/health.sh <<'EOF'
#!/usr/bin/env bash
# Exit 0 only when Docker, Ollama (with the model), and the worker are all up.
fail() { echo "UNHEALTHY: $1"; exit 1; }
systemctl is-active --quiet docker || fail "docker not active"
docker ps --format '{{.Names}}' | grep -qx ollama || fail "ollama container not running"
[ -f /etc/blog-gen/worker.env ] && . /etc/blog-gen/worker.env
MODEL="${OLLAMA_MODEL:-qwen2.5:7b}"
curl -sf http://127.0.0.1:11434/api/tags 2>/dev/null | grep -q "${MODEL%%:*}" || fail "model $MODEL not loaded"
systemctl is-active --quiet blog-gen-worker || fail "worker not active"
echo "READY"
EOF
chmod +x /opt/blog-gen/start-ollama.sh /opt/blog-gen/wait-ollama.sh /opt/blog-gen/health.sh

# Ollama service — waits for /data (the model volume) and auto-restarts.
cat > /etc/systemd/system/blog-gen-ollama.service <<'EOF'
[Unit]
Description=Ollama local LLM inference
After=docker.service network-online.target
Requires=docker.service
RequiresMountsFor=/data

[Service]
Type=simple
ExecStart=/opt/blog-gen/start-ollama.sh
ExecStop=/usr/bin/docker stop ollama
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Worker service — starts only after Ollama is serving the model.
cat > /etc/systemd/system/blog-gen-worker.service <<'EOF'
[Unit]
Description=GitHub AI Blog Generator worker
After=blog-gen-ollama.service network-online.target
Requires=blog-gen-ollama.service

[Service]
EnvironmentFile=/etc/blog-gen/worker.env
ExecStartPre=/opt/blog-gen/wait-ollama.sh
ExecStart=/usr/local/bin/blog-gen-worker
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
apt-get clean
rm -rf /var/lib/apt/lists/*

# Marker used by UserData to skip provisioning on a baked AMI.
date -u +"%Y-%m-%dT%H:%M:%SZ" > /opt/blog-gen/.ami-baked
echo "provision complete (model=$OLLAMA_MODEL gpu=$ENABLE_GPU baked_model=$BAKE_MODEL)"
