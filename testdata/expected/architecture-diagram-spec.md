# AWS Architecture Diagram Specification

## Introduction

The system decouples events from generation.

## Architecture

EventBridge routes to SQS, drained by an EC2 worker.

## Conclusion

The pattern generalises to event-driven workloads.