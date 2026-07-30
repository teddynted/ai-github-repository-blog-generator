// Package awsdiagram renders a release-specific AWS architecture diagram as a
// self-contained SVG. It parses the grounded "## Connections" table of an
// architecture-diagram-spec.md into a directed graph, then lays it out left to
// right in AWS-service-coloured boxes with solid (data/control flow) and dashed
// (governance/observability/polling) edges. It needs no external assets, no
// network, and produces deterministic output for the same input.
package awsdiagram

import "strings"

// Category is a coarse AWS service family used for node colour and glyph.
type Category string

const (
	CatCompute       Category = "compute"
	CatServerless    Category = "serverless"
	CatStorage       Category = "storage"
	CatIntegration   Category = "integration"
	CatObservability Category = "observability"
	CatSecurity      Category = "security"
	CatExternal      Category = "external"
	CatOther         Category = "other"
)

// Node is a box in the diagram.
type Node struct {
	ID       string
	Label    string
	Category Category
	// Plane is the workflow phase the node belongs to (e.g. "Build Plane",
	// "Runtime Plane", "Cross-cutting"), taken from the spec's Components
	// grouping. Empty when the spec does not group components into planes.
	Plane string
}

// Edge is a directed connection. Dashed marks a supporting relationship
// (governance/observability/out-of-band); Emphasis marks the primary edge (the
// release's central relationship) so the renderer can highlight it.
type Edge struct {
	From, To string
	Label    string
	Dashed   bool
	Emphasis bool
}

// Diagram is the parsed graph ready to render.
type Diagram struct {
	Title string
	Nodes []Node
	Edges []Edge
}

// catFill / catStroke give each category its colours (AWS-style palette).
var catStyle = map[Category][2]string{
	CatCompute:       {"#ED7100", "#B25900"},
	CatServerless:    {"#ED7100", "#B25900"},
	CatStorage:       {"#7AA116", "#587310"},
	CatIntegration:   {"#E7157B", "#A50E58"},
	CatObservability: {"#E7157B", "#A50E58"},
	CatSecurity:      {"#DD344C", "#A61F33"},
	CatExternal:      {"#232F3E", "#111820"},
	CatOther:         {"#5A6B86", "#3C485C"},
}

func (c Category) fill() string {
	if s, ok := catStyle[c]; ok {
		return s[0]
	}
	return catStyle[CatOther][0]
}

func (c Category) stroke() string {
	if s, ok := catStyle[c]; ok {
		return s[1]
	}
	return catStyle[CatOther][1]
}

// glyph is a short category tag drawn in the node's corner badge.
func (c Category) glyph() string {
	switch c {
	case CatCompute:
		return "EC2"
	case CatServerless:
		return "λ"
	case CatStorage:
		return "S3"
	case CatIntegration:
		return "EVT"
	case CatObservability:
		return "CW"
	case CatSecurity:
		return "IAM"
	case CatExternal:
		return "EXT"
	default:
		return "AWS"
	}
}

// categoryFor infers a node's category from its label.
func categoryFor(label string) Category {
	l := strings.ToLower(label)
	switch {
	case has(l, "lambda"):
		return CatServerless
	case has(l, "ec2", "instance", "builder", "host", "runtime"):
		return CatCompute
	case has(l, "s3", "bucket", "ami", "snapshot", "image", "efs"):
		return CatStorage
	case has(l, "eventbridge", "scheduler", "sqs", "sns", "queue", "event"):
		return CatIntegration
	case has(l, "cloudwatch", "logs", "metric", "observ"):
		return CatObservability
	case has(l, "iam", "role", "policy", "secret", "kms"):
		return CatSecurity
	case has(l, "github", "dev", "orchestrator", "external", "pipeline"):
		return CatExternal
	default:
		return CatOther
	}
}

// categoryForType maps a component's declared Type (from the Components section)
// to a category, falling back to label inference when the type is unknown. The
// label is still consulted for storage-vs-compute nuance (an EC2-backed AMI is
// a storage artifact, not compute).
func categoryForType(typ, label string) Category {
	t := strings.ToLower(typ)
	l := strings.ToLower(label)
	switch {
	// Scheduling/eventing before serverless: "serverless (scheduler)" backed by
	// EventBridge Scheduler is integration, not a Lambda-style serverless node.
	case has(t, "schedul", "messaging", "integration", "queue") || has(l, "eventbridge", "scheduler", "sqs", "sns"):
		return CatIntegration
	case has(t, "serverless", "lambda", "function"):
		return CatServerless
	case has(t, "compute"):
		return CatCompute
	case has(t, "storage", "image", "artifact"):
		return CatStorage
	case has(t, "messaging", "schedul", "integration", "event", "queue"):
		return CatIntegration
	case has(t, "observab", "monitor", "logging", "telemetry"):
		return CatObservability
	case has(t, "iam", "security", "identity", "access"):
		return CatSecurity
	case has(t, "external"):
		return CatExternal
	case t == "":
		return categoryFor(label)
	default:
		return categoryFor(typ + " " + label)
	}
}

// isSupportCat reports whether a category is cross-cutting (governance or
// observability), which the renderer draws with a dashed relationship.
func isSupportCat(c Category) bool {
	return c == CatSecurity || c == CatObservability
}

func has(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
