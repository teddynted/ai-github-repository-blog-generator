package contentcheck

import (
	"strings"
	"testing"
)

const goodBlog = `---
title: "Designing an Event-Driven Platform on AWS"
description: "How the system routes events through SQS to an EC2 worker."
tags: [aws, go]
---

# Designing an Event-Driven Platform on AWS

## Introduction

The system decouples release events from generation using a queue.

## Conclusion

The pattern generalises to any event-driven workload.
`

func TestBlogValidatorPasses(t *testing.T) {
	r := Validate("blog", goodBlog)
	if !r.OK() {
		t.Errorf("clean blog should pass:\n%s", r.String())
	}
}

func TestBlogValidatorCatchesRegressions(t *testing.T) {
	cases := map[string]string{
		"missing section":   strings.Replace(goodBlog, "## Conclusion", "## Wrap Up", 1),
		"marketing phrase":  goodBlog + "\nThis is a groundbreaking, revolutionary system.\n",
		"placeholder":       goodBlog + "\nTODO: write this part.\n",
		"too many diagrams": goodBlog + "\n```mermaid\nA-->B\n```\n```mermaid\nC-->D\n```\n```mermaid\nE-->F\n```\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if Validate("blog", content).OK() {
				t.Errorf("expected %s to fail validation", name)
			}
		})
	}
}

func TestGenericEmptyFails(t *testing.T) {
	if Validate("linkedin", "   ").OK() {
		t.Error("empty content should fail")
	}
}

func TestSVGValidator(t *testing.T) {
	if Validate("architecture-diagram", "<svg><rect/></svg>").OK() == false {
		t.Error("valid svg should pass")
	}
	if Validate("architecture-diagram", "not an svg").OK() {
		t.Error("non-svg should fail")
	}
}

func TestReportIsHumanReadable(t *testing.T) {
	r := Validate("blog", "empty-ish")
	s := r.String()
	if !strings.Contains(s, "blog") || (!strings.Contains(s, "✓") && !strings.Contains(s, "✗")) {
		t.Errorf("report not human-readable:\n%s", s)
	}
}

func TestHedgeIsWarnNotError(t *testing.T) {
	// A hallucination hedge is a warning — it must not fail an otherwise clean blog.
	content := strings.Replace(goodBlog, "decouples release events", "probably decouples events", 1)
	r := Validate("blog", content)
	if !r.OK() {
		t.Errorf("hedge should warn, not fail:\n%s", r.String())
	}
	if r.Warnings() == 0 {
		t.Error("expected a warning for the hedge")
	}
}
