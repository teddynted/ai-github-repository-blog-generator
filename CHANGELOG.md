# Changelog

All notable changes to this project are documented here.
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
and [Conventional Commits](https://www.conventionalcommits.org/).

<!--
This file is maintained by the release CLI (cmd/release). Each `release` run
inserts a new version section below this header, newest first. See
docs/releases.md. Do not edit released sections by hand.
-->

## [0.21.2] - 2026-07-28

### Features

- **blog:** concise 13-section publication format as cloud standard (blog@12)
- **blog:** encode editorial standard into the generator prompt (blog@11)
- **blog:** feed the repository architecture graph into blog grounding
- **blog:** reflection retry — feed validation errors back into the rewrite
- **blog:** regenerate the article on validation failure (Option A)
- **blog:** repository-first — document THIS repo, never a generic AWS article
- **blog:** staff-engineer voice — kill AI tells in the blog prompt
- **blog:** terminology/clarity quality pass in the generator (blog@13)
- **blog:** timeless meta description from a plan-generated field
- **content:** add local claude-code provider (subscription, no API credits)
- **content:** local content development & prompt testing
- **content:** per-release local output layout + clean claude-code generations
- **content:** version local blog output (history/ per run)
- **contentcheck:** deterministically flag invented counts + generic tech ledes
- **scheduler:** idle-detection auto-stop for the on-demand EC2 host

### Bug Fixes

- **blog:** emit plan TITLE as a parseable line so the timeless title is used
- **blog:** harden blog assembly against malformed model output
- **blog:** strip conversational scaffolding in the shared assembly path
- **contentcheck:** flag generic ledes only when ungrounded
- **deploy:** idle-stop degrades to no-VPC unless real subnets+SGs are supplied
- **providers:** raise writer max_tokens 4096 -> 8192 to stop article truncation
- **scheduler:** empty idle VPC-list defaults + template-level cfn-lint suppress
- **scheduler:** satisfy cfn-lint W1030 on idle-stop VPC list params

### Documentation

- **content:** add editorial-polished blog posts for v0.3.0, v0.6.0, v0.10.0


## [0.21.1] - 2026-07-27

### Features

- **ami:** match the Packer builder to the CPU worker (t3.xlarge, GPU off)
- **ami:** match the Packer builder to the CPU worker (t3.xlarge, GPU off)
- **archspec:** add AWS Architecture Diagram Specification artifact
- **blog:** generate timeless topic-based articles, not release notes
- **compute:** add instance metrics, health probe, and per-service log streams
- **compute:** own a dedicated content bucket instead of reusing artifacts
- **compute:** ship instance boot + worker + ollama logs to CloudWatch
- **content-admin:** operator CLI for history/compare/rollback/promote-latest (Phase 4)
- **contentmeta:** publish per-release metadata.json with provenance (Phase 2)
- **deploy:** publish generated content to S3 by default
- **experiments:** experiments/ namespace for A/B runs (Phase 5)
- **infra:** production storage foundation for generated content (Phase 1)
- **pipeline:** split content generation into Ollama analysis + Claude writing
- **publish:** make releases first-class in the output path
- **publish:** promote approved releases to latest/ (Phase 3)
- **releasegen:** align the blog prompt with the technical-writer standards
- **releasegen:** plan-then-write blog generation to kill release-note narration
- **routing:** hybrid AI routing — Claude for high-value, Ollama for commodity
- **routing:** wire AI_ROUTING_RULES through compute.yaml + deploy.yml
- **svgdiagram:** render architecture diagrams to self-contained SVG
- **writer:** add Anthropic API as a Claude writer provider

### Bug Fixes

- **ami:** install AWS CLI v2 from the official bundle, not the apt package
- **ami:** isolate Packer templates into subdirs so build-ami stops colliding
- **ami:** isolate Packer templates into subdirs so build-ami stops colliding
- **ami:** retry base-tools apt install to survive transient mirror skew
- **ci:** run build-ami in the production environment so OIDC AssumeRole works
- **compute:** silence the unused SSM agent
- **compute:** size the worker root volume to 40GB to match the baked AMI
- **infra:** allow cross-region Bedrock inference profiles for the writer
- **infra:** create the content bucket in bootstrap, not compute
- **ollama:** configurable inference timeout; raise default 5m→15m
- **pipeline:** salvage empty analysis via fallback + drop junk blog tags
- **reposource:** clone deep enough to read commit history
- **worker:** fall back to the local writer when Bedrock invocation fails

### Documentation

- **monitoring:** document the /blog-gen/instance log streams and live tail


## [0.21.0] - 2026-07-24

### Features

- **scripts:** add process-repository.sh to trigger /process and confirm completion


## [0.20.0] - 2026-07-23

### Features

- **mcp:** MCP integration & tool ecosystem (Milestone 22)


## [0.19.1] - 2026-07-22

### Bug Fixes

- **scheduler:** run the EC2 power window 7 days a week + DLQ hardening


## [0.19.0] - 2026-07-22

### Features

- **contentsuite:** unify release→all-artifacts pipeline; de-duplicate generators
- **infra:** reusable CloudFormation modules & cross-stack sharing (Milestone 21)
- **pipeline:** SemVer gate, parallel stages, and E2E fixtures (release readiness)
- **pipeline:** automated release path emits the full artifact suite (readiness H2)

### Bug Fixes

- **pipeline:** graceful degradation for thin releases (QA edge cases M1/M2)

### Documentation

- **scheduling:** add "Verifying the schedule is firing" section
- **security:** document prompt-injection risk & generated-content safety (QA)
- configuration reference + automated-vs-manual reconciliation (readiness)


## [0.18.0] - 2026-07-21

### Features

- **ami:** shared baked AMI management & image lifecycle (Milestone 20)


## [0.17.0] - 2026-07-20

### Features

- **platform:** extensibility platform & provider abstraction (Milestone 19)


## [0.16.0] - 2026-07-20

### Features

- **contentoptimizer:** AI content optimization engine (Milestone 18)


## [0.15.0] - 2026-07-20

### Features

- **contentanalytics:** content analytics & performance engine (Milestone 17)


## [0.14.0] - 2026-07-20

### Features

- **socialintel:** cross-platform social media intelligence (Milestone 16)


## [0.13.0] - 2026-07-20

### Features

- **publishing:** publishing automation platform (Milestone 15)


## [0.12.0] - 2026-07-20

### Features

- **governance:** review & approval workflow (Milestone 14)


## [0.11.0] - 2026-07-20

### Features

- **xthread:** technical X thread generator (Milestone 13)


## [0.10.0] - 2026-07-20

### Features

- **linkedin:** professional LinkedIn content generator (Milestone 12)


## [0.9.0] - 2026-07-20

### Features

- **architecture:** AWS architecture diagram generator (Milestone 11)


## [0.8.0] - 2026-07-20

### Features

- **seo:** canonical SEO metadata generator (Milestone 10)
- **visualassets:** AI image prompt generator (Milestone 9)


## [0.7.0] - 2026-07-19

### Features

- **tiktok:** TikTok video generator (Milestone 8)


## [0.6.0] - 2026-07-18

### Features

- **shorts:** YouTube Shorts generator (Milestone 7)


## [0.5.0] - 2026-07-18

### Features

- **youtube:** long-form YouTube script generator (Milestone 6)


## [0.4.0] - 2026-07-18

### Features

- **storyboard:** scene-by-scene video storyboard generator (Milestone 4)
- **voiceover:** synchronized voice-over script generator (Milestone 5)


## [0.3.0] - 2026-07-18

### Features

- **process:** let /process trigger release-content generation via releaseTag
- **releasegen:** long-form technical blog generation (Milestone 3)
- **releasepipeline:** close the loop from release to published content
- **releasesource:** read repos with the registered per-repo PAT
- **worker:** run the release-content pipeline on published-release events

### Bug Fixes

- **releasegen:** enforce a 150-160 char SEO meta description

### Documentation

- **architecture:** document Content Intelligence & Generation (M2-M3)
- **release-context:** clarify the endpoint is a standalone context API
- add release-to-content end-to-end validation runbook


## [0.2.0] - 2026-07-18

### Features

- **release-context:** /release-context endpoint, GitHub adapter, infra
- **releasecontext:** Content Intelligence engine for release analysis
- **releasegen:** content generation engine from Release Context

### Documentation

- **readme:** document the /release-context endpoint


## [0.1.7] - 2026-07-18

### Documentation

- **brand:** add SVG logo family and brand guide
- **brand:** add self-contained brand guide page to the repo
- **readme:** add theme-aware logo mark to the header
- **readme:** document scripts/bootstrap.sh in Installation


## [0.1.6] - 2026-07-18

### Bug Fixes

- **release:** honor flags after the subcommand (dry-run safety)

### Documentation

- add register-repository.sh wrapper for the registration curl


## [0.1.5] - 2026-07-18

### Bug Fixes

- **release:** refuse a forced bump when there are no commits to release


## [0.1.4] - 2026-07-18


## [0.1.3] - 2026-07-18


## [0.1.2] - 2026-07-18


## [0.1.1] - 2026-07-18

### Features

- **release:** commit and push CHANGELOG to the release branch

### Documentation

- clarify AI processing has two trigger sources (webhook or POST /process)


## [0.1.0] - 2026-07-16

### Features

- **adapters:** add metadata Get and secrets WebhookSecret read paths
- **analysis:** detect tech stack, dependencies, IaC, containers, CI/CD
- **api:** add manual processing trigger (POST /process)
- **app:** application bootstrap container
- **approve:** approvals dashboard CLI with auto-approve
- **archdiagram:** evidence-grounded AWS architecture diagram module
- **bootstrap:** deploy enablement (artifacts bucket + OIDC role + runbook)
- **compute:** export NotificationsSecret ARN as a stack output
- **compute:** make KEY_PAIR_NAME optional (launch with no SSH key)
- **compute:** migrate EC2 Spot to scheduled On-Demand runtime
- **compute:** pre-baked custom AMI for fast Spot startup
- **compute:** provision Ollama model serving on the instance
- **compute:** run the worker as a systemd service with SMTP secret
- **compute:** stack-managed EC2 key pair (AWS::EC2::KeyPair)
- **compute:** stack-managed EC2 key pair (AWS::EC2::KeyPair)
- **config:** environment-driven configuration loader
- **deploy:** CPU smoke-test path via INSTANCE_TYPE + ENABLE_GPU vars
- **deploy:** auto-preflight in deploy.yml (no manual setup/cleanup)
- **deploy:** auto-wire the custom AMI via SSM (no manual CUSTOM_AMI)
- **deploy:** optional GitHub-secret seeding of the SMTP password
- **events:** EventBridge publisher for matched events
- **generation:** blog-post generation from a repository snapshot
- **generation:** multiple content types and resilient GenerateAll
- **github:** minimal GitHub REST client
- **idle-shutdown:** scheduled Lambda to stop the idle instance
- **infra:** add compute stack (EC2 Spot + persistent EBS)
- **infra:** add network CloudFormation stack
- **infra:** add observability stack
- **infra:** add serverless stack
- **lifecycle:** idle-shutdown use case and stop support
- **lifecycle:** instance-starter Lambda and EC2 adapter
- **manual-trigger:** start the instance on demand (schedule override)
- **memory:** repository memory to skip already-published commits
- **metrics:** emit CloudWatch custom metrics via EMF
- **notify:** SES email notification channel
- **notify:** Slack/webhook notification channel
- **notify:** deliver email via Turbo SMTP instead of Amazon SES
- **notify:** run notifications (published/held/failed)
- **observability:** structured logging and typed errors
- **ollama:** local LLM inference client
- **pipeline:** compose process -> generate -> publish
- **pipeline:** quality review and optional human-approval gate
- **processing:** real git clone + filesystem retrieval
- **processing:** repository-processing seam with placeholders
- **publish:** S3 remote publish destination
- **publish:** write generated content to dated Markdown files
- **registration:** PAT onboarding use case, AWS adapters, and Lambda
- **release:** add semantic versioning & release management
- **release:** severity-based validation with dry-run-friendly warnings
- **repo:** repository model and GitHub URL parsing
- **resilience:** retry with backoff and prompt/context budget
- **scheduler:** change default stop time to 20:00 (6 PM–8 PM window)
- **scheduler:** daily start/stop of an EC2 instance via EventBridge Scheduler
- **secrets:** store all repo credentials in one shared secret
- **sqs:** add message consume (Receive/Delete) to the queue adapter
- **sqs:** queue-depth adapter for idle detection
- **trigger:** configurable per-repo trigger patterns (prefix + regex)
- **trigger:** support published GitHub releases as a trigger source
- **webhook:** commit-message trigger and HMAC signature verification
- **webhook:** lightweight webhook-handler Lambda
- **webhook:** publish matched events to EventBridge
- **worker:** instance worker driving the content pipeline

### Bug Fixes

- **bootstrap:** adopt existing artifacts bucket via IMPORT_EXISTING
- **bootstrap:** auto-detect and reuse an existing GitHub OIDC provider
- **bootstrap:** grant deploy role scheduler:* for the scheduler stack
- **compute:** attach data volume at boot, not via VolumeAttachment
- **deploy:** derive ARTIFACTS_BUCKET instead of reading a variable
- **infra:** declare Lambda log groups with the functions, not in observability
- **infra:** use one-time Spot request to prevent orphaned instances
- **network:** make the public-subnet AZ selectable for Spot capacity
- **network:** strip trailing newline from the security-group description
- **retry:** prioritise context cancellation over the backoff timer
- **security:** bump pinned Go toolchain to go1.26.5
- **security:** patch govulncheck findings
- **serverless:** make registration reserved concurrency opt-in

### Refactoring

- **lookup:** key shared-secret reads by repo full name
- **secrets:** store repo PAT + webhook secret in one JSON secret

### Documentation

- **deploy:** add one-page first-deploy runbook
- **first-deploy:** add AWS CloudShell bootstrap path + why it's manual
- **first-deploy:** troubleshoot OIDC "Not authorized" assume-role failures
- **plan:** add Phase 2 (content generation) section
- **plan:** add development plan and align build to single module
- **plan:** mark Milestone 3 done; bump Go to 1.24
- **plan:** mark Milestone 4 done
- **plan:** mark Milestone 5 done
- **plan:** mark Milestone 6 done
- **plan:** mark Milestone 7 done; MVP slice complete
- **readme:** add Status section reflecting the implemented MVP
- **registration:** add POST /repositories endpoint spec
- **registration:** document the required x-api-key + PAT flow prominently
- **registration:** document the required x-api-key + PAT flow prominently
- **release:** show runnable CLI commands and dry-run behavior
- **workflows:** add end-to-end webhook delivery sequence diagram
- add MVP repository registration with GitHub PATs
- add complete project documentation
- add repository registration + DynamoDB metadata store
- gate blog generation behind a commit-message trigger
- make GitHub Webhooks the primary trigger across the docs
- pivot to self-hosted OpenClaw + Ollama + EC2 Spot architecture
- reframe as developer content engine and reconcile all docs
- switch infrastructure from Terraform to AWS CloudFormation

