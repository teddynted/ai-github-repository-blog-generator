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

func architectureStyle(a analysis) string {
	base := baseArchitectureStyle(a)
	// Only label it a hybrid AI platform when both local and cloud inference are
	// grounded in the context — never for a plain infrastructure repo.
	if a.Inference.hybrid() {
		return base + " hybrid AI platform"
	}
	return base
}

func baseArchitectureStyle(a analysis) string {
	hasServerless := len(a.categoryServices("Serverless")) > 0
	hasMessaging := len(a.categoryServices("Messaging")) > 0 || len(a.categoryServices("Integration")) > 0
	// Event-driven when the release describes event flows or uses messaging/event
	// services — even if no serverless compute is detected.
	hasEventDriven := hasMessaging || len(a.Flows) > 0
	switch {
	case hasEventDriven && hasServerless:
		return "Event-driven serverless"
	case hasEventDriven:
		return "Event-driven"
	case hasServerless:
		return "Serverless"
	case len(a.categoryServices("Compute")) > 0:
		return "Container / compute-based"
	default:
		return "Application"
	}
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
