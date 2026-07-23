package mcp

import (
	"context"
	"fmt"
	"sort"
)

// DBTable is an in-memory table: ordered columns and rows keyed by column name.
type DBTable struct {
	Columns []string
	Rows    []map[string]any
}

// DBSnapshot is an offline, read-only representation of a relational database
// (standing in for SQLite/PostgreSQL). Production wiring would populate it from a
// real connection behind the same server; the in-memory form keeps the package
// offline-testable and removes any SQL-injection surface — queries are
// structured, never string-concatenated.
type DBSnapshot struct {
	Engine string // e.g. "postgres", "sqlite"
	Tables map[string]DBTable
}

type databaseServer struct {
	BaseServer
	snap DBSnapshot
}

func newDatabaseServer(snap DBSnapshot) *databaseServer {
	if snap.Engine == "" {
		snap.Engine = "in-memory"
	}
	if snap.Tables == nil {
		snap.Tables = map[string]DBTable{}
	}
	return &databaseServer{snap: snap}
}

func databaseDescriptor() ServerDescriptor {
	return ServerDescriptor{
		ID:                 "database",
		Info:               ServerInfo{Name: "database", Title: "Database", Version: "1.0.0", ProtocolVersion: ProtocolVersion},
		Category:           CategoryDatabase,
		Capabilities:       Capabilities{Tools: true},
		RequiredPermission: PermReadOnly,
		Description:        "Read-only, structured access to relational tables (no raw SQL).",
	}
}

func (s *databaseServer) Info() ServerInfo           { return databaseDescriptor().Info }
func (s *databaseServer) Capabilities() Capabilities { return databaseDescriptor().Capabilities }
func (s *databaseServer) Category() Category         { return CategoryDatabase }

func (s *databaseServer) ListTools(context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "list_tables",
			Title:       "List tables",
			Description: "List all table names.",
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "describe_table",
			Title:       "Describe table",
			Description: "Return a table's column names and row count.",
			Params:      []ParamSpec{{Name: "table", Type: TypeString, Description: "Table name.", Required: true}},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
		{
			Name:        "query",
			Title:       "Query table",
			Description: "Return rows from a table with an optional equality filter and row limit. Structured input only — no raw SQL is accepted.",
			Params: []ParamSpec{
				{Name: "table", Type: TypeString, Description: "Table name.", Required: true},
				{Name: "filter_column", Type: TypeString, Description: "Column to filter on (optional).", Default: ""},
				{Name: "filter_value", Type: TypeString, Description: "Value the filter column must equal (optional).", Default: ""},
				{Name: "limit", Type: TypeInteger, Description: "Maximum rows to return (default 100).", Default: "100"},
			},
			Annotations: ToolAnnotations{ReadOnly: true, Idempotent: true},
			Permission:  PermReadOnly,
		},
	}, nil
}

func (s *databaseServer) CallTool(_ context.Context, name string, args map[string]any) (ToolResult, error) {
	switch name {
	case "list_tables":
		names := make([]string, 0, len(s.snap.Tables))
		for t := range s.snap.Tables {
			names = append(names, t)
		}
		sort.Strings(names)
		return jsonResult(map[string]any{"engine": s.snap.Engine, "tables": names}), nil
	case "describe_table":
		t, ok := s.snap.Tables[argString(args, "table")]
		if !ok {
			return errorResult("no such table: %s", argString(args, "table")), nil
		}
		return jsonResult(map[string]any{"columns": t.Columns, "rows": len(t.Rows)}), nil
	case "query":
		t, ok := s.snap.Tables[argString(args, "table")]
		if !ok {
			return errorResult("no such table: %s", argString(args, "table")), nil
		}
		col := argString(args, "filter_column")
		if col != "" && !contains(t.Columns, col) {
			return errorResult("no such column %q in %s", col, argString(args, "table")), nil
		}
		val := argString(args, "filter_value")
		limit := parseIntDefault(argString(args, "limit"), 100)
		out := make([]map[string]any, 0, len(t.Rows))
		for _, row := range t.Rows {
			if col != "" && fmt.Sprint(row[col]) != val {
				continue
			}
			out = append(out, row)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
		return jsonResult(out), nil
	default:
		return ToolResult{}, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
}
