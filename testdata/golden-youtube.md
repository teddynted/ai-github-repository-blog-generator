# YouTube Script: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

_acme/widget · release v1.0.0 · long-form · ~1:39 · Software engineers and cloud practitioners · intermediate_

> **Notes:** runtime 1:39 is under the 10-minute long-form target; the upstream blog/storyboard may be too thin for a full long-form video

## Hook (`00:00–00:15`, showcase)

> In this release we wire up AWS Lambda, Amazon EventBridge, and Amazon SQS into one clean, event-driven pipeline. Widget v1.0.0 delivers a production-ready event-driven pipeline: EventBridge ingestion, an SQS buffer with a DLQ, and a hardened EC2 worker.

## Introduction (`00:15–00:31`)

> Welcome back. In this video we're doing a deep dive into acme/widget v1.0.0. Widget v1.0.0 delivers a production-ready event-driven pipeline: EventBridge ingestion, an SQS buffer with a DLQ, and a hardened EC2 worker. We'll be working with Go, AWS Lambda, Amazon SQS, and Amazon EventBridge. Here's the plan: we'll cover The problem, The architecture, How it works, Security, and Breaking change.

- **Technologies:** Go; AWS Lambda; Amazon SQS; Amazon EventBridge; AWS CloudFormation; Amazon EC2; Amazon CloudWatch
- **You'll learn:** Understand the problem this release solves; Read the architecture and how the components fit together; Follow how the feature was implemented
- **Agenda:** The problem; The architecture; How it works; Security; Breaking change

---

## Chapter 1 — The problem

- **Timestamp:** `00:15–00:31` (target 16s; 11–31s)
- **Storyboard scenes:** 1 · **Voice-over scenes:** 1

### Narration

> The prototype polled for work on a fixed interval. Polling wasted compute when idle, added latency when busy, and coupled ingestion to processing. We needed a pipeline that absorbs bursts, decouples the front door from the worker, and keeps cost bounded.

- **Transition:** With the problem clear, let's step through the architecture.

---

## Chapter 2 — The architecture

- **Timestamp:** `00:31–00:54` (target 23s; 18–38s)
- **Storyboard scenes:** 2 · **Voice-over scenes:** 2

### Narration

> Widget is now fully event-driven: - A webhook handler (AWS Lambda) validates each inbound event and publishes it to Amazon EventBridge. - An EventBridge rule routes matching events into an Amazon SQS queue, which provides durable buffering and retries; a dead-letter queue captures poison messages. - A scheduled EC2 worker drains the queue during its window and processes each widget. Widget is an event-driven pipeline: a webhook publishes to EventBridge, which buffers events in SQS; a scheduled EC2 worker drains the queue and processes each widget. On the AWS side we lean on AWS Lambda, Amazon EventBridge, Amazon SQS, and Amazon EC2. webhook-handler handles validate and publish inbound events. worker handles drain SQS and process widgets. The flow works like this: webhook → EventBridge → SQS → EC2 worker.

- **Visual references:** Diagram: ; AWS service icons: AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2
**Callouts:**
  - [Architecture Decision] Widget is an event-driven pipeline: a webhook publishes to EventBridge, which buffers events in SQS; a scheduled EC2 worker drains the queue and processes each widget.
  - [Best Practice] Keep the diagram the single source of truth — the storyboard and this script both reference it rather than redrawing it.
- 💬 **Engage:** Pause here and try to predict how the components talk to each other before I reveal it.
- **Transition:** Now that we've walked the architecture, let's see how it's built.

---

## Chapter 3 — How it works

- **Timestamp:** `00:54–01:16` (target 22s; 17–37s)
- **Storyboard scenes:** 3 · **Voice-over scenes:** 3

### Narration

> When a widget event arrives, the Lambda verifies its signature and publishes it to EventBridge. Because EventBridge fans out to SQS, ingestion never blocks on processing — bursts pile up safely in the queue. The EC2 worker, started on a schedule, pulls messages, processes them, and relies on SQS retries plus the DLQ for anything that fails.

- **Visual references:** Source file (code editor); Terminal: test run
**Demonstration:**
  1. Open the key source file — Show the entry point and the interface it sits behind.
  2. Run the tests — Show the unit tests passing for this package.
**Callouts:**
  - [Tip] Notice how the logic sits behind a small interface, which is what keeps it unit-testable.
- **Transition:** With the implementation in place, let's move on to security.

---

## Chapter 4 — Security

- **Timestamp:** `01:16–01:24` (target 8s; 5–23s)
- **Storyboard scenes:** 4 · **Voice-over scenes:** 4

### Narration

> The worker enforces IMDSv2, runs under a least-privilege IAM role, keeps secrets in AWS Secrets Manager, and only accepts HMAC-verified webhooks.

- 💬 **Engage:** Let me know in the comments if this maps to something you're building.
- **Transition:** From here, let's move on to breaking change.

---

## Chapter 5 — Breaking change

- **Timestamp:** `01:24–01:30` (target 6s; 5–21s)
- **Storyboard scenes:** 5 · **Voice-over scenes:** 5

### Narration

> The legacy polling endpoint has been removed. Migrate to the webhook + EventBridge ingestion path.

- **Transition:** From here, let's wrap up.

---

## Chapter 6 — Wrapping up

- **Timestamp:** `01:30–01:39` (target 9s; 5–24s)
- **Storyboard scenes:** 6 · **Voice-over scenes:** 6

### Narration

> Widget v1.0.0 turns a polling prototype into a durable, decoupled, cost-aware event-driven pipeline on AWS — reproducible with CloudFormation and hardened by default.

- **Transition:** That's the release end to end — thanks for watching.

---

## Conclusion (`01:30–01:39`)

> So that's acme/widget v1.0.0 end to end. Next up: we keep building the pipeline release by release. Thanks for watching.

- **Key takeaways:** Keep the diagram the single source of truth — the storyboard and this script both reference it rather than redrawing it.
- **Next:** we keep building the pipeline release by release

---

## Call to Action

> If you got something out of this, do three quick things: star acme/widget so you can find it again, subscribe so you catch the next release deep dive, and drop a comment with how you'd approach it differently — I read them. Links to the repo and the docs are in the description.

- **GitHub Repository** — Star and explore acme/widget (https://github.com/acme/widget)
- **Subscribe** — Subscribe for the next release deep dive
- **Like** — Like the video if the walkthrough helped
- **Comment** — Comment with how you'd build this differently
- **Future Releases** — Follow along as the pipeline grows release by release
- **Contribute** — Open-source contributions are welcome — issues and PRs both

---

## Production Metadata

- **Estimated runtime:** 1:39 · **Speaking time:** 3:03 · **Words:** 458
- **Audience:** Software engineers and cloud practitioners · **Difficulty:** intermediate · **Reading level:** intermediate
- **Suggested title:** Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS
- **Alternative titles:** How acme/widget v1.0.0 Actually Works; Building v1.0.0: A Full Architecture Walkthrough
- **Thumbnail text:** V1.0.0 · AWS Lambda
- **Playlist:** acme/widget — Release Deep Dives
- **Technical topics:** AWS Lambda; Amazon EventBridge; Amazon SQS; Amazon EC2; AWS CloudFormation; Amazon CloudWatch; Go
- **SEO keywords:** AWS Lambda; Amazon EventBridge; Amazon SQS; Amazon EC2; AWS CloudFormation; Amazon CloudWatch; Go
- **Tags:** AWS Lambda; Amazon EventBridge; Amazon SQS; Amazon EC2; AWS CloudFormation; Amazon CloudWatch; Go; software engineering; aws; golang; clean architecture; devops; tutorial; system design
- **Chapter markers:**
  - `00:00` Intro / Hook
  - `00:15` The problem
  - `00:31` The architecture
  - `00:54` How it works
  - `01:16` Security
  - `01:24` Breaking change
  - `01:30` Wrapping up
  - `01:30` Conclusion
- **Pinned comment:** 📌 acme/widget v1.0.0 — everything in this video is generated from the repository's own Release Context. Repo: https://github.com/acme/widget What would you like the next deep dive to cover?

### Suggested Description

```
Widget v1.0.0 delivers a production-ready event-driven pipeline: EventBridge ingestion, an SQS buffer with a DLQ, and a hardened EC2 worker.

⏱ Chapters:
00:00 Intro / Hook
00:15 The problem
00:31 The architecture
00:54 How it works
01:16 Security
01:24 Breaking change
01:30 Wrapping up
01:30 Conclusion

🔗 Repository: https://github.com/acme/widget

#AWSLambda #AmazonEventBridge #AmazonSQS #AmazonEC2 #AWSCloudFormation #AmazonCloudWatch #Go
```
