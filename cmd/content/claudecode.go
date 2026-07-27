package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// claudeCodeModel drives the local Claude Code CLI in print mode
// (`claude -p`), so local content generation runs on the developer's Claude
// Code subscription instead of spending Anthropic API credits. It is a
// local-development provider only: the production worker has no `claude` CLI and
// never uses it.
//
// The prompt is passed on stdin (blog prompts are several KB — larger than is
// comfortable on argv), and the completion is read from stdout.
//
// File-writing tools are disabled: without them the CLI runs in the repo and
// tries to Write the artifact to disk, is denied, and narrates that refusal
// ("the write wasn't permitted, delivering inline…") straight into the article
// body. We only want the generated text, so we deny those tools and the model
// returns prose with no chat scaffolding.
type claudeCodeModel struct{ bin string }

func (m claudeCodeModel) Generate(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, m.bin, "-p",
		"--disallowedTools", "Write,Edit,NotebookEdit")
	cmd.Stdin = strings.NewReader(prompt)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return "", fmt.Errorf("claude -p failed: %w: %s", err, msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// newClaudeCodeModel locates the claude CLI on PATH.
func newClaudeCodeModel() (Model, error) {
	bin, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found on PATH (install Claude Code)")
	}
	return claudeCodeModel{bin: bin}, nil
}
