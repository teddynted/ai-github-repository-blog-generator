// Command platform inspects and exercises the extensibility platform (Milestone
// 19): the provider registry, plug-in discovery, capability-based selection, and
// the reference implementations for every abstraction.
//
// Usage:
//
//	# List every registered provider (auto-discovered plug-ins).
//	go run ./cmd/platform list
//
//	# List providers of one kind (llm|image|video|voice|translation|vector|
//	# publishing|mcp|workflow|content).
//	go run ./cmd/platform list llm
//
//	# Run a small end-to-end demo across the reference providers.
//	go run ./cmd/platform demo
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	p "github.com/teddynted/ai-github-repository-blog-generator/internal/platform"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	ctx := context.Background()
	reg := p.Default()
	cfg := p.EnvConfig{}

	switch args[0] {
	case "list":
		return listCmd(reg, args[1:])
	case "select":
		return selectCmd(ctx, reg, cfg, args[1:])
	case "demo":
		return demoCmd(ctx, reg, cfg)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", args[0])
		usage()
		return 2
	}
}

func listCmd(reg *p.Registry, args []string) int {
	var descs []p.Descriptor
	if len(args) > 0 {
		descs = reg.Descriptors(p.Kind(args[0]))
	} else {
		descs = reg.AllDescriptors()
	}
	b, _ := json.MarshalIndent(descs, "", "  ")
	fmt.Println(string(b))
	return 0
}

func selectCmd(ctx context.Context, reg *p.Registry, cfg p.ConfigSource, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: platform select <kind> [capability]")
		return 2
	}
	var opts []p.SelectOption
	if len(args) > 1 {
		opts = append(opts, p.WithCapability(p.Capability(args[1])))
	}
	prov, err := reg.Select(ctx, p.Kind(args[0]), cfg, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("selected %s/%s — capabilities %v — health %s\n", prov.Kind(), prov.ID(), prov.Capabilities(), prov.Health(ctx).Status)
	return 0
}

// demoCmd exercises the full abstraction surface via the reference providers,
// selecting each generically (never by concrete type) to prove the model.
func demoCmd(ctx context.Context, reg *p.Registry, cfg p.ConfigSource) int {
	fmt.Println("== Extensibility platform demo (reference providers) ==")

	llm, err := p.SelectTyped[p.LLMProvider](ctx, reg, p.KindLLM, cfg, p.WithCapability(p.CapTextGeneration))
	if err != nil {
		return fail(err)
	}
	resp, _ := llm.Generate(ctx, p.LLMRequest{Prompt: "Explain provider abstraction in one line."})
	fmt.Printf("LLM      → %s\n", resp.Text)

	img, err := p.SelectTyped[p.ImageProvider](ctx, reg, p.KindImage, cfg, p.WithCapability(p.CapThumbnail))
	if err != nil {
		return fail(err)
	}
	ir, _ := img.Generate(ctx, p.ImageRequest{Prompt: "cover", Purpose: p.CapThumbnail})
	fmt.Printf("Image    → %s %dx%d %s\n", ir.Format, ir.Width, ir.Height, ir.URI)

	tr, err := p.SelectTyped[p.Translator](ctx, reg, p.KindTranslation, cfg)
	if err != nil {
		return fail(err)
	}
	tres, _ := tr.Translate(ctx, p.TranslateRequest{Text: "hello", Target: p.LangFrench})
	fmt.Printf("Translate→ %q → %s (%s)\n", tres.Text, tres.Target, tres.Provider)

	vs, err := p.SelectTyped[p.VectorStore](ctx, reg, p.KindVector, cfg)
	if err != nil {
		return fail(err)
	}
	_ = vs.Upsert(ctx, []p.Vector{{ID: "a", Values: []float32{1, 0}}, {ID: "b", Values: []float32{0, 1}}})
	matches, _ := vs.Query(ctx, []float32{1, 0.1}, 1)
	fmt.Printf("Vector   → nearest %s (score %.3f)\n", matches[0].ID, matches[0].Score)

	mcp, err := p.SelectTyped[p.MCPClient](ctx, reg, p.KindMCP, cfg)
	if err != nil {
		return fail(err)
	}
	tools, _ := mcp.ListTools(ctx)
	fmt.Printf("MCP      → %d tools discovered\n", len(tools))

	pub, err := p.SelectTyped[p.PublishingProvider](ctx, reg, p.KindPublishing, cfg)
	if err != nil {
		return fail(err)
	}
	pr, _ := pub.Publish(ctx, p.Article{Title: "Extensible Platforms"})
	fmt.Printf("Publish  → %s (%s)\n", pr.URL, pr.Status)

	fmt.Printf("\nRegistered providers: %d across %d kinds\n", len(reg.AllDescriptors()), len(reg.Kinds()))
	return 0
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func usage() {
	fmt.Fprint(os.Stderr, `platform — extensibility platform (Milestone 19)

Usage:
  platform <command> [args]

Commands:
  list [kind]            List registered providers (optionally by kind)
  select <kind> [cap]    Select the best healthy provider for a kind
  demo                   Exercise every abstraction via reference providers

Kinds: llm image video voice translation vector publishing mcp workflow content
`)
}
