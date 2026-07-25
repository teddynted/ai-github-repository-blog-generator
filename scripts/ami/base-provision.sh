#!/usr/bin/env bash
# base-provision.sh — install the shared AI Platform Base software stack.
#
# This is the project-AGNOSTIC half of the golden image: everything common to AI
# workloads, pre-installed so downstream instances boot fast and consistently.
# It installs NOTHING project-specific (no model, no app binary) — those are
# layered by the consuming project's UserData or a child AMI.
#
# Baked here (slow, network-heavy — done once at build time):
#   - Docker Engine + compose plugin
#   - Ollama (CLI/runtime; models are pulled per-project at boot, not baked)
#   - FFmpeg, Git, Python 3 + venv/pip, build-essential
#   - Go toolchain (GO_VERSION)
#   - Amazon CloudWatch Agent + AWS Systems Manager (SSM) Agent
#   - a versioned image manifest at /etc/ai-platform/ami-metadata.json
#   - CloudWatch Agent + log configuration
#
# Env: AMI_VERSION, GO_VERSION (default 1.23.4), GIT_COMMIT, RELEASE.
set -euxo pipefail

export DEBIAN_FRONTEND=noninteractive
GO_VERSION="${GO_VERSION:-1.23.4}"
AMI_VERSION="${AMI_VERSION:-v0.0.0-dev}"
GIT_COMMIT="${GIT_COMMIT:-unknown}"
RELEASE="${RELEASE:-unreleased}"
ARCH="$(dpkg --print-architecture)" # amd64

apt-get update -y
apt-get upgrade -y

# --- Base tooling ---------------------------------------------------------
apt-get install -y \
  ca-certificates curl gnupg jq unzip git ffmpeg \
  python3 python3-pip python3-venv build-essential \
  auditd apparmor apparmor-utils unattended-upgrades

# --- AWS CLI v2 (project-agnostic; used by publishing + UserData) ---------
tmp="$(mktemp -d)"
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o "$tmp/awscliv2.zip"
unzip -q "$tmp/awscliv2.zip" -d "$tmp"
"$tmp/aws/install" --update
rm -rf "$tmp"

# --- Docker Engine --------------------------------------------------------
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu jammy stable" \
  >/etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable --now docker

# --- Go toolchain ---------------------------------------------------------
goarch="amd64"; [ "$ARCH" = "arm64" ] && goarch="arm64"
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${goarch}.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz && rm /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' >/etc/profile.d/go.sh
chmod 0644 /etc/profile.d/go.sh

# --- Ollama runtime (models pulled per-project at boot, never baked) ------
curl -fsSL https://ollama.com/install.sh | sh
systemctl enable ollama || true

# --- Amazon CloudWatch Agent ---------------------------------------------
curl -fsSL "https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/${ARCH}/latest/amazon-cloudwatch-agent.deb" \
  -o /tmp/cwagent.deb
dpkg -i -E /tmp/cwagent.deb && rm /tmp/cwagent.deb

install -d /opt/aws/amazon-cloudwatch-agent/etc
cat >/opt/aws/amazon-cloudwatch-agent/etc/base-config.json <<'JSON'
{
  "agent": { "metrics_collection_interval": 60, "run_as_user": "root" },
  "metrics": {
    "namespace": "AIPlatform/Base",
    "append_dimensions": { "InstanceId": "${aws:InstanceId}" },
    "metrics_collected": {
      "cpu": { "measurement": ["cpu_usage_idle", "cpu_usage_iowait"], "totalcpu": true },
      "mem": { "measurement": ["mem_used_percent"] },
      "disk": { "measurement": ["used_percent"], "resources": ["/"] }
    }
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          { "file_path": "/var/log/cloud-init-output.log", "log_group_name": "/ai-platform/base/cloud-init", "log_stream_name": "{instance_id}" },
          { "file_path": "/var/log/ai-platform/startup.log", "log_group_name": "/ai-platform/base/startup", "log_stream_name": "{instance_id}" }
        ]
      }
    }
  }
}
JSON
# The agent is enabled but not started at bake time — the consuming project points
# it at this base config (or overlays its own) from UserData, then starts it.
systemctl enable amazon-cloudwatch-agent || true

# --- SSM Agent (present on Ubuntu AMIs via snap) --------------------------
# Installed so Session Manager access is available if the account configures an
# SSM instance-management role, but left DISABLED: without that role the agent
# only logs an AccessDenied every ~25m and this platform drives the instance via
# SQS/S3/Secrets, not SSM. Enable it (and the account role) if you want a shell.
snap install amazon-ssm-agent --classic 2>/dev/null || true
snap stop --disable amazon-ssm-agent 2>/dev/null || true
systemctl disable --now snap.amazon-ssm-agent.amazon-ssm-agent.service 2>/dev/null || true

# --- Log directory + first-boot startup hook ------------------------------
install -d -m 0755 /var/log/ai-platform
install -d -m 0755 /etc/ai-platform /opt/ai-platform

# A generic, idempotent first-boot hook downstream projects can extend. It logs
# to the CloudWatch-collected startup.log and records readiness.
cat >/opt/ai-platform/startup.sh <<'SH'
#!/usr/bin/env bash
# Generic base startup hook. Consuming projects append their own steps or drop
# files in /opt/ai-platform/startup.d/*.sh (run in lexical order).
set -euo pipefail
LOG=/var/log/ai-platform/startup.log
exec >>"$LOG" 2>&1
echo "[$(date -u +%FT%TZ)] ai-platform base startup"
systemctl is-active --quiet docker && echo "docker: up" || echo "docker: DOWN"
if [ -d /opt/ai-platform/startup.d ]; then
  for f in /opt/ai-platform/startup.d/*.sh; do
    [ -e "$f" ] || continue
    echo "[$(date -u +%FT%TZ)] running $f"
    bash "$f" || echo "startup hook $f failed"
  done
fi
echo "[$(date -u +%FT%TZ)] base startup complete"
SH
chmod 0755 /opt/ai-platform/startup.sh
install -d -m 0755 /opt/ai-platform/startup.d

cat >/etc/systemd/system/ai-platform-startup.service <<'UNIT'
[Unit]
Description=AI Platform base first-boot startup hook
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/ai-platform/startup.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable ai-platform-startup.service

# --- Versioned image manifest (queryable on any running instance) ---------
cat >/etc/ai-platform/ami-metadata.json <<JSON
{
  "schemaVersion": "1.0.0",
  "name": "ai-platform-base",
  "version": "${AMI_VERSION}",
  "baseOS": "ubuntu-22.04",
  "architecture": "x86_64",
  "gitCommit": "${GIT_COMMIT}",
  "release": "${RELEASE}",
  "goVersion": "${GO_VERSION}",
  "components": ["docker", "ollama", "ffmpeg", "git", "go", "python3", "cloudwatch-agent", "ssm-agent"],
  "hardened": true,
  "owner": "ai-github-repository-blog-generator"
}
JSON
chmod 0644 /etc/ai-platform/ami-metadata.json

echo "base-provision complete: ai-platform-base ${AMI_VERSION}"
