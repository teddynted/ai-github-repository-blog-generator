package main

import (
	"os"
	"strings"
	"testing"
)

// capture runs f with stdout redirected and returns what it printed.
func capture(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestRunServers(t *testing.T) {
	var code int
	out := capture(t, func() { code = run([]string{"servers"}) })
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"repository", "mermaid", "cloudformation", "context"} {
		if !strings.Contains(out, want) {
			t.Errorf("servers output missing %q", want)
		}
	}
}

func TestRunTools(t *testing.T) {
	var code int
	out := capture(t, func() { code = run([]string{"tools", "mermaid"}) })
	if code != 0 || !strings.Contains(out, "flowchart") {
		t.Fatalf("tools mermaid: code=%d out=%s", code, out)
	}
}

func TestRunCall(t *testing.T) {
	var code int
	out := capture(t, func() { code = run([]string{"call", "mermaid", "flowchart", "steps=Build|Ship", "direction=LR"}) })
	if code != 0 || !strings.Contains(out, "flowchart LR") {
		t.Fatalf("call: code=%d out=%s", code, out)
	}
}

func TestRunCallToolError(t *testing.T) {
	// A tool-level error result → exit code 1.
	code := run([]string{"call", "mermaid", "validate", "source=not a diagram"})
	if code != 1 {
		t.Fatalf("tool error should exit 1, got %d", code)
	}
}

func TestRunDiscover(t *testing.T) {
	var code int
	out := capture(t, func() { code = run([]string{"discover"}) })
	if code != 0 || !strings.Contains(out, "# mermaid") {
		t.Fatalf("discover: code=%d", code)
	}
}

func TestRunResources(t *testing.T) {
	var code int
	out := capture(t, func() { code = run([]string{"resources", "repository"}) })
	if code != 0 || !strings.Contains(out, "no resources") {
		t.Fatalf("resources: code=%d out=%s", code, out)
	}
}

func TestRunBadUsage(t *testing.T) {
	cases := [][]string{
		{},
		{"call", "onlyserver"},
		{"tools"},
		{"resources"},
		{"read", "server"},
		{"bogus"},
	}
	for _, args := range cases {
		if code := run(args); code == 0 {
			t.Errorf("run(%v) should fail", args)
		}
	}
	if run([]string{"help"}) != 0 {
		t.Error("help should exit 0")
	}
}

func TestParseArgs(t *testing.T) {
	args, err := parseArgs([]string{"a=1", "b=x=y"})
	if err != nil || args["a"] != "1" || args["b"] != "x=y" {
		t.Fatalf("parseArgs: %v %v", args, err)
	}
	if _, err := parseArgs([]string{"noequals"}); err == nil {
		t.Fatal("malformed pair should error")
	}
}
