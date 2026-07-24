# Packer template for the pre-baked worker AMI. It runs the same
# scripts/ami/provision.sh that the boot-time fallback would, so the AMI and the
# fallback never diverge. Build with scripts/build-ami.sh (recommended) or:
#   packer init packer/worker/ && packer build packer/worker/blog-gen.pkr.hcl
#
# Output: an AMI id to pass to the compute stack as the CustomAmi parameter.

packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

# Builder defaults match the CPU worker (t3.xlarge, no GPU) so the baked AMI is a
# faithful image of what actually runs. For a GPU worker, pass a GPU builder
# (e.g. g4dn.xlarge) AND enable_gpu=true so the driver bake is validated on real
# hardware.
variable "region" { default = "us-east-1" }
variable "instance_type" { default = "t3.xlarge" }  # match the worker; CPU-only, no driver bake
variable "ollama_model" { default = "qwen2.5:7b" }  # keep in sync with config.DefaultOllamaModel
variable "enable_gpu" { default = "false" }
variable "bake_model" { default = "true" }
variable "project" { default = "blog-gen" }

locals { ts = formatdate("YYYYMMDD-hhmmss", timestamp()) }

source "amazon-ebs" "blog_gen" {
  region          = var.region
  instance_type   = var.instance_type
  ssh_username    = "ubuntu"
  ami_name        = "${var.project}-worker-${local.ts}"
  ami_description = "Pre-baked GitHub AI Blog Generator worker host (Docker, Ollama + model, systemd services; NVIDIA when enable_gpu=true)."

  source_ami_filter {
    filters = {
      name                = "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    owners      = ["099720109477"] # Canonical
    most_recent = true
  }

  # Room for the driver, the ollama image, and the baked model seed (~5 GB).
  launch_block_device_mappings {
    device_name           = "/dev/sda1"
    volume_size           = 40
    volume_type           = "gp3"
    delete_on_termination = true
  }

  tags = {
    Project   = var.project
    Component = "worker-ami"
    Model     = var.ollama_model
    BuiltAt   = local.ts
  }
}

build {
  sources = ["source.amazon-ebs.blog_gen"]

  provisioner "file" {
    source      = "${path.root}/../../scripts/ami/provision.sh"
    destination = "/tmp/provision.sh"
  }

  provisioner "shell" {
    environment_vars = [
      "OLLAMA_MODEL=${var.ollama_model}",
      "ENABLE_GPU=${var.enable_gpu}",
      "BAKE_MODEL=${var.bake_model}",
    ]
    inline = [
      "chmod +x /tmp/provision.sh",
      "sudo -E /tmp/provision.sh",
    ]
  }

  # Machine-readable output so CI can extract the AMI id (region:ami-xxxx).
  post-processor "manifest" {
    output     = "packer-manifest.json"
    strip_path = true
  }
}
