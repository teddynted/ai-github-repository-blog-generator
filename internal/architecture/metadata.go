package architecture

import "strings"

// planDiagramMeta builds per-diagram metadata from the grounded graph.
func planDiagramMeta(pkg ReleasePackage, spec diagramSpec, generatedAt string) DiagramMeta {
	g := spec.Graph
	return DiagramMeta{
		Complexity:          complexityFor(len(g.Nodes), len(g.Edges)),
		Audience:            audience(pkg),
		ReleaseVersion:      tag(pkg),
		GeneratedAt:         generatedAt,
		ReferencedFiles:     referencedFiles(pkg, spec),
		ReferencedResources: g.awsServiceLabels(),
		NodeCount:           len(g.Nodes),
		EdgeCount:           len(g.Edges),
		Confidence:          diagramConfidence(g),
	}
}

func complexityFor(nodes, edges int) string {
	score := nodes + edges
	switch {
	case score >= 18:
		return "high"
	case score >= 8:
		return "medium"
	default:
		return "low"
	}
}

// diagramConfidence rewards diagrams whose nodes are catalogued AWS services and
// whose edges are grounded.
func diagramConfidence(g graph) int {
	if len(g.Nodes) == 0 {
		return 0
	}
	known := 0
	for _, n := range g.Nodes {
		if n.Icon != "" {
			known++
		}
	}
	score := 60
	score += int(40 * float64(known) / float64(len(g.Nodes)))
	if len(g.Edges) > 0 {
		score += 0 // edges already required to be grounded
	}
	if score > 100 {
		score = 100
	}
	return score
}

func referencedFiles(pkg ReleasePackage, spec diagramSpec) []string {
	var files []string
	for _, r := range spec.References {
		if isFilePath(r) {
			files = append(files, r)
		}
	}
	if pkg.Context != nil {
		for _, t := range pkg.Context.CloudFormation.Templates {
			files = append(files, t)
		}
	}
	return dedupe(files)
}

func isFilePath(s string) bool {
	return len(s) > 0 && (containsAny(s, "/", ".md", ".yml", ".yaml", ".go", ".json"))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// planIntelligence aggregates collection-level architecture metadata from the
// analysis. Every list is grounded.
func planIntelligence(pkg ReleasePackage, a analysis, diagrams []Diagram) Intelligence {
	confSum := 0
	for _, d := range diagrams {
		confSum += d.Metadata.Confidence
	}
	conf := 0
	if len(diagrams) > 0 {
		conf = confSum / len(diagrams)
	}

	return Intelligence{
		ArchitectureStyle:        architectureStyle(a),
		DeploymentPattern:        deploymentPattern(pkg, a),
		InfrastructureComplexity: infraComplexity(a),
		LocalInference:           a.Inference.Local,
		CloudInference:           a.Inference.Cloud,
		PrimaryWorkflow:          primaryWorkflow(a),
		OperationalModel:         operationalModel(a),
		CloudServices:            a.serviceLabels(),
		ComputeComponents:        a.categoryServices("Compute"),
		ServerlessComponents:     a.categoryServices("Serverless"),
		StorageComponents:        a.categoryServices("Storage"),
		DatabaseComponents:       a.categoryServices("Database"),
		MessagingComponents:      a.categoryServices("Messaging"),
		NetworkingComponents:     a.categoryServices("Networking"),
		SecurityComponents:       a.categoryServices("Security"),
		IntegrationServices:      a.categoryServices("Integration"),
		ObservabilityComponents:  a.categoryServices("Observability"),
		EstimatedReadingTime:     readingTime(len(diagrams)),
		DiagramConfidence:        conf,
	}
}

// architectureStyle derives the platform classification from repository EVIDENCE,
// never a fixed template. Each qualifier is only used when it is grounded:
// "Event-driven" only with event/messaging evidence (a.EventDriven), "hybrid AI"
// only when both local and cloud inference are detected, and the AI/tooling/
// automation shape from the services and structure actually present.
func architectureStyle(a analysis) string {
	hybrid := a.Inference.hybrid()
	hasAI := hybrid || len(a.Inference.Cloud) > 0 || len(a.Inference.Local) > 0 || len(a.categoryServices("AI/ML")) > 0

	if hasAI {
		suffix := "AI platform"
		if hybrid {
			suffix = "hybrid AI platform"
		}
		switch {
		case a.EventDriven:
			return "Event-driven " + suffix
		case a.hasService("Amazon API Gateway"):
			return "Service-oriented " + suffix
		case isDeveloperTooling(a):
			return "AI-enabled developer tooling platform"
		default:
			return upperFirst(suffix) // "Hybrid AI platform" / "AI platform"
		}
	}
	// No AI evidence — classify by infrastructure shape.
	switch {
	case a.EventDriven:
		return "Event-driven platform"
	case len(a.Templates) > 0 && len(a.Services) > 0:
		return "Cloud automation platform"
	case len(a.categoryServices("Serverless")) > 0:
		return "Serverless platform"
	case len(a.categoryServices("Compute")) > 0:
		return "Compute-based platform"
	default:
		return "Application"
	}
}

// isDeveloperTooling reports a CLI/tooling-shaped repo: command entry points with
// little or no cloud infrastructure of its own.
func isDeveloperTooling(a analysis) bool {
	hasCmd := false
	for _, d := range a.Dirs {
		if p := strings.ToLower(d.Path); p == "cmd" || strings.HasPrefix(p, "cmd/") {
			hasCmd = true
			break
		}
	}
	return hasCmd && len(a.Services) <= 1 && len(a.Templates) == 0
}

// upperFirst capitalises the first rune of s.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// primaryWorkflow renders the release's main event-driven flow as a concise
// arrow chain (e.g. "GitHub Release → SQS → n8n → AI Router"), grounded in the
// context's flow descriptions. Empty when no flow is described.
func primaryWorkflow(a analysis) string {
	for _, flow := range a.Flows {
		if steps := flowSteps(flow); len(steps) >= 2 {
			return strings.Join(steps, " → ")
		}
	}
	return ""
}

// operationalModel summarizes how the platform runs, grounded in what the context
// shows: hybrid self-hosted + cloud AI, IaC-provisioned AWS, or unspecified.
func operationalModel(a analysis) string {
	switch {
	case a.Inference.hybrid():
		return "Self-hosted automation with cloud AI augmentation"
	case len(a.Templates) > 0:
		return "Infrastructure as Code on AWS"
	case len(a.Services) > 0:
		return "AWS-hosted"
	default:
		return ""
	}
}

func deploymentPattern(pkg ReleasePackage, a analysis) string {
	if pkg.Context != nil && pkg.Context.Architecture.DeploymentTopology != "" {
		return firstSentences(pkg.Context.Architecture.DeploymentTopology, 1)
	}
	if len(a.Templates) > 0 {
		return "Infrastructure as Code (CloudFormation)"
	}
	return "Not specified in the release context"
}

func infraComplexity(a analysis) string {
	n := len(a.Services) + len(a.Resources)
	switch {
	case n >= 12:
		return "high"
	case n >= 5:
		return "medium"
	default:
		return "low"
	}
}

func readingTime(diagrams int) string {
	m := diagrams // ~1 min per diagram to absorb
	if m < 1 {
		m = 1
	}
	return itoa(m) + " min"
}

func audience(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.ContentIntelligence.TargetAudience != "" {
		return pkg.Context.ContentIntelligence.TargetAudience
	}
	return "Software engineers and cloud architects"
}
