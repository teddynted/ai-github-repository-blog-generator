# AI Platform Base AMI — the shared, project-agnostic golden image (Milestone 20).
#
# This bakes a hardened Ubuntu 22.04 image with the software every AI workload in
# the fleet needs (Docker, Ollama, FFmpeg, Git, Go, Python, the CloudWatch + SSM
# agents) but NOTHING project-specific — no model, no application binary. It is
# consumed by downstream projects (designing-an-ai-agent-platform-on-aws,
# football-match-prediction-platform, trading-bot, …) via the SSM Parameter Store
# / CloudFormation exports published by scripts/ami/publish-metadata.sh.
#
# Build (semantic version is REQUIRED — it tags the AMI and drives publishing):
#   scripts/build-base-ami.sh --version v1.0.0
# or directly:
#   packer init packer/base/
#   packer build -var 'version=v1.0.0' packer/base/ai-platform-base.pkr.hcl
#
# Output: packer-manifest.json (region:ami-id) which build-base-ami.sh publishes.

packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "region" { default = "us-east-1" }
variable "instance_type" { default = "t3.large" } # CPU build box; the base image is GPU-agnostic
variable "version" { default = "v0.0.0-dev" }      # SemVer — REQUIRED for real builds
variable "git_commit" { default = "unknown" }      # populated by build-base-ami.sh
variable "release" { default = "unreleased" }       # repository release tag
variable "go_version" { default = "1.23.4" }
variable "base_os" { default = "ubuntu-22.04" }

locals {
  ts    = formatdate("YYYYMMDD-hhmmss", timestamp())
  owner = "ai-github-repository-blog-generator"
}

source "amazon-ebs" "ai_platform_base" {
  region        = var.region
  instance_type = var.instance_type
  ssh_username  = "ubuntu"
  # Name embeds the version + timestamp so images are self-describing and sortable.
  ami_name        = "ai-platform-base-${var.version}-${local.ts}"
  ami_description = "AI Platform Base AMI ${var.version} — hardened Ubuntu 22.04 with Docker, Ollama, FFmpeg, Git, Go, Python, CloudWatch + SSM agents. Project-agnostic golden image."

  source_ami_filter {
    filters = {
      name                = "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    owners      = ["099720109477"] # Canonical
    most_recent = true
  }

  # Enforce IMDSv2 on the BUILD instance too (defense in depth during baking).
  imds_support = "v2.0"
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = 30
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  # Snapshots inherit these tags so lifecycle cleanup can find them by version.
  snapshot_tags = {
    Name      = "ai-platform-base-${var.version}"
    Project   = "ai-platform"
    Component = "base-ami-snapshot"
    Version   = var.version
  }

  # Rich, queryable metadata — the source of truth for publishing + lifecycle.
  tags = {
    Name         = "ai-platform-base-${var.version}"
    Project      = "ai-platform"
    Component    = "base-ami"
    Role         = "golden-base"
    Version      = var.version
    BaseOS       = var.base_os
    Architecture = "x86_64"
    GitCommit    = var.git_commit
    Release      = var.release
    Owner        = local.owner
    Lifecycle    = "current"
    BuildDate    = local.ts
    Hardened     = "true"
    ManagedBy    = "packer"
  }
}

build {
  name    = "ai-platform-base"
  sources = ["source.amazon-ebs.ai_platform_base"]

  # Provision the base software stack, then harden the OS.
  provisioner "file" {
    source      = "${path.root}/../../scripts/ami/base-provision.sh"
    destination = "/tmp/base-provision.sh"
  }
  provisioner "file" {
    source      = "${path.root}/../../scripts/ami/harden.sh"
    destination = "/tmp/harden.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "AMI_VERSION=${var.version}",
      "GO_VERSION=${var.go_version}",
      "GIT_COMMIT=${var.git_commit}",
      "RELEASE=${var.release}",
    ]
    inline = [
      "chmod +x /tmp/base-provision.sh /tmp/harden.sh",
      "sudo -E /tmp/base-provision.sh",
      "sudo -E /tmp/harden.sh",
    ]
  }

  # Machine-readable output so build-base-ami.sh can extract region:ami-id.
  post-processor "manifest" {
    output     = "packer-manifest.json"
    strip_path = true
    custom_data = {
      version    = var.version
      git_commit = var.git_commit
      release    = var.release
      base_os    = var.base_os
    }
  }
}
