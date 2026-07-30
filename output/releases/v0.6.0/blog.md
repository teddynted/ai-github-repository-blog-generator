---
title: "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform"
description: "How an event-driven AI agent platform on AWS trades boot-time UserData provisioning for versioned, pre-baked custom AMIs to make on-demand EC2 startup fast…"
tags: [aws-iam, aws-lambda, amazon-cloudwatch, amazon-ec2, amazon-eventbridge, amazon-eventbridge-scheduler, amazon-s3, github-actions]
---

# Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform

## Why This Matters

The platform brings its compute host up only when work arrives, and every start paid the same tax: a full provisioning pass ran on the boot path. Each launch updated the operating system and installed Docker, Go, Node, Python, the CloudWatch agent, and the drain agent from a stock AL2023 image before the host could do anything useful. That work was slow, and its outcome depended on live package repositories, so two starts could produce two slightly different machines. Because the serverless front door stays always available while compute runs on-demand, this startup latency is felt directly on every launch — not amortised across a long-lived fleet.

## The Existing Architecture

The runtime is an event-driven, AWS-native pipeline. GitHub reaches the platform two ways: webhooks and API calls flow through Amazon EventBridge to AWS Lambda, while commits and pull requests reach `n8n`. Lambda invokes the `claw` router, which orchestrates `n8n`, runs primary inference on `ollama` with Amazon Bedrock as fallback, uses Amazon EFS as its workspace, and writes artifacts to Amazon S3. AWS IAM scopes permissions to Lambda and `claw`, and `claw`, `n8n`, `ollama`, and Lambda all emit logs and metrics to Amazon CloudWatch.

A separate control plane governs cost and availability: Amazon EventBridge Scheduler triggers Lambda to start and stop the Amazon EC2 host. This bounds spend while keeping ingestion live. The limitation surfaced at the moment of start — the host that Lambda brings up had to provision itself before it was useful.

## The Engineering Constraint

- Boot-time UserData reprovisioned the machine from a stock AL2023 image on every start, running `dnf update` and installing the full toolchain each time.
- Startup latency was therefore high and, worse, variable — outcomes depended on package repositories resolved at boot rather than a frozen input.
- The drain agent was carried as configuration, leaving room for the running host to drift from the intended image without an obvious signal.
- The cost/availability posture had to be preserved: serverless ingestion always on, compute only on-demand. Any fix could not turn the on-demand host into a standing one.
- Provisioning and configuration were entangled in one UserData script, so a start could not separate one-time machine setup from per-boot settings.

## The Solution

The fix moves provisioning off the boot path and into a pre-baked custom AMI. A build pipeline produces a versioned image that bakes the full toolchain, the CloudWatch agent, and the drain agent — the last as code rather than configuration. The runtime host boots from that AMI, and its UserData collapses to configuration only.

Immutability is enforced at build time. The pipeline resolves the next version (for example `1.0.0 → 1.0.1`) and refuses to build if that version already exists. Once the image is created, both the AMI and its backing snapshot are tagged with `Project`, `Component`, and `Version`, so every image is traceable and lifecycle-manageable.

A drift check guards the drain agent: because it is now baked into the image, a check confirms the running agent matches the baked one, so runtime configuration cannot silently diverge from the image. None of the runtime platform components change — only how the compute host is provisioned.

## Architecture Summary

| Concern | Implementation |
| --- | --- |
| Machine provisioning | Baked once into a versioned custom AMI (toolchain, CloudWatch agent, drain agent) |
| Boot-path work | UserData reduced to configuration only |
| Image immutability | Resolve next version; refuse to overwrite an existing version |
| Traceability | AMI and snapshot tagged with `Project`, `Component`, `Version` |
| Build orchestration | Dev builder + Amazon S3 scripts + AWS Systems Manager status polling |
| Success signal | `wait instance-stopped` before `create-image` |
| Runtime start/stop | Amazon EventBridge Scheduler → AWS Lambda → Amazon EC2 |
| Drain agent integrity | Baked as code, verified by a drift check |

The build path and the runtime path are distinct planes. The build plane produces a tagged, versioned AMI; the runtime plane starts an EC2 host from it. The following comparison makes the core shift explicit.

| Dimension | Boot-time UserData | Pre-baked custom AMI |
| --- | --- | --- |
| Provisioning timing | Every boot | Once at build time |
| Startup latency | High, variable | Low, deterministic |
| Reproducibility | Depends on live repos | Frozen, versioned image |
| Drain agent | Config (drift-prone) | Baked code + drift check |
| UserData role | Full provisioning | Configuration only |
| Added overhead | None up front | AMI/snapshot storage + build pipeline |

## Architecture Diagram

```mermaid
sequenceDiagram
    participant Dev
    participant S3 as Amazon S3
    participant EC2 as Amazon EC2 (builder)
    participant SSM as AWS Systems Manager
    Dev->>Dev: resolve next version (1.0.0 → 1.0.1); refuse if it exists
    Dev->>S3: upload provision.sh, cleanup.sh, drain agent
    Dev->>EC2: run-instances (stock AL2023)
    EC2->>S3: fetch the scripts
    EC2->>EC2: dnf update; install docker/go/node/python/CW agent
    EC2->>EC2: install drain agent; write /etc/ami-manifest.json; touch ami-build.done
    Dev->>SSM: done? failed? where are you?
    SSM->>Dev: last line of the build log
    Dev->>SSM: run cleanup.sh (creds, keys, machine-id, SSM, cloud-init clean)
    Dev->>EC2: wait instance-stopped (the real success signal)
    Dev->>EC2: create-image; tag AMI + snapshot; terminate builder
```

The build plane runs out of band. Dev resolves and guards the version, stages scripts in Amazon S3, and launches a stock builder. The builder provisions itself, writes a manifest, and signals completion, while Dev polls AWS Systems Manager for status. After cleanup and a confirmed stop, Dev captures the image and tears the builder down. At runtime, Amazon EventBridge Scheduler triggers AWS Lambda, which starts an EC2 host from that AMI; UserData applies configuration only, and the host joins the `claw` inference and CloudWatch platform unchanged.

## Key Implementation Details

### Version resolution and immutability

The pipeline computes the next semantic version from existing images and refuses to build when that version already exists. Immutability is a property of construction, not policy: an existing `Version` cannot be overwritten. After capture, both the AMI and its backing snapshot are tagged with `Project`, `Component`, and `Version`, tying each image to the platform component it serves and making lifecycle queries — find, retain, retire — straightforward.

### The builder and its completion signal

The builder starts from stock AL2023, fetches `provision.sh`, `cleanup.sh`, and the drain agent from Amazon S3, runs `dnf update`, and installs Docker, Go, Node, Python, and the CloudWatch agent. It writes `/etc/ami-manifest.json` to record what the image contains, then touches `ami-build.done` to signal completion. The drain agent is installed as code here, not injected later as configuration.

### Out-of-band status via Systems Manager

Build progress is inspected through AWS Systems Manager rather than a network service on the builder. Dev asks whether the build is done, failed, or stuck, and SSM returns the last line of the build log. This keeps the builder free of inbound access while still giving the pipeline a live view of a long-running provisioning step.

### Cleanup and quiesced capture

Cleanup strips credentials, SSH keys, `machine-id`, and SSM registration, then runs `cloud-init clean` — without that, UserData would never run again on instances launched from the image. The builder then runs `shutdown -h now`. The pipeline treats `wait instance-stopped` as the real success signal and only then calls `create-image`, so the AMI is captured from a quiesced filesystem. The builder is terminated on every exit path.

## Why These Decisions Were Made

### Refuse duplicate versions, tag every image

A naming convention alone would let a rebuild silently replace an image with the same version. Refusing duplicates makes each `Version` a permanent, distinct artifact, and tagging the AMI and snapshot makes the fleet auditable. The cost is a build that stops rather than proceeds, which is the intended behaviour for immutable images.

### `cloud-init clean` before capture

Reducing UserData to configuration only is worthless if it never executes. Leaving `cloud-init` state in the image would mark it as already-run, so later boots would skip the config-only UserData. Running `cloud-init clean` before capture guarantees the reduced UserData runs on every instance launched from the image.

### `instance-stopped`, not build-done alone

`ami-build.done` proves the script finished; it does not prove the filesystem is consistent. Capturing while processes still flush would bake a dirty image. Waiting for `instance-stopped` after `shutdown -h now` guarantees a quiesced filesystem, so the resulting AMI is a clean, repeatable starting point rather than a snapshot of an in-flight machine.

## Repository Impact

The work concentrates in infrastructure and documentation, with no application feature or fix code — a maintenance and hardening change. Infrastructure remains defined as CloudFormation across the platform's templates, spanning EC2, S3, Lambda, Events, Scheduler, IAM, and Logs. The infrastructure changes add the versioned AMI build, switch the compute host to boot from the custom AMI, collapse UserData to configuration, and introduce the AMI lifecycle plus the drain-agent drift check.

Documentation gains a hand-authored "as built" SVG of the platform and a living "as built" diagram, with the remaining diagrams marked as point-in-time snapshots. A dedicated sequence diagram documents the AMI bake pipeline end to end, from version resolution through builder teardown.

## Benefits

- Faster, more deterministic instance startup by removing per-boot provisioning from the critical path.
- Immutable, versioned images that are traceable through `Project`/`Component`/`Version` tags and reproducible from a frozen input.
- The drain agent baked as code, plus a drift check, reduces configuration divergence between the running host and its image.
- The cost/availability posture is preserved: compute stays on-demand while serverless ingestion stays always available.

## Tradeoffs

- The AMI build pipeline is a new thing to own and operate: builder lifecycle, Systems Manager polling, cleanup, and guaranteed teardown on every exit path.
- Baked toolchains (`dnf` packages, Docker, Go, Node, Python, the CloudWatch agent) go stale, so images must be periodically rebuilt and re-versioned to stay current.
- Stored AMIs and their backing snapshots carry ongoing storage cost and version-management overhead that boot-time provisioning did not.

## What This Enables Next

A host that boots fast and deterministically from a frozen image is the prerequisite for using interruptible capacity well. When a Spot instance can be reclaimed at short notice, startup time is no longer a fixed penalty paid on every launch, so the platform can absorb reclaim-and-replace cycles without the compute host becoming a bottleneck. The baked drain agent already sits in the image to handle graceful shutdown. Together, the pre-baked AMI and the drain agent form the foundation the Spot-startup optimisation is built on.

## Conclusion

The reusable pattern here is the golden AMI: push slow, repeated provisioning off the boot path into an immutable, versioned image, and reduce boot-time work to configuration only. The discipline that makes it safe is specific — resolve and guard the version so images cannot be overwritten, tag the AMI and its snapshot for lifecycle tracking, treat `instance-stopped` after cleanup and `cloud-init clean` as the real success signal, and bake agents as code with a drift check rather than shipping them as mutable configuration. Applied to this platform, that shift turned a slow, variable UserData install into a fast, repeatable start without disturbing the event-driven pipeline around it. Engineers building on-demand or Spot compute can take the same approach: separate what a machine *is* from how it is *configured*, freeze the former into a versioned image, and let each start do only the small, deterministic remainder.
