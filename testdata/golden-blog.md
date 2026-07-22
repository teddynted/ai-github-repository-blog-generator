---
title: "Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS"
description: "How Widget v1.0.0 processes events with EventBridge, SQS, and a hardened EC2 worker."
tags: [aws, serverless, eventbridge, sqs, go]
---

# Shipping Widget v1.0.0: An Event-Driven Pipeline on AWS

Widget v1.0.0 is the first production release — an event-driven, serverless
widget-processing platform built with Go on AWS. This post walks through what
changed, how the pipeline works, and why it matters.

## The problem

The prototype polled for work on a fixed interval. Polling wasted compute when
idle, added latency when busy, and coupled ingestion to processing. We needed a
pipeline that absorbs bursts, decouples the front door from the worker, and keeps
cost bounded.

## The architecture

Widget is now fully event-driven:

- A **webhook handler** (AWS Lambda) validates each inbound event and publishes it
  to **Amazon EventBridge**.
- An EventBridge **rule** routes matching events into an **Amazon SQS** queue,
  which provides durable buffering and retries; a **dead-letter queue** captures
  poison messages.
- A scheduled **EC2 worker** drains the queue during its window and processes each
  widget.

The bus, queue, DLQ, and worker are defined as a single **AWS CloudFormation**
stack, so the whole pipeline is reproducible infrastructure-as-code.

## How it works

When a widget event arrives, the Lambda verifies its signature and publishes it to
EventBridge. Because EventBridge fans out to SQS, ingestion never blocks on
processing — bursts pile up safely in the queue. The EC2 worker, started on a
schedule, pulls messages, processes them, and relies on SQS retries plus the DLQ
for anything that fails.

## Security

The worker enforces **IMDSv2**, runs under a **least-privilege IAM role**, keeps
secrets in **AWS Secrets Manager**, and only accepts **HMAC-verified** webhooks.

## Breaking change

The legacy polling endpoint has been **removed**. Migrate to the webhook +
EventBridge ingestion path.

## Wrapping up

Widget v1.0.0 turns a polling prototype into a durable, decoupled, cost-aware
event-driven pipeline on AWS — reproducible with CloudFormation and hardened by
default.
