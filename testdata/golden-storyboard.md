# Storyboard: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

_acme/widget · release v1.0.0 · 6 scenes · ~1:25 (short, 9:16)_

**Suggested title:** Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS  
**Audience:** Software engineers and cloud practitioners · **Difficulty:** intermediate · **Production:** high · **Animation:** low

---

## Scene 1 — The problem

- **Objective:** Establish the problem and why this change matters.
- **Timing:** 16s recommended (13–21s, medium)
- **Narration:** The prototype polled for work on a fixed interval. Polling wasted compute when idle, added latency when busy, and coupled ingestion to processing. We needed a pipeline that absorbs bursts, decouples the front door from the worker, and keeps cost bounded.
- **Visual:** Text-forward slide framing the problem, with a supporting timeline or before/after graphic. Assets: Timeline, Flow Diagram.
- **Camera:** Focus Shift — Rack focus from context to the problem statement.
- **Animation:**
  1. Fade In → scene
- **Overlays:**
  - [Title] The problem
- **Assets:** Timeline, Flow Diagram
- **Transition:** Diagram Morph (0.8s)
- **Music:** neutral, technical

---

## Scene 2 — The architecture

- **Objective:** Explain the system architecture and how components interact.
- **Timing:** 23s recommended (20–28s, slow)
- **Narration:** Widget is now fully event-driven: - A webhook handler (AWS Lambda) validates each inbound event and publishes it to Amazon EventBridge. - An EventBridge rule routes matching events into an Amazon SQS queue, which provides durable buffering and retries; a dead-letter queue captures poison messages. - A scheduled EC2 worker drains the queue during its window and processes each widget.
- **Visual:** The architecture diagram () centred on canvas, building and highlighting nodes as narration proceeds. Assets: Architecture Diagram, AWS Icons.
- **Camera:** Diagram Focus — Frame the diagram; move to each highlighted node.
- **Animation:**
  1. Fade In → scene
  2. Diagram Build — Build the diagram edge by edge.
  3. Highlight Node → Webhook — Highlight and label the node as narration reaches it.
  4. Highlight Node → EventBridge — Highlight and label the node as narration reaches it.
  5. Highlight Node → SQS — Highlight and label the node as narration reaches it.
  6. Highlight Node → Worker — Highlight and label the node as narration reaches it.
  7. Draw Arrow — Trace the data/control flow between nodes.
- **Overlays:**
  - [Title] The architecture
  - [AWS Service Label] AWS Lambda
  - [AWS Service Label] Amazon EventBridge
  - [AWS Service Label] Amazon SQS
  - [AWS Service Label] Amazon EC2
  - [AWS Service Label] AWS CloudFormation
- **Diagrams:**
  -  (flowchart) — Diagram Build; highlight: Webhook, EventBridge, SQS, Worker
- **Assets:** Architecture Diagram, AWS Icons
- **Transition:** Cross Dissolve (0.6s)
- **Music:** calm, focused

---

## Scene 3 — How it works

- **Objective:** Explain how the feature was implemented.
- **Timing:** 22s recommended (19–27s, slow)
- **Narration:** When a widget event arrives, the Lambda verifies its signature and publishes it to EventBridge. Because EventBridge fans out to SQS, ingestion never blocks on processing — bursts pile up safely in the queue. The EC2 worker, started on a schedule, pulls messages, processes them, and relies on SQS retries plus the DLQ for anything that fails.
- **Visual:** A code editor focused on the key implementation, with the relevant lines highlighted. Assets: Code Editor, Terminal Recording.
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

## Scene 4 — Security

- **Objective:** Explain: Security.
- **Timing:** 8s recommended (5–13s, fast)
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

## Scene 5 — Breaking change

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

## Scene 6 — Wrapping up

- **Objective:** Summarise the takeaways and point to what's next.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** Widget v1. 0. 0 turns a polling prototype into a durable, decoupled, cost-aware event-driven pipeline on AWS — reproducible with CloudFormation and hardened by default.
- **Visual:** Closing title card recapping key takeaways with a subtle call to action. Assets: Repository Logo, Title Card.
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

- `0:00` The problem
- `0:16` The architecture
- `0:39` How it works
- `1:01` Security
- `1:09` Breaking change
- `1:15` Wrapping up

