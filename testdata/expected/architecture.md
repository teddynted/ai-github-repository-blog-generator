# Architecture Diagrams: widget

_Repository-level architecture overview · Event-driven_

> **Notes:** diagrams are generated from the repository README, documented AWS integrations, automation workflows, and the current project structure. The architecture document is intentionally **version-independent** so it can be reused across releases, branches, and generated documentation workflows.

---

## Platform Overview

Widget is an event-driven pipeline: a webhook publishes to EventBridge, which buffers events in SQS; a scheduled EC2 worker drains the queue and processes each widget.

---

## Deployment Architecture — AWS Integration

_Deployment architecture_

**Type:** Deployment Diagram · **Complexity:** low

```mermaid
flowchart TD
    subgraph Compute["Compute"]
        Amazon_EC2["Amazon EC2"]
    end
    subgraph Serverless["Serverless"]
        AWS_Lambda["AWS Lambda"]
    end
    subgraph Integration["Integration"]
        Amazon_EventBridge["Amazon EventBridge"]
        AWS_CloudFormation["AWS CloudFormation"]
    end
    subgraph Messaging["Messaging"]
        Amazon_SQS["Amazon SQS"]
    end
    subgraph Observability["Observability"]
        Amazon_CloudWatch["Amazon CloudWatch"]
    end
    style AWS_Lambda fill:#ED7100,stroke:#232F3E,color:#fff
    style Amazon_EventBridge fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_SQS fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_EC2 fill:#ED7100,stroke:#232F3E,color:#fff
    style AWS_CloudFormation fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_CloudWatch fill:#E7157B,stroke:#232F3E,color:#fff
```

### AWS Services Used

- AWS Lambda
- Amazon EventBridge
- Amazon SQS
- Amazon EC2
- AWS CloudFormation
- Amazon CloudWatch

---

## Logical Architecture — Data Flow

_Logical architecture_

**Type:** Data Flow Diagram · **Complexity:** low

```mermaid
flowchart LR
    Webhook["Webhook"]
    Amazon_EventBridge["Amazon EventBridge"]
    Amazon_SQS["Amazon SQS"]
    Amazon_EC2["Amazon EC2"]
    Webhook --> Amazon_EventBridge
    Amazon_EventBridge --> Amazon_SQS
    Amazon_SQS --> Amazon_EC2
    style Amazon_EventBridge fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_SQS fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_EC2 fill:#ED7100,stroke:#232F3E,color:#fff
```

### Key Components

- Webhook
- Amazon EventBridge
- Amazon SQS
- Amazon EC2

---

## CI/CD & Infrastructure Automation

_Delivery and infrastructure automation_

**Type:** Deployment Diagram · **Complexity:** medium

```mermaid
flowchart LR
    github["GitHub"]
    workflow["CI/CD Workflow"]
    cloudformation["AWS CloudFormation"]
    Amazon_EventBridge["Amazon EventBridge"]
    Amazon_SQS["Amazon SQS"]
    Amazon_EC2["Amazon EC2"]
    github -->|triggers| workflow
    workflow -->|deploys| cloudformation
    cloudformation -->|provisions| Amazon_EventBridge
    cloudformation -->|provisions| Amazon_SQS
    cloudformation -->|provisions| Amazon_EC2
    style cloudformation fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_EventBridge fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_SQS fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_EC2 fill:#ED7100,stroke:#232F3E,color:#fff
```

### Deployment Characteristics

- GitHub
- CI/CD Workflow
- AWS CloudFormation
- Amazon EventBridge
- Amazon SQS
- Amazon EC2

---

## Architecture Intelligence

| Attribute | Value |
| --- | --- |
| **Architecture style** | Event-driven |
| **Primary workflow** | Webhook → Amazon EventBridge → Amazon SQS → Amazon EC2 |
| **Infrastructure complexity** | medium |
| **Operational model** | Infrastructure as Code on AWS |
| **Centralized observability** | Amazon CloudWatch |
| **Estimated reading time** | 4 min |

---

## Generation Context

This document is generated from the **current repository state**, including the README, documentation, workflows, and project structure available at generation time. No release-specific version information is embedded in the architecture document.
