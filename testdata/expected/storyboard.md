# Storyboard: Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

_acme/widget · release v1.0.0 · 4 scenes · ~0:40 (short, 9:16)_

**Suggested title:** Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS  
**Audience:** Software engineers and cloud practitioners · **Difficulty:** intermediate · **Production:** medium · **Animation:** low

---

## Scene 1 — Introduction

- **Objective:** Hook the viewer and frame what the release is about.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** ## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
- **Visual:** Animated title card with the repository logo and release tag; brief motion-graphic intro. Assets: Repository Logo, Title Card.
- **Camera:** Slow Zoom In — Ease onto the title to draw the viewer in.
- **Animation:**
  1. Fade In → scene
  2. Scale Up → title — Title card scales up into place.
- **Overlays:**
  - [Title] Introduction
  - [Subtitle] acme/widget v1.0.0
- **Assets:** Repository Logo, Title Card
- **Transition:** Diagram Morph (0.8s)
- **Music:** upbeat, energetic intro

---

## Scene 2 — Architecture

- **Objective:** Explain the system architecture and how components interact.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** ## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
- **Visual:** The architecture diagram (release architecture diagram): build the graph edge by edge, lower-third each AWS service as narration names it, and pulse the single most important cross-plane hand-off. Assets: Architecture Diagram, AWS Icons.
- **Camera:** Diagram Focus — Frame the diagram; move to each highlighted node.
- **Animation:**
  1. Fade In → scene
  2. Diagram Build → release architecture diagram — Build the diagram edge by edge.
  3. Draw Arrow → release architecture diagram — Trace the primary flow between the key nodes.
- **Overlays:**
  - [Title] Architecture
  - [AWS Service Label] AWS Lambda
  - [AWS Service Label] Amazon EventBridge
  - [AWS Service Label] Amazon SQS
  - [AWS Service Label] Amazon EC2
  - [AWS Service Label] AWS CloudFormation
- **Diagrams:**
  - release architecture diagram (flowchart) — Diagram Build; highlight: 
- **Assets:** Architecture Diagram, AWS Icons
- **Transition:** Diagram Morph (0.8s)
- **Music:** calm, focused

---

## Scene 3 — Architecture Diagrams

- **Objective:** Walk through the architecture diagram visually.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** ## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
- **Visual:** Recall the architecture diagram (release architecture diagram) already on screen — do NOT rebuild it. Pan/zoom to the region this scene discusses and re-highlight only its nodes. Assets: Architecture Diagram, AWS Icons.
- **Camera:** Diagram Focus — Frame the diagram; move to each highlighted node.
- **Animation:**
  1. Fade In → scene
  2. Recall Diagram → release architecture diagram — Bring the existing diagram back; pan/zoom to this scene's region rather than rebuilding it.
  3. Draw Arrow → release architecture diagram — Trace the primary flow between the key nodes.
- **Overlays:**
  - [Title] Architecture Diagrams
  - [AWS Service Label] AWS Lambda
  - [AWS Service Label] Amazon EventBridge
  - [AWS Service Label] Amazon SQS
  - [AWS Service Label] Amazon EC2
  - [AWS Service Label] AWS CloudFormation
- **Diagrams:**
  - release architecture diagram (flowchart) — Diagram Build; highlight: 
- **Assets:** Architecture Diagram, AWS Icons
- **Transition:** Cross Dissolve (0.6s)
- **Music:** calm, focused

---

## Scene 4 — Conclusion

- **Objective:** Summarise the takeaways and point to what's next.
- **Timing:** 10s recommended (7–15s, medium)
- **Narration:** ## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.
- **Visual:** Closing title card recapping the reusable pattern with a subtle call to action. Assets: Repository Logo, Title Card.
- **Camera:** Slow Zoom Out — Zoom out to close the video calmly.
- **Animation:**
  1. Fade In → scene
  2. Fade Out → scene — Fade out to the outro.
- **Overlays:**
  - [Title] Conclusion
- **Assets:** Repository Logo, Title Card
- **Transition:** Fade (1.0s)
- **Music:** warm, resolving

---

## Chapters

- `0:00` Introduction
- `0:10` Architecture
- `0:20` Architecture Diagrams
- `0:30` Conclusion

