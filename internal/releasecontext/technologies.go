package releasecontext

import (
	"path"
	"sort"
	"strings"
)

// awsServiceNames maps a CloudFormation/service code to a friendly product name.
var awsServiceNames = map[string]string{
	"Lambda": "AWS Lambda", "ApiGateway": "Amazon API Gateway", "ApiGatewayV2": "Amazon API Gateway",
	"Events": "Amazon EventBridge", "Scheduler": "Amazon EventBridge Scheduler",
	"SQS": "Amazon SQS", "SNS": "Amazon SNS", "S3": "Amazon S3", "IAM": "AWS IAM",
	"DynamoDB": "Amazon DynamoDB", "EC2": "Amazon EC2", "Logs": "Amazon CloudWatch",
	"CloudWatch": "Amazon CloudWatch", "StepFunctions": "AWS Step Functions",
	"SecretsManager": "AWS Secrets Manager", "SSM": "AWS Systems Manager",
}

// langByExt maps a source extension to a language name.
var langByExt = map[string]string{
	".go": "Go", ".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
	".java": "Java", ".rb": "Ruby", ".rs": "Rust",
}

// goModuleTech maps a go.mod dependency substring to a technology.
var goModuleTech = []struct{ needle, name, category string }{
	{"aws-lambda-go", "AWS Lambda (Go runtime)", "Framework"},
	{"aws-sdk-go-v2", "AWS SDK for Go v2", "Library"},
	{"go-git", "go-git", "Library"},
	{"aws/aws-lambda-go/events", "API Gateway proxy events", "Library"},
}

// analyzeTechnologies builds the technology inventory from the file inventory
// and the CloudFormation services already parsed.
func analyzeTechnologies(files []RawFile, cfnServices []string) []Technology {
	acc := newTechAcc()

	langFiles := map[string]int{}
	for _, f := range files {
		lp := strings.ToLower(f.Path)
		ext := path.Ext(lp)
		if lang, ok := langByExt[ext]; ok {
			langFiles[lang]++
		}
		base := path.Base(lp)
		switch {
		case base == "dockerfile" || strings.HasPrefix(base, "docker-compose"):
			acc.add("Docker", "Tool", "high", "Docker/Compose files")
		case base == "makefile":
			acc.add("Make", "Tool", "high", "Makefile")
		case strings.HasPrefix(lp, "packer/") || ext == ".pkr.hcl":
			acc.add("HashiCorp Packer", "Deployment", "high", "packer/ build definitions")
		case strings.HasPrefix(lp, ".github/workflows/"):
			acc.add("GitHub Actions", "Deployment", "high", ".github/workflows")
		case strings.HasPrefix(lp, "infrastructure/") && (ext == ".yaml" || ext == ".yml"):
			acc.add("AWS CloudFormation", "Deployment", "high", "infrastructure/ templates")
		}
		if strings.Contains(lp, "ollama") {
			acc.add("Ollama", "Tool", "high", "local LLM runtime")
		}
		if strings.Contains(lp, "n8n") {
			acc.add("n8n", "Tool", "medium", "workflow orchestration")
		}
		// go.mod dependency scan.
		if base == "go.mod" && f.Content != "" {
			acc.add("Go modules", "Tool", "high", "go.mod")
			for _, m := range goModuleTech {
				if strings.Contains(f.Content, m.needle) {
					acc.add(m.name, m.category, "high", "go.mod dependency")
				}
			}
		}
	}

	for lang, n := range langFiles {
		conf := "medium"
		if n >= 5 {
			conf = "high"
		}
		acc.add(lang, "Language", conf, plural(n, "source file"))
	}
	for _, svc := range cfnServices {
		name := awsServiceNames[svc]
		if name == "" {
			name = "AWS " + svc
		}
		acc.add(name, "AWS Service", "high", "CloudFormation resource")
	}

	return acc.sorted()
}

type techAcc struct {
	order []string
	by    map[string]Technology
	rank  map[string]int
}

func newTechAcc() *techAcc {
	return &techAcc{by: map[string]Technology{}, rank: map[string]int{"low": 0, "medium": 1, "high": 2}}
}

func (a *techAcc) add(name, category, confidence, evidence string) {
	if ex, ok := a.by[name]; ok {
		if a.rank[confidence] > a.rank[ex.Confidence] {
			ex.Confidence = confidence
			ex.Evidence = evidence
			a.by[name] = ex
		}
		return
	}
	a.order = append(a.order, name)
	a.by[name] = Technology{Name: name, Category: category, Confidence: confidence, Evidence: evidence}
}

func (a *techAcc) sorted() []Technology {
	out := make([]Technology, 0, len(a.by))
	for _, n := range a.order {
		out = append(out, a.by[n])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func plural(n int, noun string) string {
	s := noun
	if n != 1 {
		s += "s"
	}
	return itoa(n) + " " + s
}
