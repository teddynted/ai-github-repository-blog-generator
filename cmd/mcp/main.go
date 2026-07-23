// Command mcp is the operator CLI for the MCP integration layer (internal/mcp).
// It exercises the same client the pipeline uses to talk to MCP servers, so it
// doubles as a discovery/debugging tool and a smoke test of the tool ecosystem.
//
// It is a trusted, local, single-tenant tool: it uses the AllowAllAuthorizer by
// default so an operator can inspect everything. In a multi-caller deployment the
// library is wired with a PolicyAuthorizer instead (see docs/mcp.md).
//
// Usage:
//
//	go run ./cmd/mcp servers                         # list registered servers
//	go run ./cmd/mcp tools <server>                  # list a server's tools
//	go run ./cmd/mcp resources <server>              # list a server's resources
//	go run ./cmd/mcp discover                        # list every server + its tools
//	go run ./cmd/mcp call <server> <tool> k=v k=v…   # invoke a tool
//	go run ./cmd/mcp read <server> <uri>             # read a resource
//
// The default registry serves the repository itself: the filesystem server is
// rooted at ".", documentation at "docs".
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/mcp"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	ctx := context.Background()
	client := mcp.NewClient(mcp.DefaultRegistry, mcp.Config{Caller: "cli"},
		mcp.WithAuthorizer(mcp.AllowAllAuthorizer{}))

	switch args[0] {
	case "servers":
		return listServers(client)
	case "tools":
		if len(args) < 2 {
			return fail("tools requires a <server> argument")
		}
		return listTools(ctx, client, args[1])
	case "resources":
		if len(args) < 2 {
			return fail("resources requires a <server> argument")
		}
		return listResources(ctx, client, args[1])
	case "discover":
		return discover(ctx, client)
	case "call":
		if len(args) < 3 {
			return fail("call requires <server> <tool> [k=v ...]")
		}
		return callTool(ctx, client, args[1], args[2], args[3:])
	case "read":
		if len(args) < 3 {
			return fail("read requires <server> <uri>")
		}
		return readResource(ctx, client, args[1], args[2])
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		return fail("unknown command %q", args[0])
	}
}

func listServers(c *mcp.Client) int {
	for _, d := range c.Servers() {
		fmt.Printf("%-15s %-14s %-11s %s\n", d.ID, d.Category, d.RequiredPermission, d.Description)
	}
	return 0
}

func listTools(ctx context.Context, c *mcp.Client, server string) int {
	tools, err := c.ListTools(ctx, server)
	if err != nil {
		return fail("list tools: %v", err)
	}
	for _, t := range tools {
		flags := annotations(t.Annotations)
		fmt.Printf("%-20s [%s] %s\n", t.Name, t.Permission, t.Description)
		if flags != "" {
			fmt.Printf("  %s\n", flags)
		}
		for _, p := range t.Params {
			req := "optional"
			if p.Required {
				req = "required"
			}
			fmt.Printf("  - %s (%s, %s) %s\n", p.Name, p.Type, req, p.Description)
		}
	}
	return 0
}

func listResources(ctx context.Context, c *mcp.Client, server string) int {
	res, err := c.ListResources(ctx, server)
	if err != nil {
		return fail("list resources: %v", err)
	}
	if len(res) == 0 {
		fmt.Println("(no resources)")
		return 0
	}
	for _, r := range res {
		fmt.Printf("%-24s %-16s %s\n", r.URI, r.MimeType, r.Title)
	}
	return 0
}

func discover(ctx context.Context, c *mcp.Client) int {
	for _, d := range c.Servers() {
		fmt.Printf("# %s (%s)\n", d.ID, d.Category)
		tools, err := c.ListTools(ctx, d.ID)
		if err != nil {
			fmt.Printf("  error: %v\n", err)
			continue
		}
		for _, t := range tools {
			fmt.Printf("  %-20s %s\n", t.Name, t.Description)
		}
	}
	return 0
}

func callTool(ctx context.Context, c *mcp.Client, server, tool string, kv []string) int {
	args, err := parseArgs(kv)
	if err != nil {
		return fail("%v", err)
	}
	res, err := c.CallTool(ctx, server, tool, args)
	if err != nil {
		return fail("call: %v", err)
	}
	printResult(res)
	if res.IsError {
		return 1
	}
	return 0
}

func readResource(ctx context.Context, c *mcp.Client, server, uri string) int {
	rc, err := c.ReadResource(ctx, server, uri)
	if err != nil {
		return fail("read: %v", err)
	}
	fmt.Print(rc.Text)
	if !strings.HasSuffix(rc.Text, "\n") {
		fmt.Println()
	}
	return 0
}

func printResult(res mcp.ToolResult) {
	for _, c := range res.Content {
		switch c.Kind {
		case mcp.ContentJSON:
			b, _ := json.MarshalIndent(c.JSON, "", "  ")
			fmt.Println(string(b))
		default:
			fmt.Println(c.Text)
		}
	}
}

// parseArgs turns "k=v" pairs into a tool argument map. Values are kept as
// strings; the server's validation coerces them per the tool's ParamSpecs.
func parseArgs(kv []string) (map[string]any, error) {
	args := map[string]any{}
	for _, pair := range kv {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("argument %q is not in k=v form", pair)
		}
		args[k] = v
	}
	return args, nil
}

func annotations(a mcp.ToolAnnotations) string {
	var f []string
	if a.ReadOnly {
		f = append(f, "read-only")
	}
	if a.Destructive {
		f = append(f, "destructive")
	}
	if a.Idempotent {
		f = append(f, "idempotent")
	}
	return strings.Join(f, ", ")
}

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	return 1
}

func usage() {
	fmt.Fprint(os.Stderr, `mcp — operator CLI for the MCP integration layer

Commands:
  servers                      list registered servers
  tools <server>               list a server's tools
  resources <server>           list a server's resources
  discover                     list every server and its tools
  call <server> <tool> [k=v…]  invoke a tool
  read <server> <uri>          read a resource

Examples:
  go run ./cmd/mcp servers
  go run ./cmd/mcp call mermaid flowchart steps='Build|Test|Ship'
  go run ./cmd/mcp call cloudformation validate_template template="$(cat infrastructure/worker.yaml)"
`)
}
