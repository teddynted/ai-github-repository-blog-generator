package archdiagram

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/processing"
)

// Confidence classifies how strongly a component is supported by evidence.
type Confidence string

const (
	// High: the service is named directly in repository files (IaC, SDK client).
	High Confidence = "High"
	// Medium: strongly inferred from configuration (e.g. a Postgres driver → RDS).
	Medium Confidence = "Medium"
	// Low: possible but uncertain; excluded from the primary AWS diagram.
	Low Confidence = "Low"
)

// rank orders confidence for "keep the strongest" merges.
func (c Confidence) rank() int {
	switch c {
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	default:
		return 0
	}
}

// Service is a detected AWS service mapped from a logical component.
type Service struct {
	ID         string     // stable node id, e.g. "s3"
	Name       string     // e.g. "Amazon S3"
	Logical    string     // logical role, e.g. "Object storage"
	Confidence Confidence // High / Medium / Low
	Evidence   []string   // repository evidence, e.g. "infra/main.tf: aws_s3_bucket"
}

// Detection is the architectural interpretation of a repository: the AWS
// services it maps to, plus coarse capability flags used to decide which
// secondary diagrams are worth drawing.
type Detection struct {
	Compute  Service   // the app's primary compute (EC2 by deployment default)
	Source   Service   // GitHub (always: it is a GitHub repository)
	Services []Service // all detected services incl. Compute/Source, id-sorted

	HasAI         bool
	HasAPI        bool
	HasDatabase   bool
	HasCache      bool
	HasQueue      bool
	HasEvents     bool
	HasContainers bool
	HasKubernetes bool
	HasCICD       bool
	Languages     []string
}

type svcDef struct{ name, logical string }

// Terraform resource prefixes → AWS service (direct evidence → High).
var terraformServices = map[string]svcDef{
	"aws_s3_bucket":               {"Amazon S3", "Object storage"},
	"aws_dynamodb_table":          {"Amazon DynamoDB", "NoSQL database"},
	"aws_db_instance":             {"Amazon RDS", "Relational database"},
	"aws_rds_cluster":             {"Amazon RDS", "Relational database"},
	"aws_lambda_function":         {"AWS Lambda", "Serverless compute"},
	"aws_apigatewayv2":            {"Amazon API Gateway", "API gateway"},
	"aws_api_gateway_rest_api":    {"Amazon API Gateway", "API gateway"},
	"aws_sqs_queue":               {"Amazon SQS", "Message queue"},
	"aws_sns_topic":               {"Amazon SNS", "Pub/sub messaging"},
	"aws_cloudwatch_event":        {"Amazon EventBridge", "Event bus"},
	"aws_scheduler_schedule":      {"Amazon EventBridge", "Scheduler"},
	"aws_cloudwatch":              {"Amazon CloudWatch", "Monitoring & logging"},
	"aws_secretsmanager_secret":   {"AWS Secrets Manager", "Secrets"},
	"aws_elasticache":             {"Amazon ElastiCache", "In-memory cache"},
	"aws_ecs_service":             {"Amazon ECS", "Container orchestration"},
	"aws_ecs_cluster":             {"Amazon ECS", "Container orchestration"},
	"aws_eks_cluster":             {"Amazon EKS", "Kubernetes"},
	"aws_cloudfront_distribution": {"Amazon CloudFront", "CDN"},
	"aws_route53":                 {"Amazon Route 53", "DNS"},
	"aws_lb":                      {"Elastic Load Balancing", "Load balancer"},
	"aws_alb":                     {"Elastic Load Balancing", "Load balancer"},
	"aws_vpc":                     {"Amazon VPC", "Network isolation"},
	"aws_instance":                {"Amazon EC2", "Compute"},
	"aws_iam_role":                {"AWS IAM", "Identity & access"},
	"aws_wafv2":                   {"AWS WAF", "Web application firewall"},
	"aws_ssm_parameter":           {"AWS Systems Manager", "Configuration"},
}

// CloudFormation resource-type prefixes → AWS service (direct evidence → High).
var cfnServices = map[string]svcDef{
	"AWS::S3::Bucket":               {"Amazon S3", "Object storage"},
	"AWS::DynamoDB::Table":          {"Amazon DynamoDB", "NoSQL database"},
	"AWS::RDS::":                    {"Amazon RDS", "Relational database"},
	"AWS::Lambda::Function":         {"AWS Lambda", "Serverless compute"},
	"AWS::ApiGateway":               {"Amazon API Gateway", "API gateway"},
	"AWS::SQS::Queue":               {"Amazon SQS", "Message queue"},
	"AWS::SNS::Topic":               {"Amazon SNS", "Pub/sub messaging"},
	"AWS::Events::":                 {"Amazon EventBridge", "Event bus"},
	"AWS::Scheduler::":              {"Amazon EventBridge", "Scheduler"},
	"AWS::SecretsManager::":         {"AWS Secrets Manager", "Secrets"},
	"AWS::ElastiCache::":            {"Amazon ElastiCache", "In-memory cache"},
	"AWS::ECS::":                    {"Amazon ECS", "Container orchestration"},
	"AWS::EKS::":                    {"Amazon EKS", "Kubernetes"},
	"AWS::CloudFront::":             {"Amazon CloudFront", "CDN"},
	"AWS::Route53::":                {"Amazon Route 53", "DNS"},
	"AWS::ElasticLoadBalancingV2::": {"Elastic Load Balancing", "Load balancer"},
	"AWS::EC2::VPC":                 {"Amazon VPC", "Network isolation"},
	"AWS::EC2::Instance":            {"Amazon EC2", "Compute"},
	"AWS::IAM::":                    {"AWS IAM", "Identity & access"},
	"AWS::WAFv2::":                  {"AWS WAF", "Web application firewall"},
	"AWS::Logs::":                   {"Amazon CloudWatch", "Monitoring & logging"},
	"AWS::CloudWatch::":             {"Amazon CloudWatch", "Monitoring & logging"},
	"AWS::SSM::Parameter":           {"AWS Systems Manager", "Configuration"},
}

// AWS SDK client package names → AWS service (a used client → High).
var sdkServices = map[string]svcDef{
	"@aws-sdk/client-s3":              {"Amazon S3", "Object storage"},
	"@aws-sdk/client-dynamodb":        {"Amazon DynamoDB", "NoSQL database"},
	"@aws-sdk/client-sqs":             {"Amazon SQS", "Message queue"},
	"@aws-sdk/client-sns":             {"Amazon SNS", "Pub/sub messaging"},
	"@aws-sdk/client-lambda":          {"AWS Lambda", "Serverless compute"},
	"@aws-sdk/client-secrets-manager": {"AWS Secrets Manager", "Secrets"},
	"@aws-sdk/client-eventbridge":     {"Amazon EventBridge", "Event bus"},
	"@aws-sdk/client-cloudwatch":      {"Amazon CloudWatch", "Monitoring & logging"},
	"aws-sdk/clients/s3":              {"Amazon S3", "Object storage"},
}

// Runtime dependency hints → AWS service, inferred (Medium): the app talks to
// this kind of backing service, which on AWS is provided by the mapped service.
var dependencyHints = map[string]svcDef{
	"psycopg2":    {"Amazon RDS", "Relational database"},
	"psycopg":     {"Amazon RDS", "Relational database"},
	"pg":          {"Amazon RDS", "Relational database"},
	"mysql2":      {"Amazon RDS", "Relational database"},
	"mysqlclient": {"Amazon RDS", "Relational database"},
	"sequelize":   {"Amazon RDS", "Relational database"},
	"sqlalchemy":  {"Amazon RDS", "Relational database"},
	"lib/pq":      {"Amazon RDS", "Relational database"},
	"redis":       {"Amazon ElastiCache", "In-memory cache"},
	"ioredis":     {"Amazon ElastiCache", "In-memory cache"},
}

// Detect performs the architectural interpretation of an already-analysed
// repository snapshot. It never invents services: each one carries the evidence
// that produced it. Files are read from snap.LocalPath when available.
func Detect(snap processing.Snapshot) Detection {
	acc := newAccumulator()
	acc.Languages = append(acc.Languages, snap.Analysis.Languages...)

	// Capability flags seeded from the coarse analysis already collected.
	acc.HasContainers = containsAny(snap.Analysis.Containers, "Docker", "Docker Compose")
	acc.HasKubernetes = containsAny(snap.Analysis.Containers, "Kubernetes")
	acc.HasCICD = len(snap.Analysis.CICD) > 0

	// Scan repository files for concrete AWS evidence (IaC, SDK clients, deps).
	scanFiles(snap.LocalPath, acc)

	// Corpus-level hints from README/docs (AI usage, API surface).
	corpus := strings.ToLower(readmeAndDocs(snap))
	acc.scanCorpus(corpus)

	return acc.finish()
}

// accumulator collects evidence during detection.
type accumulator struct {
	services map[string]*Service // keyed by service id
	Detection
}

func newAccumulator() *accumulator {
	return &accumulator{services: map[string]*Service{}}
}

// add records a service with evidence, keeping the strongest confidence seen and
// merging (de-duplicating) evidence lines.
func (a *accumulator) add(def svcDef, conf Confidence, evidence string) {
	id := serviceID(def.name)
	s, ok := a.services[id]
	if !ok {
		s = &Service{ID: id, Name: def.name, Logical: def.logical, Confidence: conf}
		a.services[id] = s
	}
	if conf.rank() > s.Confidence.rank() {
		s.Confidence = conf
	}
	if evidence != "" && !contains(s.Evidence, evidence) {
		s.Evidence = append(s.Evidence, evidence)
	}
	a.setFlags(def.name)
}

func (a *accumulator) setFlags(name string) {
	switch name {
	case "Amazon RDS", "Amazon DynamoDB":
		a.HasDatabase = true
	case "Amazon ElastiCache":
		a.HasCache = true
	case "Amazon SQS":
		a.HasQueue = true
	case "Amazon SNS", "Amazon EventBridge":
		a.HasEvents = true
	case "Amazon API Gateway":
		a.HasAPI = true
	case "Amazon ECS":
		a.HasContainers = true
	case "Amazon EKS":
		a.HasKubernetes = true
	}
}

func (a *accumulator) scanCorpus(corpus string) {
	if corpus == "" {
		return
	}
	for _, kw := range []string{"llm", "openai", "anthropic", "langchain", "ollama", "llama", "gpt", "embeddings", "rag", "transformers", "hugging face", "huggingface"} {
		if strings.Contains(corpus, kw) {
			a.HasAI = true
			break
		}
	}
	for _, kw := range []string{"rest api", "graphql", "endpoint", "/api/", "openapi", "swagger"} {
		if strings.Contains(corpus, kw) {
			a.HasAPI = true
			break
		}
	}
}

// finish resolves the compute substrate and deployment-context base services,
// then returns an id-sorted Detection.
func (a *accumulator) finish() Detection {
	d := a.Detection

	// Source is always GitHub — this is, by definition, a GitHub repository.
	d.Source = Service{ID: "github", Name: "GitHub", Logical: "Source repository",
		Confidence: High, Evidence: []string{"repository is hosted on GitHub"}}

	// Compute substrate: strongest container/serverless evidence wins, else the
	// deployment-context default of an EC2 (Spot) instance.
	switch {
	case a.services[serviceID("AWS Lambda")] != nil:
		d.Compute = *a.services[serviceID("AWS Lambda")]
	case a.services[serviceID("Amazon EKS")] != nil:
		d.Compute = *a.services[serviceID("Amazon EKS")]
	case a.services[serviceID("Amazon ECS")] != nil:
		d.Compute = *a.services[serviceID("Amazon ECS")]
	case a.services["ec2"] != nil:
		// A directly-detected EC2 instance (High) — keep its evidence.
		d.Compute = *a.services["ec2"]
	default:
		d.Compute = Service{ID: "ec2", Name: "Amazon EC2 (Spot)", Logical: "Application compute",
			Confidence: Medium, Evidence: []string{"deployment context: EC2 Spot instance"}}
		a.services[d.Compute.ID] = &d.Compute
	}

	// If the repository uses AI, the LLM maps to OpenClaw on EC2 (never Bedrock).
	if a.HasAI {
		llm := Service{ID: "openclaw", Name: "OpenClaw on Amazon EC2", Logical: "LLM inference",
			Confidence: Medium, Evidence: []string{"repository references LLM/AI usage; deployment context: OpenClaw on EC2 Spot"}}
		a.services[llm.ID] = &llm
	}

	for _, s := range a.services {
		d.Services = append(d.Services, *s)
	}
	// Include GitHub in the service list for reporting completeness.
	d.Services = append(d.Services, d.Source)
	sort.Slice(d.Services, func(i, j int) bool { return d.Services[i].ID < d.Services[j].ID })
	d.Languages = dedupe(d.Languages)
	return d
}

// scanFiles walks the working copy (bounded) and applies IaC / SDK / dependency
// evidence rules. A missing or empty LocalPath simply yields no file evidence.
func scanFiles(root string, acc *accumulator) {
	if root == "" {
		return
	}
	const maxFiles, maxBytes = 400, 512 * 1024
	seen := 0
	_ = filepath.WalkDir(root, func(path string, dEntry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if dEntry.IsDir() {
			if skipDir(dEntry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if seen >= maxFiles {
			return filepath.SkipDir
		}
		name := dEntry.Name()
		if !interestingFile(name) {
			return nil
		}
		info, err := dEntry.Info()
		if err != nil || info.Size() > maxBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		seen++
		rel, _ := filepath.Rel(root, path)
		applyRules(rel, name, string(data), acc)
		return nil
	})
}

func applyRules(rel, name, content string, acc *accumulator) {
	lower := strings.ToLower(name)

	// Terraform.
	if strings.HasSuffix(lower, ".tf") || strings.HasSuffix(lower, ".tf.json") {
		for token, def := range terraformServices {
			if strings.Contains(content, token) {
				acc.add(def, High, rel+": "+token)
			}
		}
	}
	// CloudFormation / CDK-synth / SAM templates (yaml/json containing AWS:: types).
	if strings.Contains(content, "AWS::") {
		for token, def := range cfnServices {
			if strings.Contains(content, token) {
				acc.add(def, High, rel+": "+token)
			}
		}
	}
	// Kubernetes manifests.
	if (strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")) &&
		strings.Contains(content, "apiVersion:") && strings.Contains(content, "kind:") {
		acc.HasKubernetes = true
		acc.HasContainers = true
	}
	// SDK client usage and dependency manifests.
	if isManifest(lower) || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".js") ||
		strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".go") {
		for token, def := range sdkServices {
			if strings.Contains(content, token) {
				acc.add(def, High, rel+": "+token)
			}
		}
		lc := strings.ToLower(content)
		for token, def := range dependencyHints {
			if strings.Contains(lc, token) {
				acc.add(def, Medium, rel+": depends on "+token)
			}
		}
	}
	// docker-compose backing services → Medium inference.
	if strings.Contains(lower, "docker-compose") {
		acc.HasContainers = true
		lc := strings.ToLower(content)
		if strings.Contains(lc, "postgres") || strings.Contains(lc, "mysql") || strings.Contains(lc, "mariadb") {
			acc.add(svcDef{"Amazon RDS", "Relational database"}, Medium, rel+": database service in compose")
		}
		if strings.Contains(lc, "redis") {
			acc.add(svcDef{"Amazon ElastiCache", "In-memory cache"}, Medium, rel+": redis service in compose")
		}
	}
	if lower == "dockerfile" {
		acc.HasContainers = true
	}
}

// --- small helpers -------------------------------------------------------

func serviceID(name string) string {
	s := strings.ToLower(name)
	s = strings.NewReplacer("amazon ", "", "aws ", "", " (spot)", "", " on amazon ec2", "").Replace(s)
	s = idSanitizer.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func interestingFile(name string) bool {
	lower := strings.ToLower(name)
	if isManifest(lower) || lower == "dockerfile" || strings.Contains(lower, "docker-compose") {
		return true
	}
	for _, ext := range []string{".tf", ".tf.json", ".yaml", ".yml", ".json", ".ts", ".js", ".py", ".go"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isManifest(lower string) bool {
	switch lower {
	case "package.json", "requirements.txt", "pyproject.toml", "pom.xml", "build.gradle", "go.mod":
		return true
	}
	return false
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", "target", "__pycache__", ".venv", "venv":
		return true
	}
	return false
}

func readmeAndDocs(snap processing.Snapshot) string {
	var b strings.Builder
	b.WriteString(snap.Readme)
	for _, d := range snap.Docs {
		b.WriteByte('\n')
		b.WriteString(d.Content)
	}
	return b.String()
}

func containsAny(list []string, targets ...string) bool {
	for _, v := range list {
		for _, t := range targets {
			if strings.EqualFold(v, t) {
				return true
			}
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
