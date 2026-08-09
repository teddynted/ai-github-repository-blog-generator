# Storyboard: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

_acme/widget · release v1.0.0 · 7 scenes · ~1:41 (short, 9:16)_

**Suggested title:** Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS  
**Audience:** Software engineers and cloud practitioners · **Difficulty:** intermediate · **Production:** medium · **Animation:** low

> **Notes:** the release blog has no Mermaid diagram; architecture scenes have no diagram references

---

## Scene 1 — An Event-Driven Pipeline on AWS

- **Objective:** Open on the article title and orient the viewer.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** The prototype polled for work on a fixed interval. Polling wasted compute when idle, added latency when busy, and coupled ingestion to processing.
- **Visual:** Clean title card: the article title centred on the brand background with a brief motion-graphic intro. No diagram, no photograph.
- **Camera:** Slow Zoom In — Ease onto the title to draw the viewer in.
- **Transition:** Cross Dissolve (0.6s)
- **Music:** upbeat, energetic intro

---

## Scene 2 — The problem

- **Objective:** Establish the problem and why this change matters.
- **Timing:** 17s recommended (14–22s, slow)
- **Narration:** The prototype polled for work on a fixed interval. Polling wasted compute when idle, added latency when busy, and coupled ingestion to processing. We needed a pipeline that absorbs bursts, decouples the front door from the worker, and keeps cost bounded.
- **Visual:** A before/after timeline dramatising the operational cost — animate the slow path filling up; keep on-screen text light so the narration carries it. Assets: Timeline, Flow Diagram, Motion Graphics.
- **Camera:** Focus Shift — Rack focus from context to the problem statement.
- **Animation:**
  1. Fade In → scene
- **Overlays:**
  - [Title] The problem
- **Assets:** Timeline, Flow Diagram, Motion Graphics
- **Transition:** Diagram Morph (0.8s)
- **Music:** neutral, technical

---

## Scene 3 — The architecture

- **Objective:** Explain the system architecture and how components interact.
- **Timing:** 25s recommended (22–30s, slow)
- **Narration:** Widget is now fully event-driven: - A webhook handler (AWS Lambda) validates each inbound event and publishes it to Amazon EventBridge. - An EventBridge rule routes matching events into an Amazon SQS queue, which provides durable buffering and retries; a dead-letter queue captures poison messages. - A scheduled EC2 worker drains the queue during its window and processes each widget.
- **Visual:** An architecture canvas that builds the components in one at a time, with a lower-third AWS-service label appearing as each is introduced. Assets: Architecture Diagram, AWS Icons.
- **Camera:** Diagram Focus — Frame the diagram; move to each highlighted node.
- **Animation:**
  1. Fade In → scene
- **Overlays:**
  - [Title] The architecture
  - [AWS Service Label] AWS Lambda
  - [AWS Service Label] Amazon EventBridge
  - [AWS Service Label] Amazon SQS
  - [AWS Service Label] Amazon EC2
  - [AWS Service Label] AWS CloudFormation
- **Assets:** Architecture Diagram, AWS Icons
- **Transition:** Cross Dissolve (0.6s)
- **Music:** calm, focused

---

## Scene 4 — How it works

- **Objective:** Explain how the feature was implemented.
- **Timing:** 24s recommended (21–29s, slow)
- **Narration:** When a widget event arrives, the Lambda verifies its signature and publishes it to EventBridge. Because EventBridge fans out to SQS, ingestion never blocks on processing — bursts pile up safely in the queue. The EC2 worker, started on a schedule, pulls messages, processes them, and relies on SQS retries plus the DLQ for anything that fails.
- **Visual:** A terminal or editor screen recording of the key mechanism — scroll or type the real identifiers, tags, and config, highlighting each as the narration reaches it. Assets: Code Editor, Terminal Recording.
- **Camera:** Push In — Push in on the key code as it is explained.
- **Animation:**
  1. Fade In → scene
  2. Code Typing → code — Type the key implementation code.
- **Overlays:**
  - [Title] How it works
- **Assets:** Code Editor, Terminal Recording
- **Transition:** Cross Dissolve (0.6s)
- **Music:** neutral, technical

---

## Scene 5 — Security

- **Objective:** Explain: Security.
- **Timing:** 9s recommended (6–14s, medium)
- **Narration:** The worker enforces IMDSv2, runs under a least-privilege IAM role, keeps secrets in AWS Secrets Manager, and only accepts HMAC-verified webhooks.
- **Visual:** A supporting visual for: Security. Assets: Code Editor.
- **Camera:** Static — Steady frame.
- **Animation:**
  1. Fade In → scene
- **Overlays:**
  - [Title] Security
- **Assets:** Code Editor
- **Transition:** Cross Dissolve (0.6s)
- **Music:** neutral, technical

---

## Scene 6 — Breaking change

- **Objective:** Explain: Breaking change.
- **Timing:** 6s recommended (4–11s, fast)
- **Narration:** The legacy polling endpoint has been removed. Migrate to the webhook + EventBridge ingestion path.
- **Visual:** A supporting visual for: Breaking change. Assets: Code Editor.
- **Camera:** Static — Steady frame.
- **Animation:**
  1. Fade In → scene
- **Overlays:**
  - [Title] Breaking change
- **Assets:** Code Editor
- **Transition:** Cross Dissolve (0.6s)
- **Music:** neutral, technical

---

## Scene 7 — Wrapping up

- **Objective:** Summarise the takeaways and point to what's next.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** Widget v1.0.0 turns a polling prototype into a durable, decoupled, cost-aware event-driven pipeline on AWS — reproducible with CloudFormation and hardened by default.
- **Visual:** Closing title card recapping the reusable pattern with a subtle call to action. Assets: Repository Logo, Title Card.
- **Camera:** Slow Zoom Out — Zoom out to close the video calmly.
- **Animation:**
  1. Fade In → scene
  2. Fade Out → scene — Fade out to the outro.
- **Overlays:**
  - [Title] Wrapping up
- **Assets:** Repository Logo, Title Card
- **Transition:** Fade (1.0s)
- **Music:** warm, resolving

---

## Chapters

- `0:00` An Event-Driven Pipeline on AWS
- `0:10` The problem
- `0:27` The architecture
- `0:52` How it works
- `1:16` Security
- `1:25` Breaking change
- `1:31` Wrapping up

