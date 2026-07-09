package archdiagram

import "sort"

// Diagram is one rendered diagram plus the prose the blog needs around it.
type Diagram struct {
	Type        string // "aws-solution", "component", "data-flow", "deployment", "cicd", "ai-workflow"
	Title       string
	Description string
	Purpose     string
	Mermaid     string
}

// primaryServices returns detected services eligible for the primary AWS diagram
// (High and Medium confidence only), id-sorted for deterministic output.
func (d Detection) primaryServices() []Service {
	out := make([]Service, 0, len(d.Services))
	for _, s := range d.Services {
		if s.ID == "github" {
			continue // GitHub is the source, drawn separately as the entry point
		}
		if s.Confidence == Low {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// awsSolution builds the primary AWS Solution Architecture diagram: source →
// compute inside an "AWS Cloud" boundary, with every High/Medium service the
// compute layer depends on. Only evidence-backed services appear.
func (d Detection) awsSolution() (Diagram, bool) {
	svcs := d.primaryServices()
	g := NewGraph("LR")

	g.AddNode(d.Source.ID, d.Source.Name)
	g.AddNode(d.Compute.ID, d.Compute.Name)

	cloudNodes := []string{d.Compute.ID}
	for _, s := range svcs {
		if s.ID == d.Compute.ID {
			continue
		}
		g.AddNode(s.ID, s.Name)
		cloudNodes = append(cloudNodes, s.ID)
		g.AddEdge(d.Compute.ID, s.ID, edgeVerb(s))
	}

	g.AddEdge(d.Source.ID, d.Compute.ID, "deploys to")
	g.AddCluster("aws", "AWS Cloud", cloudNodes...)

	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "aws-solution",
		Title:       "AWS Solution Architecture",
		Description: "Primary view of the repository mapped onto AWS services, showing the compute layer and the evidence-backed services it depends on.",
		Purpose:     "Give readers an at-a-glance, AWS-native picture of how the system is deployed.",
		Mermaid:     g.Mermaid(),
	}, true
}

// component builds the logical component diagram (source → compute → grouped
// backing capabilities), independent of AWS branding.
func (d Detection) component() (Diagram, bool) {
	g := NewGraph("LR")
	g.AddNode("client", "Client / Consumers")
	app := "app"
	g.AddNode(app, "Application ("+joinTop(d.Languages, 2)+")")
	g.AddEdge("client", app, "requests")

	for _, s := range d.primaryServices() {
		if s.ID == d.Compute.ID {
			continue
		}
		g.AddNode(s.ID, s.Logical)
		g.AddEdge(app, s.ID, "")
	}
	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "component",
		Title:       "Component Diagram",
		Description: "Logical components and their relationships, grouped by responsibility rather than by AWS service.",
		Purpose:     "Explain the application's internal structure before mapping it to infrastructure.",
		Mermaid:     g.Mermaid(),
	}, true
}

// dataFlow builds the request/data path through the system.
func (d Detection) dataFlow() (Diagram, bool) {
	g := NewGraph("LR")
	g.AddNode("client", "Client")
	prev := "client"
	if d.HasAPI {
		g.AddNode("api", "Amazon API Gateway")
		g.AddEdge(prev, "api", "HTTPS")
		prev = "api"
	}
	g.AddNode(d.Compute.ID, d.Compute.Name)
	g.AddEdge(prev, d.Compute.ID, "invoke")

	// Order the data stores so the flow reads consistently.
	for _, id := range []string{"rds", "dynamodb", "elasticache", "s3"} {
		if s := d.serviceByID(id); s != nil && s.Confidence != Low {
			g.AddNode(s.ID, s.Name)
			g.AddEdge(d.Compute.ID, s.ID, "read / write")
		}
	}
	if s := d.serviceByID("openclaw"); s != nil {
		g.AddNode(s.ID, s.Name)
		g.AddEdge(d.Compute.ID, s.ID, "inference")
	}
	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "data-flow",
		Title:       "Data Flow Diagram",
		Description: "How a request and its data move through the system from client to backing stores.",
		Purpose:     "Trace the runtime path of data for reviewers reasoning about latency and persistence.",
		Mermaid:     g.Mermaid(),
	}, true
}

// deployment builds the AWS deployment topology (VPC boundary around compute).
func (d Detection) deployment() (Diagram, bool) {
	g := NewGraph("TD")
	g.AddNode("users", "Users")
	g.AddNode(d.Compute.ID, d.Compute.Name)
	g.AddEdge("users", d.Compute.ID, "HTTPS")

	inVPC := []string{d.Compute.ID}
	for _, id := range []string{"elb", "rds", "elasticache", "vpc"} {
		if s := d.serviceByID(id); s != nil && s.Confidence != Low && s.ID != "vpc" {
			g.AddNode(s.ID, s.Name)
			g.AddEdge(d.Compute.ID, s.ID, "")
			inVPC = append(inVPC, s.ID)
		}
	}
	// Regional/managed services outside the VPC boundary.
	for _, id := range []string{"s3", "dynamodb", "sqs", "eventbridge", "secrets_manager", "cloudwatch"} {
		if s := d.serviceByID(id); s != nil && s.Confidence != Low {
			g.AddNode(s.ID, s.Name)
			g.AddEdge(d.Compute.ID, s.ID, "")
		}
	}
	g.AddCluster("vpc", "Amazon VPC", inVPC...)
	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "deployment",
		Title:       "Deployment Diagram",
		Description: "Deployment topology on AWS, showing what runs inside the VPC versus regional managed services.",
		Purpose:     "Clarify the network boundary and where each service is provisioned.",
		Mermaid:     g.Mermaid(),
	}, true
}

// cicd builds the CI/CD pipeline diagram. Drawn only when CI/CD is detected.
func (d Detection) cicd() (Diagram, bool) {
	if !d.HasCICD {
		return Diagram{}, false
	}
	g := NewGraph("LR")
	g.AddNode("dev", "Developer")
	g.AddNode("github", "GitHub")
	g.AddNode("actions", "GitHub Actions")
	g.AddNode(d.Compute.ID, d.Compute.Name+" (AWS)")
	g.AddEdge("dev", "github", "push")
	g.AddEdge("github", "actions", "trigger workflow")
	g.AddEdge("actions", d.Compute.ID, "deploy")
	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "cicd",
		Title:       "CI/CD Pipeline",
		Description: "The delivery path from commit to AWS deployment via GitHub Actions.",
		Purpose:     "Show how changes reach production.",
		Mermaid:     g.Mermaid(),
	}, true
}

// aiWorkflow builds the AI/LLM workflow diagram. Drawn only when AI is detected.
func (d Detection) aiWorkflow() (Diagram, bool) {
	if !d.HasAI {
		return Diagram{}, false
	}
	g := NewGraph("LR")
	g.AddNode("client", "Client request")
	g.AddNode(d.Compute.ID, d.Compute.Name)
	g.AddNode("openclaw", "OpenClaw on Amazon EC2")
	g.AddNode("response", "Response")
	g.AddEdge("client", d.Compute.ID, "prompt")
	g.AddEdge(d.Compute.ID, "openclaw", "inference")
	g.AddEdge("openclaw", d.Compute.ID, "completion")
	g.AddEdge(d.Compute.ID, "response", "return")
	if err := g.Validate(); err != nil {
		return Diagram{}, false
	}
	return Diagram{
		Type:        "ai-workflow",
		Title:       "AI Workflow",
		Description: "How an AI request flows through the application to local LLM inference on OpenClaw.",
		Purpose:     "Explain the AI path and reinforce that inference is self-hosted, not a managed API.",
		Mermaid:     g.Mermaid(),
	}, true
}

// buildDiagrams assembles every applicable diagram in blog order.
func (d Detection) buildDiagrams() []Diagram {
	var out []Diagram
	builders := []func() (Diagram, bool){
		d.awsSolution, d.component, d.dataFlow, d.deployment, d.cicd, d.aiWorkflow,
	}
	for _, b := range builders {
		if dg, ok := b(); ok {
			out = append(out, dg)
		}
	}
	return out
}

func (d Detection) serviceByID(id string) *Service {
	for i := range d.Services {
		if d.Services[i].ID == id {
			return &d.Services[i]
		}
	}
	return nil
}

// edgeVerb picks a short relationship label for a service on the primary diagram.
func edgeVerb(s Service) string {
	switch s.Logical {
	case "Object storage", "Relational database", "NoSQL database", "In-memory cache":
		return "reads / writes"
	case "Message queue", "Pub/sub messaging", "Event bus", "Scheduler":
		return "publishes"
	case "Secrets", "Configuration":
		return "reads"
	case "Monitoring & logging":
		return "emits to"
	case "LLM inference":
		return "inference"
	default:
		return "uses"
	}
}

func joinTop(list []string, n int) string {
	if len(list) == 0 {
		return "service"
	}
	if len(list) > n {
		list = list[:n]
	}
	out := ""
	for i, v := range list {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
