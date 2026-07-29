package releasecontext

import (
	"fmt"
	"sort"
	"strings"
)

// analyzeArchitecture synthesizes an architectural overview from the structure,
// CloudFormation, technology, and documentation analyses already on rc.
func analyzeArchitecture(rc *ReleaseContext) Architecture {
	var arch Architecture

	// AWS services: union of CFN services (friendly) and AWS-Service technologies.
	svc := map[string]bool{}
	for _, s := range rc.CloudFormation.Services {
		if name := awsServiceNames[s]; name != "" {
			svc[name] = true
		} else {
			svc[s] = true
		}
	}
	for _, t := range rc.Technologies {
		if t.Category == "AWS Service" {
			svc[t.Name] = true
		}
	}
	for s := range svc {
		arch.AWSServices = append(arch.AWSServices, s)
	}
	sort.Strings(arch.AWSServices)

	arch.Components = deriveComponents(rc)
	arch.EventDrivenFlows = deriveFlows(svc)
	arch.DeploymentTopology = deriveTopology(rc)
	arch.Scalability = deriveScalability(svc)
	arch.Reliability = deriveReliability(svc)
	arch.Security = deriveSecurity(rc, svc)
	arch.Overview = archOverview(rc, arch)
	arch.Insights = archInsights(rc, arch)
	return arch
}

func deriveComponents(rc *ReleaseContext) []ArchitectureComponent {
	var comps []ArchitectureComponent
	dirs := map[string]bool{}
	for _, d := range rc.RepositoryStructure.Directories {
		dirs[strings.TrimSuffix(d.Path, "/")] = true
	}
	if dirs["lambdas"] {
		comps = append(comps, ArchitectureComponent{Name: "Lambda functions", Responsibility: "Serverless entry points (webhook, registration, manual trigger, scheduler)", Kind: "lambda"})
	}
	if dirs["cmd"] {
		comps = append(comps, ArchitectureComponent{Name: "CLI / worker binaries", Responsibility: "Instance worker and operational CLIs", Kind: "compute"})
	}
	if dirs["internal"] {
		comps = append(comps, ArchitectureComponent{Name: "Domain packages", Responsibility: "Business logic behind small, testable ports", Kind: "library"})
	}
	if dirs["infrastructure"] {
		comps = append(comps, ArchitectureComponent{Name: "CloudFormation stacks", Responsibility: "Provision the AWS topology as code", Kind: "iac"})
	}
	// Category-driven components from CFN.
	if rc.CloudFormation.Counts.Messaging > 0 {
		comps = append(comps, ArchitectureComponent{Name: "Event bus & queue", Responsibility: "Durable, decoupled event delivery", Kind: "queue"})
	}
	if rc.CloudFormation.Counts.Storage > 0 {
		comps = append(comps, ArchitectureComponent{Name: "Managed storage", Responsibility: "State, metadata, and secrets", Kind: "store"})
	}
	return comps
}

func deriveFlows(svc map[string]bool) []string {
	var flows []string
	if svc["Amazon EventBridge"] && svc["Amazon SQS"] {
		flows = append(flows, "API Gateway/Webhook → EventBridge → SQS → worker")
	}
	if svc["Amazon EventBridge Scheduler"] {
		flows = append(flows, "EventBridge Scheduler → Lambda → EC2 start/stop")
	}
	return flows
}

func deriveTopology(rc *ReleaseContext) string {
	c := rc.CloudFormation.Counts
	if len(rc.CloudFormation.Templates) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"A modular, multi-stack CloudFormation deployment: %d serverless, %d compute, %d storage, and %d networking resources, with least-privilege IAM (%d roles/policies).",
		c.Serverless, c.Compute, c.Storage, c.Networking, c.IAM,
	)
}

func deriveScalability(svc map[string]bool) string {
	if svc["Amazon SQS"] {
		return "Event ingestion scales independently of compute: bursts buffer in SQS and drain when the worker is available, decoupling throughput from instance uptime."
	}
	if svc["AWS Lambda"] {
		return "Stateless Lambda entry points scale horizontally with request volume."
	}
	return ""
}

func deriveReliability(svc map[string]bool) string {
	var parts []string
	if svc["Amazon SQS"] {
		parts = append(parts, "durable SQS buffering with a dead-letter queue")
	}
	if svc["AWS Lambda"] {
		parts = append(parts, "managed, self-healing Lambda execution")
	}
	if len(parts) == 0 {
		return ""
	}
	return "Reliability comes from " + strings.Join(parts, " and ") + "."
}

func deriveSecurity(rc *ReleaseContext, svc map[string]bool) string {
	var parts []string
	if rc.CloudFormation.Counts.IAM > 0 {
		parts = append(parts, "least-privilege IAM roles")
	}
	if svc["AWS Secrets Manager"] {
		parts = append(parts, "secrets stored in AWS Secrets Manager (never in code)")
	}
	if containsString(patternNames(rc), "Least-privilege security") {
		parts = append(parts, "HMAC-verified webhooks")
	}
	if len(parts) == 0 {
		return ""
	}
	return "Security is enforced through " + strings.Join(parts, ", ") + "."
}

func archOverview(rc *ReleaseContext, arch Architecture) string {
	if len(arch.AWSServices) == 0 && len(arch.Components) == 0 {
		return rc.RepositoryStructure.Overview
	}
	lead := fmt.Sprintf("An event-driven, AWS-native system built from %d major components", len(arch.Components))
	if svcs := topN(arch.AWSServices, 6); len(svcs) > 0 {
		lead += " on " + strings.Join(svcs, ", ")
	}
	return strings.TrimSpace(lead + ". " + rc.RepositoryStructure.Overview)
}

func archInsights(rc *ReleaseContext, arch Architecture) []string {
	var ins []string
	if len(arch.EventDrivenFlows) > 0 {
		ins = append(ins, "The pipeline is fully event-driven: "+strings.Join(arch.EventDrivenFlows, "; ")+".")
	}
	if rc.CloudFormation.Counts.Serverless > 0 && rc.CloudFormation.Counts.Compute > 0 {
		ins = append(ins, "It blends serverless ingestion with an on-demand compute host, bounding cost while keeping the front door always available.")
	}
	if len(rc.Mermaid) > 0 {
		ins = append(ins, fmt.Sprintf("%d architecture diagram(s) document the system, aiding onboarding and technical writing.", len(rc.Mermaid)))
	}
	return ins
}

func patternNames(rc *ReleaseContext) []string { return rc.Documentation.ArchitecturePatterns }

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
