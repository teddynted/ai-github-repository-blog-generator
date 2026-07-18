package releasecontext

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// isCFNTemplatePath is a cheap path heuristic (used before content is loaded)
// for CloudFormation templates that live outside infrastructure/.
func isCFNTemplatePath(lp string) bool {
	ext := path.Ext(lp)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" && ext != ".template" {
		return false
	}
	return strings.Contains(lp, "cloudformation") || strings.Contains(lp, "cfn") ||
		strings.Contains(lp, "template") || strings.Contains(lp, "stack")
}

// isCFNTemplate reports whether file content is a CloudFormation template.
func isCFNTemplate(content string) bool {
	return strings.Contains(content, "AWSTemplateFormatVersion") ||
		(strings.Contains(content, "\nResources:") && strings.Contains(content, "Type: AWS::")) ||
		(strings.HasPrefix(content, "Resources:") && strings.Contains(content, "Type: AWS::"))
}

// analyzeCloudFormation parses every CloudFormation template in the inventory
// into a structured infrastructure summary.
func analyzeCloudFormation(files []RawFile) CloudFormationAnalysis {
	var a CloudFormationAnalysis
	services := map[string]bool{}
	for _, f := range files {
		if f.Content == "" || !isCFNTemplate(f.Content) {
			continue
		}
		a.Templates = append(a.Templates, f.Path)
		res, params, outputs := parseTemplate(f.Path, f.Content)
		a.Resources = append(a.Resources, res...)
		a.Parameters = append(a.Parameters, params...)
		a.Outputs = append(a.Outputs, outputs...)
		for _, r := range res {
			if r.Service != "" {
				services[r.Service] = true
			}
		}
	}
	sort.Strings(a.Templates)
	sort.Strings(a.Parameters)
	sort.Strings(a.Outputs)
	sort.Slice(a.Resources, func(i, j int) bool { return a.Resources[i].LogicalID < a.Resources[j].LogicalID })

	for s := range services {
		a.Services = append(a.Services, s)
	}
	sort.Strings(a.Services)

	a.Counts = countResources(a)
	a.Summary = summarizeCFN(a)
	return a
}

// parseTemplate does a focused, indentation-aware parse of a CFN template. It is
// intentionally lightweight (no YAML dependency): CloudFormation sections are
// top-level keys, and their members are the 2-space-indented keys beneath them.
func parseTemplate(tmpl, content string) (resources []CFNResource, params, outputs []string) {
	section := ""   // current top-level section: Resources|Parameters|Outputs|...
	logicalID := "" // current resource logical id (under Resources)
	for _, line := range strings.Split(content, "\n") {
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			logicalID = ""
			continue
		}
		// A member of a section: exactly 2-space indent, "Name:".
		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			name := strings.TrimSuffix(trimmed, ":")
			switch section {
			case "Parameters":
				params = append(params, name)
			case "Outputs":
				outputs = append(outputs, name)
			case "Resources":
				logicalID = name
				resources = append(resources, CFNResource{LogicalID: name, Template: path.Base(tmpl)})
			}
			continue
		}
		// The resource Type line: attach it to the current logical id.
		if section == "Resources" && logicalID != "" && strings.HasPrefix(trimmed, "Type:") {
			typ := strings.TrimSpace(strings.TrimPrefix(trimmed, "Type:"))
			typ = strings.Trim(typ, "'\"")
			for i := range resources {
				if resources[i].LogicalID == logicalID && resources[i].Type == "" {
					resources[i].Type = typ
					resources[i].Service = serviceFromType(typ)
					resources[i].Category = categoryForCFNType(typ)
					break
				}
			}
		}
	}
	return resources, params, outputs
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

// serviceFromType turns "AWS::S3::Bucket" into "S3".
func serviceFromType(t string) string {
	parts := strings.Split(t, "::")
	if len(parts) >= 2 && parts[0] == "AWS" {
		return parts[1]
	}
	return ""
}

// categoryForCFNType buckets a resource type for the infrastructure summary.
func categoryForCFNType(t string) string {
	svc := serviceFromType(t)
	switch svc {
	case "IAM":
		return "IAM"
	case "Lambda", "ApiGateway", "ApiGatewayV2", "StepFunctions", "Scheduler":
		return "Serverless"
	case "Events":
		return "Messaging"
	case "SQS", "SNS":
		return "Messaging"
	case "S3", "DynamoDB", "EFS", "SecretsManager", "SSM":
		return "Storage"
	case "EC2":
		if strings.Contains(t, "Instance") || strings.Contains(t, "Volume") || strings.Contains(t, "LaunchTemplate") {
			return "Compute"
		}
		return "Networking"
	case "AutoScaling", "ECS", "Batch":
		return "Compute"
	case "ElasticLoadBalancingV2", "Route53", "CloudFront":
		return "Networking"
	case "Logs", "CloudWatch":
		return "Observability"
	default:
		return "Other"
	}
}

func countResources(a CloudFormationAnalysis) CFNCounts {
	c := CFNCounts{Resources: len(a.Resources), Parameters: len(a.Parameters), Outputs: len(a.Outputs)}
	for _, r := range a.Resources {
		switch r.Category {
		case "IAM":
			c.IAM++
		case "Serverless":
			c.Serverless++
		case "Compute":
			c.Compute++
		case "Storage":
			c.Storage++
		case "Networking":
			c.Networking++
		case "Messaging":
			c.Messaging++
		}
	}
	return c
}

func summarizeCFN(a CloudFormationAnalysis) string {
	if len(a.Templates) == 0 {
		return "No CloudFormation templates were found."
	}
	return fmt.Sprintf(
		"%d CloudFormation template(s) define %d resources across %s (%d serverless, %d compute, %d storage, %d networking, %d IAM, %d messaging), with %d parameters and %d outputs.",
		len(a.Templates), a.Counts.Resources, strings.Join(a.Services, ", "),
		a.Counts.Serverless, a.Counts.Compute, a.Counts.Storage, a.Counts.Networking, a.Counts.IAM, a.Counts.Messaging,
		a.Counts.Parameters, a.Counts.Outputs,
	)
}
