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

## The Queue That Absorbed the Release Storm

The system decouples release events from generation using a queue.

## What Running It Taught Me

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
		"forbidden heading": goodBlog + "\n## Conclusion\n\nWrap up.\n",
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

func TestBlogCatchesInventedCounts(t *testing.T) {
	// The exact leaks observed in live generation must now fail deterministically.
	for _, bad := range []string{
		"The repository organizes 65 files across the codebase.",
		"The system employs 15 architecture diagrams.",
		"The repository contains four top-level directories.",
		"It splits logic into three packages.",
	} {
		if Validate("blog", goodBlog+"\n"+bad).OK() {
			t.Errorf("invented count not caught: %q", bad)
		}
	}
	// Anaphoric references to a known number of things must NOT false-positive.
	for _, ok := range []string{
		"The two components scale separately without coordinating deployment.",
		"The repository separates orchestration from inference.",
	} {
		if !Validate("blog", goodBlog+"\n"+ok).OK() {
			t.Errorf("false positive on legitimate sentence: %q", ok)
		}
	}
}

func TestBlogCatchesGenericLedes(t *testing.T) {
	for _, bad := range []string{
		"Event-driven architectures decouple producers from consumers.",
		"AWS provides services for compute, routing, and storage.",
		"Go offers advantages for building distributed systems.",
		"Serverless architectures are popular for good reason.",
	} {
		if Validate("blog", goodBlog+"\n"+bad).OK() {
			t.Errorf("generic lede not caught: %q", bad)
		}
	}
	// Grounded sentences that merely START with a lede stem must PASS — the ban
	// is on ungrounded/teaching sentences, not any mention of the pattern.
	for _, ok := range []string{
		"The repository routes events through EventBridge to an SQS buffer.",
		"Event-driven architecture enables the platform to scale ingestion independently.",
		"AWS provides the managed services the pipeline relies on for inference.",
	} {
		if !Validate("blog", goodBlog+"\n"+ok).OK() {
			t.Errorf("false positive on grounded sentence: %q", ok)
		}
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
