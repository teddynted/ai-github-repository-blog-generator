---
title: "Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS"
description: "Widget v1.0.0 delivers a production-ready event-driven pipeline: EventBridge ingestion, an SQS buffer with a DLQ, and a hardened EC2 worker. It builds on AWS…"
tags: [go, aws-lambda, amazon-sqs, amazon-eventbridge, aws-cloudformation, software-architecture, cloud-computing]
---

# Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

## Introduction

The system decouples events from generation.

## Architecture

EventBridge routes to SQS, drained by an EC2 worker.

## Architecture Diagrams

The following diagrams are taken directly from the repository's documentation.

_Webhook publishes to EventBridge, which buffers in SQS, drained by the EC2 worker._

```mermaid
flowchart TD
```

## Conclusion

The pattern generalises to event-driven workloads.
