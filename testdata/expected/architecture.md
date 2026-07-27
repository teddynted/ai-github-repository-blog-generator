# Architecture Diagrams: acme/widget v1.0.0

_4 diagrams · Event-driven serverless architecture · confidence 92/100_

---

## widget v1.0.0 — High-Level AWS Architecture

_Services grouped by category_

**Type:** High-Level Architecture · **Complexity:** low · **Confidence:** 100/100

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.

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

**AWS services:** AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, AWS CloudFormation, Amazon CloudWatch

**Grounded in:** AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, AWS CloudFormation, Amazon CloudWatch

<details><summary>Graphviz (DOT)</summary>

```dot
digraph Architecture {
  rankdir=TB;
  graph [fontname="Helvetica", splines=true, nodesep=0.5, ranksep=0.6];
  node [shape=box, style="rounded,filled", fontname="Helvetica", fillcolor="#EEF1F5", color="#232F3E"];
  edge [fontname="Helvetica", color="#546174"];

  subgraph cluster_0 {
    label="Compute";
    style="rounded";
    color="#B7C0CD";
    Amazon_EC2 [label="Amazon EC2", fillcolor="#ED7100", fontcolor="#FFFFFF"];
  }
  subgraph cluster_1 {
    label="Serverless";
    style="rounded";
    color="#B7C0CD";
    AWS_Lambda [label="AWS Lambda", fillcolor="#ED7100", fontcolor="#FFFFFF"];
  }
  subgraph cluster_2 {
    label="Integration";
    style="rounded";
    color="#B7C0CD";
    Amazon_EventBridge [label="Amazon EventBridge", fillcolor="#E7157B", fontcolor="#FFFFFF"];
    AWS_CloudFormation [label="AWS CloudFormation", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  }
  subgraph cluster_3 {
    label="Messaging";
    style="rounded";
    color="#B7C0CD";
    Amazon_SQS [label="Amazon SQS", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  }
  subgraph cluster_4 {
    label="Observability";
    style="rounded";
    color="#B7C0CD";
    Amazon_CloudWatch [label="Amazon CloudWatch", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  }

}
```

</details>

**PNG export:** 1920x1080 (16:9), 144 DPI, transparent background

---

## widget v1.0.0 — Event-Driven Flow

_Grounded in the release's described flows_

**Type:** Event-Driven Architecture · **Complexity:** low · **Confidence:** 90/100

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.

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

**AWS services:** Amazon EventBridge, Amazon SQS, Amazon EC2

**Grounded in:** webhook → EventBridge → SQS → EC2 worker

<details><summary>Graphviz (DOT)</summary>

```dot
digraph Architecture {
  rankdir=LR;
  graph [fontname="Helvetica", splines=true, nodesep=0.5, ranksep=0.6];
  node [shape=box, style="rounded,filled", fontname="Helvetica", fillcolor="#EEF1F5", color="#232F3E"];
  edge [fontname="Helvetica", color="#546174"];

  Webhook [label="Webhook", fillcolor="#EEF1F5", fontcolor="#232F3E"];
  Amazon_EventBridge [label="Amazon EventBridge", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_SQS [label="Amazon SQS", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_EC2 [label="Amazon EC2", fillcolor="#ED7100", fontcolor="#FFFFFF"];

  Webhook -> Amazon_EventBridge;
  Amazon_EventBridge -> Amazon_SQS;
  Amazon_SQS -> Amazon_EC2;
}
```

</details>

**PNG export:** 1920x1080 (16:9), 144 DPI, transparent background

---

## widget v1.0.0 — Request Sequence

**Type:** Sequence Diagram · **Complexity:** low · **Confidence:** 90/100

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.

```mermaid
sequenceDiagram
    participant Webhook as Webhook
    participant Amazon_EventBridge as Amazon EventBridge
    participant Amazon_SQS as Amazon SQS
    participant Amazon_EC2 as Amazon EC2
    Webhook->>Amazon_EventBridge: sends to
    Amazon_EventBridge->>Amazon_SQS: sends to
    Amazon_SQS->>Amazon_EC2: sends to
```

**AWS services:** Amazon EventBridge, Amazon SQS, Amazon EC2

**Grounded in:** webhook → EventBridge → SQS → EC2 worker

<details><summary>Graphviz (DOT)</summary>

```dot
digraph Architecture {
  rankdir=LR;
  graph [fontname="Helvetica", splines=true, nodesep=0.5, ranksep=0.6];
  node [shape=box, style="rounded,filled", fontname="Helvetica", fillcolor="#EEF1F5", color="#232F3E"];
  edge [fontname="Helvetica", color="#546174"];

  Webhook [label="Webhook", fillcolor="#EEF1F5", fontcolor="#232F3E"];
  Amazon_EventBridge [label="Amazon EventBridge", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_SQS [label="Amazon SQS", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_EC2 [label="Amazon EC2", fillcolor="#ED7100", fontcolor="#FFFFFF"];

  Webhook -> Amazon_EventBridge;
  Amazon_EventBridge -> Amazon_SQS;
  Amazon_SQS -> Amazon_EC2;
}
```

</details>

**PNG export:** 1920x1080 (16:9), 144 DPI, transparent background

---

## widget v1.0.0 — CI/CD & Provisioning

_From release to provisioned infrastructure_

**Type:** CI/CD Pipeline · **Complexity:** medium · **Confidence:** 91/100

## Introduction The system decouples events from generation. ## Architecture EventBridge routes to SQS, drained by an EC2 worker. ## Conclusion The pattern generalises to event-driven workloads.

```mermaid
flowchart LR
    github["GitHub Release"]
    workflow["CI/CD Workflow"]
    cloudformation["AWS CloudFormation"]
    AWS_Lambda["AWS Lambda"]
    Amazon_EventBridge["Amazon EventBridge"]
    Amazon_SQS["Amazon SQS"]
    Amazon_EC2["Amazon EC2"]
    AWS_CloudFormation["AWS CloudFormation"]
    Amazon_CloudWatch["Amazon CloudWatch"]
    github -->|triggers| workflow
    workflow -->|deploys| cloudformation
    cloudformation -->|provisions| AWS_Lambda
    cloudformation -->|provisions| Amazon_EventBridge
    cloudformation -->|provisions| Amazon_SQS
    cloudformation -->|provisions| Amazon_EC2
    cloudformation -->|provisions| AWS_CloudFormation
    cloudformation -->|provisions| Amazon_CloudWatch
    style cloudformation fill:#E7157B,stroke:#232F3E,color:#fff
    style AWS_Lambda fill:#ED7100,stroke:#232F3E,color:#fff
    style Amazon_EventBridge fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_SQS fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_EC2 fill:#ED7100,stroke:#232F3E,color:#fff
    style AWS_CloudFormation fill:#E7157B,stroke:#232F3E,color:#fff
    style Amazon_CloudWatch fill:#E7157B,stroke:#232F3E,color:#fff
```

**AWS services:** AWS CloudFormation, AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, Amazon CloudWatch

**Grounded in:** infrastructure/pipeline.yaml, AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, AWS CloudFormation, Amazon CloudWatch

<details><summary>Graphviz (DOT)</summary>

```dot
digraph Architecture {
  rankdir=LR;
  graph [fontname="Helvetica", splines=true, nodesep=0.5, ranksep=0.6];
  node [shape=box, style="rounded,filled", fontname="Helvetica", fillcolor="#EEF1F5", color="#232F3E"];
  edge [fontname="Helvetica", color="#546174"];

  github [label="GitHub Release", fillcolor="#EEF1F5", fontcolor="#232F3E"];
  workflow [label="CI/CD Workflow", fillcolor="#EEF1F5", fontcolor="#232F3E"];
  cloudformation [label="AWS CloudFormation", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  AWS_Lambda [label="AWS Lambda", fillcolor="#ED7100", fontcolor="#FFFFFF"];
  Amazon_EventBridge [label="Amazon EventBridge", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_SQS [label="Amazon SQS", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_EC2 [label="Amazon EC2", fillcolor="#ED7100", fontcolor="#FFFFFF"];
  AWS_CloudFormation [label="AWS CloudFormation", fillcolor="#E7157B", fontcolor="#FFFFFF"];
  Amazon_CloudWatch [label="Amazon CloudWatch", fillcolor="#E7157B", fontcolor="#FFFFFF"];

  github -> workflow [label="triggers"];
  workflow -> cloudformation [label="deploys"];
  cloudformation -> AWS_Lambda [label="provisions"];
  cloudformation -> Amazon_EventBridge [label="provisions"];
  cloudformation -> Amazon_SQS [label="provisions"];
  cloudformation -> Amazon_EC2 [label="provisions"];
  cloudformation -> AWS_CloudFormation [label="provisions"];
  cloudformation -> Amazon_CloudWatch [label="provisions"];
}
```

</details>

**PNG export:** 1920x1080 (16:9), 144 DPI, transparent background

---

## Architecture Intelligence

- **Style:** Event-driven serverless · **Deployment:** Single-region, single public subnet; serverless front door, scheduled EC2 compute.
- **Infrastructure complexity:** medium
- **Cloud services:** AWS Lambda, Amazon EventBridge, Amazon SQS, Amazon EC2, AWS CloudFormation, Amazon CloudWatch
- **Compute:** Amazon EC2
- **Serverless:** AWS Lambda
- **Messaging:** Amazon SQS
- **Integration:** Amazon EventBridge, AWS CloudFormation
- **Observability:** Amazon CloudWatch
- **Estimated reading time:** 4 min · **Diagram confidence:** 92/100

