package archspec

import (
	"fmt"
	"strings"
)

// assembleOffline builds the specification deterministically from the Release
// Context, with no model. It emits the same section contract as the prompt and
// draws every line from evidence — nothing is invented — so an offline or
// degraded run still produces an honest, renderer-ready specification.
func assembleOffline(pkg ReleasePackage) string {
	rctx := pkg.Context
	cfn := rctx.CloudFormation
	arch := rctx.Architecture

	var b strings.Builder
	b.WriteString(specHeading + "\n\n")

	// --- Diagram Metadata ---
	b.WriteString("## Diagram Metadata\n")
	fmt.Fprintf(&b, "- Title: %s\n", specTitle(pkg))
	fmt.Fprintf(&b, "- Purpose: Represent the AWS architecture %s implements, as captured from repository evidence.\n", rctx.Repository.FullName)
	if topic := releaseTopic(pkg.Blog.Title); topic != "" {
		fmt.Fprintf(&b, "- Primary Engineering Topic: %s\n", topic)
	}
	fmt.Fprintf(&b, "- Repository: %s\n", rctx.Repository.FullName)
	fmt.Fprintf(&b, "- Milestone: %s\n", firstNonEmpty(rctx.Release.Tag, rctx.Release.Name))
	fmt.Fprintf(&b, "- Diagram Version: %s\n", DiagramVersion)
	b.WriteString("- Output Formats: SVG\n\n")

	// --- Components ---
	b.WriteString("## Components\n")
	if len(cfn.Resources) > 0 {
		for _, r := range cfn.Resources {
			fmt.Fprintf(&b, "- **%s** — Type: %s; AWS Service: %s; Purpose: provisioned by CloudFormation as %s; Repository Evidence: %s (%s).\n",
				r.LogicalID, dash(componentType(r.Category)), dash(r.Service), dash(r.Type), dash(r.Template), dash(r.Type))
		}
	} else {
		for _, svc := range arch.AWSServices {
			fmt.Fprintf(&b, "- **%s** — Type: aws-service; AWS Service: %s; Repository Evidence: detected in the architecture analysis.\n", svc, svc)
		}
	}
	for _, c := range arch.Components {
		kind := c.Kind
		if kind == "" {
			kind = "component"
		}
		fmt.Fprintf(&b, "- **%s** — Type: %s; Purpose: %s; Repository Evidence: architecture components.\n", c.Name, kind, c.Responsibility)
	}
	b.WriteString("\n")

	// --- Connections ---
	b.WriteString("## Connections\n")
	wrote := false
	for _, f := range arch.EventDrivenFlows {
		fmt.Fprintf(&b, "- %s (Direction: forward; Repository Evidence: event-driven flows).\n", f)
		wrote = true
	}
	for _, d := range rctx.Mermaid {
		for _, e := range d.Edges {
			label := "flow"
			if e.Label != "" {
				label = e.Label
			}
			fmt.Fprintf(&b, "- %s → %s (Purpose: %s; Repository Evidence: %s).\n", e.From, e.To, label, dash(d.Source))
			wrote = true
		}
	}
	if !wrote {
		b.WriteString("- No explicit connections were captured in the repository evidence.\n")
	}
	b.WriteString("\n")

	// --- Security ---
	b.WriteString("## Security\n")
	secWrote := false
	if strings.TrimSpace(arch.Security) != "" {
		fmt.Fprintf(&b, "- %s\n", arch.Security)
		secWrote = true
	}
	if cfn.Counts.IAM > 0 {
		fmt.Fprintf(&b, "- IAM: %d least-privilege role/policy resource(s) defined in CloudFormation.\n", cfn.Counts.IAM)
		secWrote = true
	}
	if cfn.Counts.Networking > 0 {
		fmt.Fprintf(&b, "- Networking: %d networking resource(s) define the network boundary.\n", cfn.Counts.Networking)
		secWrote = true
	}
	if !secWrote {
		b.WriteString("- No security boundaries were captured in the repository evidence.\n")
	}
	b.WriteString("\n")

	// --- Operational Flow ---
	b.WriteString("## Operational Flow\n")
	if len(arch.EventDrivenFlows) > 0 {
		for i, f := range arch.EventDrivenFlows {
			fmt.Fprintf(&b, "%d. %s\n", i+1, f)
		}
	} else if strings.TrimSpace(arch.Overview) != "" {
		fmt.Fprintf(&b, "- %s\n", arch.Overview)
	} else {
		b.WriteString("- The end-to-end flow was not captured in the repository evidence.\n")
	}
	b.WriteString("\n")

	// --- Failure Handling ---
	b.WriteString("## Failure Handling\n")
	if cfn.Counts.Messaging > 0 {
		b.WriteString("- Messaging resources provide buffering/decoupling between components; consult the templates for dead-letter/retry configuration.\n")
	} else {
		b.WriteString("- Retry, dead-letter, and timeout behaviour were not captured in the repository evidence.\n")
	}
	b.WriteString("\n")

	// --- Rendering Notes ---
	b.WriteString("## Rendering Notes\n")
	if cfn.Counts.Networking > 0 {
		b.WriteString("- Group networked compute inside a VPC boundary.\n")
	}
	b.WriteString("- Lay the diagram out left-to-right following the operational flow.\n")
	b.WriteString("- Treat serverless/compute resources as primary components and storage/messaging/IAM as supporting components.\n")
	b.WriteString("- Render as SVG with a landscape viewBox (roughly 3:2); scale icons uniformly.\n")

	return b.String()
}

// componentType maps a CloudFormation category to a diagram component type.
func componentType(category string) string {
	switch strings.ToLower(category) {
	case "compute":
		return "compute"
	case "serverless":
		return "serverless"
	case "storage":
		return "storage"
	case "networking":
		return "networking"
	case "iam":
		return "iam"
	case "messaging":
		return "messaging"
	default:
		return category
	}
}
