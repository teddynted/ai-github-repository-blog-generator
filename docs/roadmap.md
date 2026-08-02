# Roadmap

Planned direction for the **GitHub AI Blog Generator**. Dates are indicative; scope may shift as the project evolves. Items marked ✅ are considered done for the milestone.

Related: [Requirements](./requirements.md) · [Contributing](./contributing.md).

```mermaid
timeline
    title Release Roadmap
    v1.0 : POST /process trigger : Claude via Bedrock->Anthropic Provider Router : Step Functions + on-demand t4g + CloudFormation
    v1.1 : Custom trigger patterns : Better prompts : Per-repo schedules
    v2.0 : More trigger sources : Multi-model : Web dashboard + approvals
    v3.0 : Multi-repo : Fine-tuned models : Collaboration
```

---

## Version 1.0 — Foundation

The core opt-in, event-driven pipeline, deployable from scratch with CloudFormation.

- ✅ **Repository registration (MVP)** — onboard with URL + GitHub PAT; validate access, store metadata (DynamoDB) and the PAT (Secrets Manager)
- ✅ **`POST /process` trigger** — the single, authenticated entry point that starts a Step Functions execution
- ✅ **Step Functions orchestration** — resolve host by tag → start → SSM ready gate → enqueue to SQS
- ✅ Durable job buffering with Amazon SQS (+ dead-letter queue)
- ✅ On-demand EC2 start (state machine) + scheduled/idle stop (EventBridge Scheduler)
- ✅ **AI Provider Router** — Amazon Bedrock (Claude Opus 4.8) primary, Anthropic API fallback on quota
- ✅ **S3 idempotency** — existing artifacts reused, never regenerated
- ✅ Repository analysis and **Repository Memory** for continuity
- ✅ Quality review and **optional human approval** before publishing
- ✅ Persistent gp3 EBS volume for n8n + PostgreSQL state and Repository Memory
- ✅ CloudFormation deployment (VPC, public subnet, IGW, route tables, SGs, IAM, On-Demand t4g EC2, EBS, API Gateway, Lambdas, Step Functions, EventBridge Scheduler, SQS, CloudWatch)
- ✅ n8n workflow orchestration with retries, error handling, and notifications

**Exit criteria:** deploying the stacks + importing the workflows + registering a repo yields a pipeline where `POST /process` starts the host, generates content via the Provider Router (reusing anything already in S3), publishes it, and the instance stops on the window/idle-stop.

---

## Version 1.1 — Trigger Flexibility & Resilience

Make the trigger configurable and the compute layer more robust.

- **Configurable custom trigger patterns** — `[blog]`, regular expressions, per-repository rules
- Improved prompts (better structure, tighter context selection, few-shot examples)
- **Configurable per-repository schedule windows** (beyond the default 18:00–20:00, 7 days a week)
- Multi-Availability-Zone instance placement
- Bedrock model right-sizing per artifact (cheaper models for template transforms)
- Faster cold start (cached Docker images, compose pre-pull)

---

## Version 2.0 — More Trigger Sources & Reach

Extend the trigger system and make model choice and publishing configurable — **without changing the core architecture** (all sources start the same Step Functions execution).

- **Additional trigger sources:** GitHub Release/PR webhooks, Git Tags, Pull Request labels
- ✅ **Manual generation** via an authenticated REST trigger (`POST /process`) — see [Manual Trigger](./manual-trigger.md)
- **Scheduled repository summaries**
- **Multi-model support** — different Bedrock/Anthropic models per content type
- Additional content types (video/short/podcast scripts, auto-generated diagrams)
- Multi-language content generation
- **Web dashboard** for run history, approvals, and content review

---

## Version 3.0 — Auth, Scale & Collaboration

From single-run tool to team platform, and from PATs to GitHub Apps.

- **GitHub App authentication** (recommended long-term approach, replacing per-repo PATs) + **OAuth login**
- **Push-based trigger sources** (release/PR webhooks), fine-grained repository permissions, and **secret rotation**
- **Multiple repositories per user**, repository groups/organisations, and **multi-user workspaces**
- **Web-based repository management dashboard** (run history, approvals, content review)
- **Multi-repository batch processing** (org-wide scans)
- **Fine-tuned / custom models** for documentation style
- A second local-model review pass for accuracy and tone

---

## Continuously (all versions)

- Hardening security and least-privilege posture
- Cost tracking and optimisation (trigger ratios, idle behaviour, EBS sizing)
- Expanded test coverage and CI checks
- Documentation upkeep

---

## Contributing to the Roadmap

Have an idea or want to pick up a milestone item? Open an issue describing the use case, or comment on an existing one. See [Contributing](./contributing.md). Roadmap items are tracked as GitHub issues/milestones so progress is visible.
