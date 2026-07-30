# AWS Architecture Diagram Specification

## Source Inputs
- blog.md
- architecture.md

## Generation Metadata
- Artifact Generator: architecture-diagram-spec
- Artifact Role: Derived intermediate representation (IR)
- Generated From Release: v1.0.0
- Compatibility: Renderer-safe, non-authoritative source artifact

## Introduction

The system decouples events from generation.

## Architecture

EventBridge routes to SQS, drained by an EC2 worker.

## Conclusion

The pattern generalises to event-driven workloads.

## Graph Definition Contract
- **Components** define the canonical node set.
- **Connections** define the canonical edge set.
- Renderers must not infer additional nodes or edges from descriptive text.
- **Operational Flow**, **Security**, **Failure Handling**, and **Rendering Notes** provide semantic annotations only.
- If descriptive text conflicts with the graph definition, **Components + Connections** take precedence.

## Renderer Contract
- This artifact is intended to be consumed directly by automated SVG / draw.io / Mermaid renderers.
- Renderers should treat the **Components** and **Connections** sections as the authoritative graph definition.
- **Operational Flow**, **Security**, and **Failure Handling** provide semantic annotations and must not introduce additional visual nodes unless explicitly declared in **Components**.
- If a conflict exists, **Components + Connections** take precedence over descriptive sections.
