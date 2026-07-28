// Package contentsuite is the single orchestrator that turns one Release Context
// into the complete set of release artifacts (Milestones 3–13) in dependency
// order, wiring the dedicated generator packages in-process. It is the "fan-out"
// that the QA quality gate (H1) found missing: before it, an operator had to run
// ~10 CLIs by hand, threading each stage's output file into the next.
//
// Design:
//   - It composes the EXISTING, tested generator packages (storyboard, voiceover,
//     youtube, shorts, tiktok, visualassets, seo, architecture, linkedin,
//     xthread) — no generation logic is duplicated here.
//   - Every stage is fault-tolerant: a stage that fails (e.g. a storyboard with no
//     sections, or a release with no groundable infrastructure) is recorded in the
//     manifest and the run CONTINUES; one weak input never aborts the whole
//     pipeline. This also resolves the QA edge cases M1/M2 at the orchestration
//     level without weakening any generator's grounding.
//   - It is pure (no file I/O): Run returns a Suite; the caller writes artifacts.
package contentsuite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/teddynted/ai-github-repository-blog-generator/internal/architecture"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/archspec"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/linkedin"
	rc "github.com/teddynted/ai-github-repository-blog-generator/internal/releasecontext"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/releasegen"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/seo"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/shorts"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/storyboard"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/svgdiagram"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/tiktok"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/visualassets"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/voiceover"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/xthread"
	"github.com/teddynted/ai-github-repository-blog-generator/internal/youtube"
)

// SchemaVersion is the suite manifest version (SemVer, additive-only).
const SchemaVersion = "1.0.0"

// StageStatus is one stage's outcome.
type StageStatus string

const (
	StageOK      StageStatus = "ok"
	StageFailed  StageStatus = "failed"
	StageSkipped StageStatus = "skipped"
)

// StageResult records how one artifact stage went.
type StageResult struct {
	Name      string      `json:"name"`
	Milestone int         `json:"milestone"`
	Status    StageStatus `json:"status"`
	Artifact  string      `json:"artifact,omitempty"` // output filename when ok
	Error     string      `json:"error,omitempty"`
}

// Manifest correlates a release with every artifact produced for it.
type Manifest struct {
	SchemaVersion string        `json:"schemaVersion"`
	Repository    string        `json:"repository"`
	Release       string        `json:"release"`
	ContextID     string        `json:"contextId"`
	GeneratedAt   time.Time     `json:"generatedAt"`
	Offline       bool          `json:"offline"`
	Stages        []StageResult `json:"stages"`
	Produced      int           `json:"produced"`
	Skipped       int           `json:"skipped"`
	Failed        int           `json:"failed"`
}

// Artifact is one rendered output ready to write.
type Artifact struct {
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
	Markdown string `json:"-"`
	// Ext is the file extension without the dot ("md", "svg"). Empty means "md".
	Ext string `json:"ext,omitempty"`
	// Provenance — the provider/model/prompt that produced this artifact.
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"promptVersion,omitempty"`
}

// Suite is the complete set of artifacts from one release.
type Suite struct {
	Manifest     Manifest
	Blog         releasegen.BlogPost
	Storyboard   storyboard.Storyboard
	VoiceOver    voiceover.VoiceOverScript
	YouTube      youtube.YouTubeScript
	Shorts       shorts.ShortsCollection
	TikTok       tiktok.TikTokCollection
	VisualAssets visualassets.VisualAssetCollection
	SEO          seo.SEOMetadata
	Architecture architecture.ArchitectureCollection
	LinkedIn     linkedin.LinkedInCollection
	XThread      xthread.XThreadCollection
	DiagramSpec  archspec.Spec
	artifacts    []Artifact
}

// Artifacts returns the rendered outputs for every successful stage, in order.
func (s *Suite) Artifacts() []Artifact { return s.artifacts }

// Orchestrator wires the model + per-generator options. A nil Model runs every
// generator in its deterministic, offline mode (no network) — except the blog,
// which requires a model; supply a pre-generated blog to Run when offline.
type Orchestrator struct {
	Model releasegen.Model
	// ModelFor, when set, selects the model per content kind (Hybrid AI Routing):
	// premium model for high-value artifacts, local model for commodity ones. The
	// argument is the stage name ("blog", "architecture", "seo-metadata", …). When
	// nil, Model is used for every stage — fully backward compatible.
	ModelFor func(kind string) releasegen.Model
	// Provenance, when set, reports how a stage's artifact was produced
	// (provider, model, prompt version) for the release metadata. Optional; when
	// nil, artifacts carry no provenance.
	Provenance     func(kind string) (provider, model, promptVersion string)
	Logger         *slog.Logger
	MaxShorts      int
	MaxVideos      int
	MaxAssets      int
	MaxPosts       int
	MaxThreads     int
	PostsPerThread int
	// Only, when non-empty, restricts the run to the named stages plus their
	// transitive dependencies; every other stage is skipped (not generated, not
	// recorded). When nil/empty, all stages run — fully backward compatible, so
	// the cloud pipeline (which never sets it) is unchanged. Used by the local
	// --from-blog per-artifact workflow so requesting one artifact no longer runs
	// the entire suite. Unknown stage names contribute only themselves.
	Only map[string]bool
}

// stageDeps maps each stage to the stages whose output it consumes, mirroring the
// data flow in Run (the s.* fields each generator reads). It drives Orchestrator.Only:
// the minimal set to execute is the requested stages plus this graph's transitive
// closure. Keep it in sync with Run.
var stageDeps = map[string][]string{
	"blog":           nil,
	"storyboard":     {"blog"},
	"voiceover":      {"storyboard"},
	"youtube":        {"voiceover"},
	"youtube-shorts": {"youtube"},
	"tiktok":         {"youtube-shorts"},
	"visual-assets":  {"tiktok"},
	"seo-metadata":   {"visual-assets"},
	// architecture reads only the storyboard's repo/release labels (seeded from the
	// release context in Run), not its generated scenes — so it depends on the blog
	// alone and never triggers the Ollama-bound storyboard stage. linkedin/x-thread,
	// by contrast, genuinely consume SEO + visual-assets + architecture *content*,
	// so they keep their full chains (local output stays identical to the cloud).
	"architecture":              {"blog"},
	"linkedin":                  {"architecture", "seo-metadata"},
	"x-thread":                  {"architecture", "seo-metadata"},
	"architecture-diagram-spec": {"blog"},
	"architecture-diagram":      {"blog"},
}

// activeStages returns the set of stages to execute. A nil result means "run
// everything" (Only unset); otherwise it is each requested stage expanded over
// stageDeps to include its transitive dependencies.
func (o *Orchestrator) activeStages() map[string]bool {
	if len(o.Only) == 0 {
		return nil
	}
	active := map[string]bool{}
	var add func(string)
	add = func(stage string) {
		if active[stage] {
			return
		}
		active[stage] = true
		for _, dep := range stageDeps[stage] {
			add(dep)
		}
	}
	for stage := range o.Only {
		add(stage)
	}
	return active
}

// model returns the model for a stage: the router's per-kind choice when
// ModelFor is set, otherwise the single Model.
func (o *Orchestrator) model(kind string) releasegen.Model {
	if o.ModelFor != nil {
		return o.ModelFor(kind)
	}
	return o.Model
}

// Run executes every stage for one release. If blog is nil it is generated first
// (requires a Model); otherwise the supplied blog is used verbatim (the offline
// path). It never returns a fatal error for a single failed stage — inspect the
// manifest for per-stage status.
func (o *Orchestrator) Run(ctx context.Context, rctx *rc.ReleaseContext, blog *releasegen.BlogPost) *Suite {
	s := &Suite{Manifest: Manifest{
		SchemaVersion: SchemaVersion,
		Repository:    rctx.Repository.FullName,
		Release:       rctx.Release.Tag,
		ContextID:     rctx.ContextID,
		GeneratedAt:   time.Now(),
		Offline:       o.Model == nil,
	}}

	// active reports whether a stage should run: nil ⇒ every stage (default);
	// otherwise the requested stages plus their transitive dependencies (Only).
	active := o.activeStages()
	run := func(stage string) bool { return active == nil || active[stage] }
	if active != nil && o.Logger != nil {
		o.Logger.Info("stage selection", "requested", sortedKeys(o.Only), "running", sortedKeys(active))
	}

	// --- M3 Blog (foundational) ---
	if !run("blog") {
		// Nothing selected depends on the blog (only possible for unknown stage
		// names); there is nothing to derive, so return the empty suite.
		return s
	}
	if blog != nil {
		s.Blog = *blog
		blogOut := stageOutcome{name: "blog", milestone: 3, filename: "01-blog.md", status: StageOK, md: s.Blog.Markdown}
		if o.Provenance != nil {
			blogOut.provider, blogOut.model, blogOut.promptVersion = o.Provenance("blog")
		}
		s.record(blogOut)
	} else {
		s.record(o.run("blog", 3, "01-blog.md", func() (string, error) {
			b, err := (&releasegen.Generator{Model: o.model("blog")}).Blog(ctx, rctx)
			s.Blog = b
			return b.Markdown, err
		}))
	}

	// Seed the storyboard's repo/release labels from the release context. The
	// architecture stage reads only these labels (not the storyboard's generated
	// scenes), so this lets architecture run without the Ollama-bound storyboard
	// stage. When the storyboard stage runs, it overwrites this with the full
	// result — architecture reads the same labels either way, so its output is
	// identical whether or not the storyboard stage ran.
	s.Storyboard = storyboard.Storyboard{
		SchemaVersion: storyboard.SchemaVersion,
		Metadata: storyboard.Metadata{
			Repository:      rctx.Repository.FullName,
			Release:         rctx.Release.Tag,
			SourceBlogTitle: s.Blog.Title,
		},
	}

	// --- M4 Storyboard (blog + context) ---
	if run("storyboard") {
		s.record(o.run("storyboard", 4, "02-storyboard.md", func() (string, error) {
			sb, err := (&storyboard.Generator{Model: o.model("storyboard")}).Storyboard(ctx, s.Blog, rctx)
			s.Storyboard = sb
			return sb.Markdown(), err
		}))
	}

	// --- M5 Voice-over (storyboard) ---
	if run("voiceover") {
		s.record(o.run("voiceover", 5, "03-voiceover.md", func() (string, error) {
			vo, err := (&voiceover.Generator{Model: o.model("voiceover")}).VoiceOver(ctx, s.Storyboard)
			s.VoiceOver = vo
			return vo.Markdown(), err
		}))
	}

	// --- M11 Architecture (context+blog+storyboard) runs CONCURRENTLY with the
	// M6→M10 chain: it depends only on the blog + storyboard (both ready), not on
	// the YouTube→SEO chain. Its result is recorded in canonical position (after
	// SEO) so the manifest/artifact order stays deterministic regardless of timing.
	var wg sync.WaitGroup
	var archOut stageOutcome
	archActive := run("architecture")
	if archActive {
		wg.Add(1)
		go func() {
			defer wg.Done()
			archOut = o.run("architecture", 11, "09-architecture.md", func() (string, error) {
				col, err := (&architecture.Generator{Model: o.model("architecture")}).Architecture(ctx, architecture.ReleasePackage{
					Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard,
				})
				s.Architecture = col
				// Emit the version-independent, repository-level architecture document
				// (no release tag, curated structure) so the same stable doc is produced
				// locally and in the cloud.
				return col.RepoLevelMarkdown(), err
			})
		}()
	}

	// --- M6 YouTube script (context+blog+storyboard+voiceover) ---
	if run("youtube") {
		s.record(o.run("youtube", 6, "04-youtube-script.md", func() (string, error) {
			sc, err := (&youtube.Generator{Model: o.model("youtube")}).YouTube(ctx, youtube.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver,
			})
			s.YouTube = sc
			return sc.Markdown(), err
		}))
	}

	// --- M7 YouTube Shorts (+youtube) ---
	if run("youtube-shorts") {
		s.record(o.run("youtube-shorts", 7, "05-youtube-shorts.md", func() (string, error) {
			col, err := (&shorts.Generator{Model: o.model("youtube-shorts"), MaxShorts: o.MaxShorts}).YouTubeShorts(ctx, shorts.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
			})
			s.Shorts = col
			return col.Markdown(), err
		}))
	}

	// --- M8 TikTok (+shorts) ---
	if run("tiktok") {
		s.record(o.run("tiktok", 8, "06-tiktok.md", func() (string, error) {
			col, err := (&tiktok.Generator{Model: o.model("tiktok"), MaxVideos: o.MaxVideos}).TikTok(ctx, tiktok.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts,
			})
			s.TikTok = col
			return col.Markdown(), err
		}))
	}

	// --- M9 Visual Assets (+tiktok) ---
	if run("visual-assets") {
		s.record(o.run("visual-assets", 9, "07-visual-assets.md", func() (string, error) {
			col, err := (&visualassets.Generator{Model: o.model("visual-assets"), MaxAssets: o.MaxAssets}).VisualAssets(ctx, visualassets.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok,
			})
			s.VisualAssets = col
			return col.Markdown(), err
		}))
	}

	// --- M10 SEO metadata (+visual assets) ---
	if run("seo-metadata") {
		s.record(o.run("seo-metadata", 10, "08-seo-metadata.md", func() (string, error) {
			m, err := (&seo.Generator{Model: o.model("seo-metadata")}).SEO(ctx, seo.ReleasePackage{
				Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube, Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets,
			})
			s.SEO = m
			return m.Markdown(), err
		}))
	}

	// Join the architecture branch and record it at its canonical position (11).
	wg.Wait()
	if archActive {
		s.record(archOut)
	}

	// --- M12 LinkedIn and M13 X thread both depend on everything above but NOT on
	// each other, so they run concurrently; results are recorded in canonical order.
	var liOut, xtOut stageOutcome
	liActive, xtActive := run("linkedin"), run("x-thread")
	if liActive {
		wg.Add(1)
		go func() {
			defer wg.Done()
			liOut = o.run("linkedin", 12, "10-linkedin.md", func() (string, error) {
				col, err := (&linkedin.Generator{Model: o.model("linkedin"), MaxPosts: o.MaxPosts}).LinkedIn(ctx, linkedin.ReleasePackage{
					Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
					Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
				})
				s.LinkedIn = col
				return col.Markdown(), err
			})
		}()
	}
	if xtActive {
		wg.Add(1)
		go func() {
			defer wg.Done()
			xtOut = o.run("x-thread", 13, "11-x-thread.md", func() (string, error) {
				col, err := (&xthread.Generator{Model: o.model("x-thread"), MaxThreads: o.MaxThreads, PostsPerThread: o.PostsPerThread}).XThread(ctx, xthread.ReleasePackage{
					Context: rctx, Blog: s.Blog, Storyboard: s.Storyboard, VoiceOver: s.VoiceOver, YouTube: s.YouTube,
					Shorts: s.Shorts, TikTok: s.TikTok, VisualAssets: s.VisualAssets, SEO: s.SEO, Architecture: s.Architecture,
				})
				s.XThread = col
				return col.Markdown(), err
			})
		}()
	}
	wg.Wait()
	if liActive {
		s.record(liOut)
	}
	if xtActive {
		s.record(xtOut)
	}

	// --- M14 AWS Architecture Diagram Specification (context + blog topic) ---
	// A separate, renderer-facing artifact (NOT appended to the blog): a
	// repository-grounded spec a downstream pipeline turns into an AWS diagram.
	// Skipped gracefully when the repository has no groundable AWS evidence.
	if run("architecture-diagram-spec") {
		s.record(o.run("architecture-diagram-spec", 14, "12-architecture-diagram-spec.md", func() (string, error) {
			spec, err := (&archspec.Generator{Model: o.model("architecture-diagram-spec"), Logger: o.Logger}).Spec(ctx, archspec.ReleasePackage{
				Context: rctx, Blog: s.Blog,
			})
			s.DiagramSpec = spec
			return spec.Markdown(), err
		}))
	}

	// --- M15 Architecture diagram (SVG) ---
	// A deterministic, self-contained SVG rendering of the repository's
	// architecture diagrams (the Mermaid flows the blog embeds, or a grounded
	// CFN/service graph). Skipped when the release has no diagrammable evidence.
	if run("architecture-diagram") {
		s.record(o.runExt("architecture-diagram", 15, "13-architecture-diagram.svg", "svg", func() (string, error) {
			graphs := diagramGraphs(rctx)
			if len(graphs) == 0 {
				return "", errNoDiagram
			}
			return svgdiagram.RenderDocument(diagramTitle(rctx, s.Blog), graphs), nil
		}))
	}

	return s
}

// sortedKeys returns the map's keys in deterministic order, for stable logging.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// errNoDiagram signals the release has nothing diagrammable, so the SVG stage
// skips gracefully rather than emitting an empty image.
var errNoDiagram = errors.New("contentsuite: no diagrammable architecture evidence")

// maxSVGDiagrams caps how many diagram panels the SVG embeds (mirrors the blog's
// two-diagram ceiling).
const maxSVGDiagrams = 2

// diagramGraphs derives the diagram panels from repository evidence, most
// specific first: the repository's Mermaid diagrams (real nodes + edges), else a
// grounded graph of CloudFormation resources, else the detected AWS services.
// It never fabricates connections.
func diagramGraphs(rctx *rc.ReleaseContext) []svgdiagram.Graph {
	if len(rctx.Mermaid) > 0 {
		var graphs []svgdiagram.Graph
		for i, d := range rctx.Mermaid {
			if i >= maxSVGDiagrams {
				break
			}
			g := svgdiagram.Graph{Title: firstNonEmptyStr(d.Summary, fmt.Sprintf("Diagram %d", i+1))}
			for _, n := range d.Nodes {
				g.Nodes = append(g.Nodes, svgdiagram.Node{ID: n, Label: n})
			}
			for _, e := range d.Edges {
				g.Edges = append(g.Edges, svgdiagram.Edge{From: e.From, To: e.To, Label: e.Label})
			}
			graphs = append(graphs, g)
		}
		return graphs
	}
	if len(rctx.CloudFormation.Resources) > 0 {
		g := svgdiagram.Graph{Title: "AWS Architecture"}
		for _, r := range rctx.CloudFormation.Resources {
			g.Nodes = append(g.Nodes, svgdiagram.Node{ID: r.LogicalID, Label: firstNonEmptyStr(r.Service, r.LogicalID), Group: r.Category})
		}
		return []svgdiagram.Graph{g}
	}
	if len(rctx.Architecture.AWSServices) > 0 {
		g := svgdiagram.Graph{Title: "AWS Services"}
		for _, s := range rctx.Architecture.AWSServices {
			g.Nodes = append(g.Nodes, svgdiagram.Node{ID: s, Label: s})
		}
		return []svgdiagram.Graph{g}
	}
	return nil
}

// diagramTitle is the SVG document title, anchored to the article topic.
func diagramTitle(rctx *rc.ReleaseContext, blog releasegen.BlogPost) string {
	name := firstNonEmptyStr(blog.Title, rctx.Repository.Name, rctx.Repository.FullName)
	return name + " — Architecture"
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// stageOutcome is the computed result of one stage, before it is recorded into
// the manifest. Separating computation from recording lets independent stages run
// concurrently while the manifest and artifact list stay in canonical order.
type stageOutcome struct {
	name          string
	milestone     int
	filename      string
	ext           string
	provider      string
	model         string
	promptVersion string
	status        StageStatus
	err           error
	md            string
}

// run executes one generator and returns its outcome WITHOUT touching shared
// manifest state, so it is safe to call from concurrent goroutines. A deliberate
// "nothing to do" refusal (no infrastructure to diagram, no Short-worthy moments,
// no TikTok-worthy topics) becomes a graceful skip, not a failure. The caller
// records the outcome (Suite.record) in canonical order.
func (o *Orchestrator) run(name string, milestone int, filename string, fn func() (string, error)) stageOutcome {
	md, err := fn()
	if err != nil {
		status := StageFailed
		if isSkip(err) {
			status = StageSkipped
		}
		if o.Logger != nil {
			o.Logger.Warn("content stage "+string(status), slog.String("stage", name), slog.Int("milestone", milestone), slog.String("error", err.Error()))
		}
		return stageOutcome{name: name, milestone: milestone, status: status, err: err}
	}
	out := stageOutcome{name: name, milestone: milestone, filename: filename, status: StageOK, md: md}
	if o.Provenance != nil {
		out.provider, out.model, out.promptVersion = o.Provenance(name)
	}
	return out
}

// runExt is run() for a stage whose artifact is not Markdown (e.g. an SVG); it
// stamps the file extension onto the outcome.
func (o *Orchestrator) runExt(name string, milestone int, filename, ext string, fn func() (string, error)) stageOutcome {
	out := o.run(name, milestone, filename, fn)
	out.ext = ext
	return out
}

// isSkip reports whether an error is a grounded "nothing worthy to generate"
// refusal — a graceful skip rather than a failure. New generators that add such a
// sentinel are registered here.
func isSkip(err error) bool {
	return errors.Is(err, architecture.ErrNoInfrastructure) ||
		errors.Is(err, archspec.ErrNoEvidence) ||
		errors.Is(err, errNoDiagram) ||
		errors.Is(err, shorts.ErrNoMoments) ||
		errors.Is(err, tiktok.ErrNoTopics)
}

// record appends a stage outcome to the manifest (and the artifact list on
// success). It is called only from the orchestrating goroutine, in canonical
// milestone order, so the manifest is deterministic even when stages run
// concurrently.
func (s *Suite) record(o stageOutcome) {
	r := StageResult{Name: o.name, Milestone: o.milestone, Status: o.status, Artifact: o.filename}
	if o.err != nil {
		r.Error = o.err.Error()
	}
	s.Manifest.Stages = append(s.Manifest.Stages, r)
	switch o.status {
	case StageOK:
		s.Manifest.Produced++
		if o.filename != "" {
			s.artifacts = append(s.artifacts, Artifact{
				Filename: o.filename, Kind: o.name, Markdown: o.md, Ext: o.ext,
				Provider: o.provider, Model: o.model, PromptVersion: o.promptVersion,
			})
		}
	case StageSkipped:
		s.Manifest.Skipped++
	case StageFailed:
		s.Manifest.Failed++
	}
}
