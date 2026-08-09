// Package scenediagram composes a deterministic, on-brand architecture diagram
// for a video scene: it detects the AWS services / components named in the
// scene and lays out clean, text-free service-icon tiles connected by flow
// arrows on the dark-navy brand background, as an SVG. A rasterizer (rsvg) turns
// it into the scene's background PNG.
//
// Unlike a text-to-image model, this is exact and repeatable — the tiles are the
// real services under discussion, never a hallucinated cityscape, and there is
// no garbled in-image text. The stylized glyphs here are brand-consistent
// placeholders; official AWS Architecture Icons can be swapped into iconFor
// without changing the layout.
package scenediagram

import (
	"embed"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// AnimationFrames is the number of frames in one seamless flow-animation loop;
// the renderer rasterizes SVG(...,phase) for phase = i/AnimationFrames and loops
// the result under the caption for the scene's duration.
const AnimationFrames = 20

// icons holds the official AWS Architecture Icons (64px SVGs) for the services we
// diagram. They are embedded into the binary so the renderer needs no runtime
// asset fetch. Non-AWS nodes (OpenClaw, GitHub, Ollama, n8n, Claude, webhook) keep the
// stylized glyphs in catalog.
//
//go:embed icons/*.svg
var icons embed.FS

// iconFile maps a detected component key to its embedded official-icon filename.
var iconFile = map[string]string{
	"eventbridge":    "eventbridge",
	"step functions": "stepfunctions",
	"sqs":            "sqs",
	"api gateway":    "apigateway",
	"lambda":         "lambda",
	"ec2":            "ec2",
	"bedrock":        "bedrock",
	"s3":             "s3",
	"efs":            "efs",
	"dynamodb":       "dynamodb",
	"cloudwatch":     "cloudwatch",
	"iam":            "iam",
}

var idAttr = regexp.MustCompile(`id="([^"]+)"`)
var idRef = regexp.MustCompile(`url\(#([^)]+)\)`)
var hrefRef = regexp.MustCompile(`(xlink:href|href)="#([^"]+)"`)

// officialIcon returns the inner SVG of a service's official AWS icon, with all
// internal ids namespaced by prefix so composing several icons (or the same icon
// twice) into one document never collides. Returns false for non-AWS components.
func officialIcon(key, prefix string) (string, bool) {
	f, ok := iconFile[key]
	if !ok {
		return "", false
	}
	raw, err := icons.ReadFile("icons/" + f + ".svg")
	if err != nil {
		return "", false
	}
	s := innerSVG(string(raw))
	s = idAttr.ReplaceAllString(s, `id="`+prefix+`$1"`)
	s = idRef.ReplaceAllString(s, `url(#`+prefix+`$1)`)
	s = hrefRef.ReplaceAllString(s, `$1="#`+prefix+`$2"`)
	return s, true
}

// innerSVG returns the content between the outer <svg …> and </svg>.
func innerSVG(s string) string {
	i := strings.Index(s, "<svg")
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], ">")
	if j < 0 {
		return ""
	}
	start := i + j + 1
	end := strings.LastIndex(s, "</svg>")
	if end < start {
		return ""
	}
	return s[start:end]
}

// Brand palette (matches the renderer's deep-slate title card and caption band).
const (
	bgTop    = "#0F1B2E"
	bgBottom = "#0A1220"
	grid     = "#1B2A44"
	stroke   = "#2C3E5E"
	flow     = "#FF9900" // AWS orange event/flow lines
	glyphOn  = "#EAF2FF"
)

// tile is one service node: a rounded square with a category colour and a simple
// white glyph, no text.
type tile struct {
	color string // category colour
	glyph string // inner SVG, drawn in a 0..100 local box, stroke/fill glyphOn
}

// catalog maps a detected component key to its tile. Keys are matched as
// substrings of the scene text (longest/most-specific first in detect()).
var catalog = map[string]tile{
	"eventbridge":    {"#C925D3", glyphEventBus}, // app integration (purple/magenta)
	"step functions": {"#C925D3", glyphBranch},   //
	"step function":  {"#C925D3", glyphBranch},   //
	"sqs":            {"#C925D3", glyphQueue},    //
	"api gateway":    {"#C925D3", glyphGateway},  //
	"lambda":         {"#ED7100", glyphBolt},     // compute (orange)
	"ec2":            {"#ED7100", glyphChip},     //
	"bedrock":        {"#01A88D", glyphAI},       // ML (teal/green)
	"claude":         {"#01A88D", glyphAI},       //
	"ollama":         {"#7AA116", glyphNode},     // local/other (green)
	"n8n":            {"#7AA116", glyphChain},    //
	"s3":             {"#7AA116", glyphBucket},   // storage (green)
	"efs":            {"#7AA116", glyphDisc},     //
	"dynamodb":       {"#4D72F3", glyphDb},       // database (blue)
	"cloudwatch":     {"#E7157B", glyphGauge},    // management (pink)
	"iam":            {"#DD344C", glyphShield},   // security (red)
	"github":         {"#5A6B86", glyphRepo},     // source (slate)
	"webhook":        {"#5A6B86", glyphPulse},    //
	"openclaw":       {"#8C4FFF", glyphAgent},    // orchestrator hub (agent violet)
	// Broader platform stack (stylized glyphs; official AWS icons can be dropped
	// into iconFor later without changing the layout).
	"fargate":         {"#ED7100", glyphContainer}, // compute (orange)
	"ecs":             {"#ED7100", glyphContainer}, //
	"cloudformation":  {"#E7157B", glyphStack},     // management / IaC (pink)
	"ecr":             {"#7AA116", glyphContainer}, // container registry (green)
	"secrets manager": {"#DD344C", glyphKey},       // security (red)
	"kms":             {"#DD344C", glyphKey},       //
	"sns":             {"#C925D3", glyphCast},      // app integration (magenta)
	"polly":           {"#01A88D", glyphWave},      // ML / media (teal)
	"replicate":       {"#01A88D", glyphAI},        //
	"anthropic":       {"#01A88D", glyphAI},        //
	"nova":            {"#01A88D", glyphAI},        //
	"docker":          {"#2496ED", glyphContainer}, // containers (docker blue)
	"ffmpeg":          {"#5A6B86", glyphFilm},      // media tool (slate)
	"mcp":             {"#5A6B86", glyphChain},     // protocol (slate)
	"github actions":  {"#5A6B86", glyphGear},      // CI (slate)
}

// detectionOrder lists keys so longer/more-specific ones win (e.g. "step
// functions" before a bare "step").
var detectionOrder = []string{
	// Multi-word / more-specific keys first so they win over a contained substring
	// (e.g. "github actions" before "github", "step functions" before "step").
	"openclaw", "github actions", "secrets manager", "step functions", "step function",
	"api gateway", "cloudformation", "cloudwatch", "eventbridge", "dynamodb",
	"fargate", "lambda", "bedrock", "anthropic", "replicate", "nova", "polly",
	"claude", "ollama", "docker", "ffmpeg", "github", "webhook", "n8n", "mcp",
	"ecr", "ecs", "iam", "kms", "sns", "sqs", "efs", "ec2", "s3", "claw",
}

// Detect returns the ordered, de-duplicated component keys named in text, capped
// at max (0 = no cap). Ordering is by first appearance so the diagram reads the
// way the scene describes the flow.
func Detect(text string, max int) []string {
	l := strings.ToLower(text)
	type hit struct {
		idx, end int
		key      string
	}
	var hits []hit
	seen := map[string]bool{}
	for _, key := range detectionOrder {
		canon := canonical(key)
		if seen[canon] {
			continue
		}
		i := strings.Index(l, key)
		if i < 0 {
			continue
		}
		end := i + len(key)
		// Skip a match contained within a longer, already-accepted one (e.g. the
		// "github" inside "github actions"). detectionOrder lists the longer/more
		// specific keys first, so the containing match is accepted before this one.
		contained := false
		for _, h := range hits {
			if i >= h.idx && end <= h.end {
				contained = true
				break
			}
		}
		if contained {
			continue
		}
		seen[canon] = true
		hits = append(hits, hit{i, end, canon})
	}
	// stable sort by appearance
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].idx < hits[j-1].idx; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.key)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out
}

// canonical collapses aliases (step function/step functions, claude→bedrock-ish
// stays distinct visually but n8n/ollama stay their own tiles).
func canonical(key string) string {
	switch key {
	case "step function":
		return "step functions"
	case "claw": // the diagram node-id is the orchestrator OpenClaw
		return "openclaw"
	}
	return key
}

// Has reports whether text names at least one component we can diagram.
func Has(text string) bool { return len(Detect(text, 1)) > 0 }

// FlowRank returns a component's architectural role rank (source→sink; see
// flowRank). Unknown components rank last. Exported so callers can order a set of
// components into a coherent flow without hardcoding relationships.
func FlowRank(key string) int {
	if r, ok := flowRank[key]; ok {
		return r
	}
	return 99
}

// SVG builds the full scene-background SVG (w×h) for the components named in
// text: brand background + grid, the service tiles laid out along the flow
// direction (a column for portrait, a row for landscape), connected by arrows.
// Returns "" if no component is recognized (caller falls back).
func SVG(text string, w, h int, phase float64) string {
	return Compose(Detect(text, 4), w, h, phase)
}

// flowRank orders components by their architectural role, source-to-sink, so an
// overview reads as the real flow: source → ingress → compute → workflow →
// orchestrator → inference → storage → observability → security.
var flowRank = map[string]int{
	"github": 0, "webhook": 0, "github actions": 0,
	"api gateway": 1, "eventbridge": 1, "sqs": 1, "step functions": 1, "sns": 1,
	"lambda": 2, "ec2": 2, "fargate": 2, "ecs": 2, "docker": 2, "ffmpeg": 2,
	"n8n": 3, "mcp": 3,
	"openclaw": 4,
	"ollama":   5, "bedrock": 5, "claude": 5, "anthropic": 5, "replicate": 5, "nova": 5, "polly": 5,
	"efs": 6, "s3": 6, "dynamodb": 6, "ecr": 6,
	"cloudwatch": 7, "cloudformation": 7,
	"iam": 8, "kms": 8, "secrets manager": 8,
}

// FlowSort returns keys ordered by architectural role (see flowRank), so a set of
// components collected from a release renders as a coherent source-to-sink spine.
// Ties keep input order; unknown keys sort last.
func FlowSort(keys []string) []string {
	out := append([]string(nil), keys...)
	rank := func(k string) int {
		if r, ok := flowRank[k]; ok {
			return r
		}
		return 99
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && rank(out[j]) < rank(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Compose renders the branded architecture SVG for an explicit, ordered set of
// component keys (bypassing text detection) — used for the opening system
// overview. Returns "" for an empty set.
func Compose(keys []string, w, h int, phase float64) string {
	if len(keys) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, w, h, w, h)
	// background gradient + faint grid
	fmt.Fprintf(&b, `<defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`, bgTop, bgBottom)
	b.WriteString(`<pattern id="grid" width="64" height="64" patternUnits="userSpaceOnUse">`)
	fmt.Fprintf(&b, `<path d="M64 0H0V64" fill="none" stroke="%s" stroke-width="1"/></pattern></defs>`, grid)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="url(#bg)"/><rect width="%d" height="%d" fill="url(#grid)" opacity="0.5"/>`, w, h, w, h)

	portrait := h >= w
	n := len(keys)
	tileSize := 220
	// A many-node overview uses smaller tiles and the full frame (there is no
	// caption band to clear); a few-node scene keeps large tiles.
	topPct, bottomPct := 18, 66
	switch {
	case n >= 5:
		tileSize = 150
		topPct, bottomPct = 9, 90
	case portrait && n >= 3:
		tileSize = 200
	}
	// node centres along the primary axis, centred on the cross axis.
	cx, cy := make([]int, n), make([]int, n)
	if portrait {
		top, bottom := h*topPct/100, h*bottomPct/100
		for i := 0; i < n; i++ {
			cx[i] = w / 2
			if n == 1 {
				cy[i] = (top + bottom) / 2
			} else {
				cy[i] = top + (bottom-top)*i/(n-1)
			}
		}
	} else {
		left, right := w*14/100, w*86/100
		for i := 0; i < n; i++ {
			cy[i] = h * 42 / 100
			if n == 1 {
				cx[i] = w / 2
			} else {
				cx[i] = left + (right-left)*i/(n-1)
			}
		}
	}
	// flow arrows first (behind tiles), with particles at this animation phase
	for i := 0; i+1 < n; i++ {
		b.WriteString(arrow(cx[i], cy[i], cx[i+1], cy[i+1], tileSize/2, phase))
	}
	// tiles: official AWS icon where we have one, else the stylized glyph
	for i, k := range keys {
		b.WriteString(drawNode(i, cx[i], cy[i], tileSize, k))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// drawNode renders one node: the official AWS icon (rounded-clipped, on the
// scene) when available, otherwise the stylized fallback tile.
func drawNode(i, cx, cy, size int, key string) string {
	if inner, ok := officialIcon(key, fmt.Sprintf("n%d_", i)); ok {
		x, y := cx-size/2, cy-size/2
		rad := size / 6
		clip := fmt.Sprintf("clip%d", i)
		return fmt.Sprintf(
			`<g><defs><clipPath id="%s"><rect x="%d" y="%d" width="%d" height="%d" rx="%d"/></clipPath></defs>`+
				`<g clip-path="url(#%s)"><svg x="%d" y="%d" width="%d" height="%d" viewBox="0 0 80 80" preserveAspectRatio="xMidYMid meet">%s</svg></g>`+
				`<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="none" stroke="%s" stroke-width="3"/></g>`,
			clip, x, y, size, size, rad, clip, x, y, size, size, inner, x, y, size, size, rad, stroke)
	}
	return drawTile(cx, cy, size, catalog[key])
}

// arrow draws a glowing flow line + arrowhead from node a to node b, trimmed so
// it starts/ends at the tile edge (r = half tile size).
func arrow(ax, ay, bx, by, r int, phase float64) string {
	dx, dy := float64(bx-ax), float64(by-ay)
	d := hypot(dx, dy)
	if d == 0 {
		return ""
	}
	ux, uy := dx/d, dy/d
	x1, y1 := float64(ax)+ux*float64(r+8), float64(ay)+uy*float64(r+8)
	x2, y2 := float64(bx)-ux*float64(r+22), float64(by)-uy*float64(r+22)
	// arrowhead
	hx, hy := float64(bx)-ux*float64(r+8), float64(by)-uy*float64(r+8)
	px, py := -uy, ux
	a1x, a1y := hx-ux*22+px*13, hy-uy*22+py*13
	a2x, a2y := hx-ux*22-px*13, hy-uy*22-py*13
	var b strings.Builder
	fmt.Fprintf(&b,
		`<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="6" stroke-linecap="round" opacity="0.55"/>`+
			`<polygon points="%.0f,%.0f %.0f,%.0f %.0f,%.0f" fill="%s"/>`,
		x1, y1, x2, y2, flow, hx, hy, a1x, a1y, a2x, a2y, flow)
	// Flow particles: dots travelling start→end, spaced so the loop is seamless.
	const dots = 2
	for k := 0; k < dots; k++ {
		t := phase + float64(k)/dots
		t -= math.Floor(t)
		cxk := x1 + (x2-x1)*t
		cyk := y1 + (y2-y1)*t
		fmt.Fprintf(&b,
			`<circle cx="%.0f" cy="%.0f" r="17" fill="%s" opacity="0.22"/><circle cx="%.0f" cy="%.0f" r="8" fill="%s"/>`,
			cxk, cyk, flow, cxk, cyk, flow)
	}
	return b.String()
}

// drawTile renders one service tile centred at (cx,cy) with side `size`.
func drawTile(cx, cy, size int, t tile) string {
	x, y := cx-size/2, cy-size/2
	rad := size / 6
	var b strings.Builder
	// soft glow + rounded tile with a subtle top highlight
	fmt.Fprintf(&b, `<g><rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="%s" stroke="%s" stroke-width="3"/>`,
		x, y, size, size, rad, t.color, stroke)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="#FFFFFF" opacity="0.10"/>`,
		x, y, size, size*2/5, rad)
	// glyph, scaled into the central 60% of the tile
	gs := size * 60 / 100
	gx, gy := cx-gs/2, cy-gs/2
	fmt.Fprintf(&b, `<g transform="translate(%d,%d) scale(%.3f)">%s</g></g>`, gx, gy, float64(gs)/100.0, t.glyph)
	return b.String()
}

func hypot(a, b float64) float64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	// good-enough integer-scale hypot without importing math for one call
	s := a
	if b > s {
		s = b
	}
	x, y := a/s, b/s
	return s * (1 + (x*x+y*y-1)/2) // 1st-order; distances here are coarse layout only
}

// --- glyphs: drawn in a 0..100 box, light strokes/fills (glyphOn) ---

const (
	glyphBolt     = `<path d="M55 8 L28 56 H48 L42 92 L74 40 H52 Z" fill="` + glyphOn + `"/>`
	glyphEventBus = `<circle cx="50" cy="50" r="12" fill="` + glyphOn + `"/><g stroke="` + glyphOn + `" stroke-width="7" stroke-linecap="round"><line x1="50" y1="12" x2="50" y2="30"/><line x1="50" y1="70" x2="50" y2="88"/><line x1="12" y1="50" x2="30" y2="50"/><line x1="70" y1="50" x2="88" y2="50"/><line x1="24" y1="24" x2="37" y2="37"/><line x1="63" y1="63" x2="76" y2="76"/></g>`
	glyphBranch   = `<g stroke="` + glyphOn + `" stroke-width="7" fill="none"><path d="M20 50 H45 M45 50 C45 25 75 25 75 25 M45 50 C45 75 75 75 75 75"/></g><g fill="` + glyphOn + `"><circle cx="20" cy="50" r="9"/><circle cx="78" cy="25" r="9"/><circle cx="78" cy="75" r="9"/></g>`
	glyphQueue    = `<rect x="16" y="38" width="68" height="24" rx="6" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g fill="` + glyphOn + `"><rect x="26" y="44" width="10" height="12"/><rect x="45" y="44" width="10" height="12"/><rect x="64" y="44" width="10" height="12"/></g>`
	glyphGateway  = `<path d="M25 30 Q50 10 75 30" fill="none" stroke="` + glyphOn + `" stroke-width="7"/><rect x="30" y="34" width="40" height="46" rx="6" fill="none" stroke="` + glyphOn + `" stroke-width="7"/>`
	glyphChip     = `<rect x="30" y="30" width="40" height="40" rx="4" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g stroke="` + glyphOn + `" stroke-width="6"><line x1="42" y1="18" x2="42" y2="30"/><line x1="58" y1="18" x2="58" y2="30"/><line x1="42" y1="70" x2="42" y2="82"/><line x1="58" y1="70" x2="58" y2="82"/><line x1="18" y1="42" x2="30" y2="42"/><line x1="18" y1="58" x2="30" y2="58"/><line x1="70" y1="42" x2="82" y2="42"/><line x1="70" y1="58" x2="82" y2="58"/></g>`
	glyphAI       = `<circle cx="50" cy="50" r="26" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><circle cx="50" cy="50" r="10" fill="` + glyphOn + `"/><g fill="` + glyphOn + `"><circle cx="50" cy="18" r="6"/><circle cx="82" cy="50" r="6"/><circle cx="50" cy="82" r="6"/><circle cx="18" cy="50" r="6"/></g>`
	glyphNode     = `<circle cx="50" cy="50" r="20" fill="` + glyphOn + `"/>`
	// glyphAgent: an orchestrator hub — a solid core dispatching along three arms to
	// worker nodes. Marks OpenClaw as the central component that routes the flow.
	glyphAgent = `<g stroke="` + glyphOn + `" stroke-width="7" fill="none"><path d="M50 50 L50 20 M50 50 L24 74 M50 50 L76 74"/></g><g fill="` + glyphOn + `"><circle cx="50" cy="50" r="14"/><circle cx="50" cy="18" r="8"/><circle cx="22" cy="76" r="8"/><circle cx="78" cy="76" r="8"/></g>`
	glyphChain = `<g fill="` + glyphOn + `"><circle cx="24" cy="50" r="11"/><circle cx="50" cy="50" r="11"/><circle cx="76" cy="50" r="11"/></g><g stroke="` + glyphOn + `" stroke-width="6"><line x1="35" y1="50" x2="39" y2="50"/><line x1="61" y1="50" x2="65" y2="50"/></g>`
	// Additional stylized glyphs so every technology the platform uses has a tile.
	glyphContainer = `<rect x="20" y="26" width="60" height="48" rx="5" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><line x1="20" y1="42" x2="80" y2="42" stroke="` + glyphOn + `" stroke-width="6"/><circle cx="31" cy="34" r="3" fill="` + glyphOn + `"/><g fill="` + glyphOn + `"><rect x="30" y="52" width="12" height="14"/><rect x="46" y="52" width="12" height="14"/></g>`
	glyphWave      = `<g stroke="` + glyphOn + `" stroke-width="7" stroke-linecap="round"><line x1="24" y1="42" x2="24" y2="58"/><line x1="38" y1="30" x2="38" y2="70"/><line x1="52" y1="22" x2="52" y2="78"/><line x1="66" y1="34" x2="66" y2="66"/><line x1="80" y1="44" x2="80" y2="56"/></g>`
	glyphKey       = `<circle cx="34" cy="50" r="16" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g stroke="` + glyphOn + `" stroke-width="6" stroke-linecap="round"><line x1="48" y1="50" x2="82" y2="50"/><line x1="70" y1="50" x2="70" y2="64"/><line x1="82" y1="50" x2="82" y2="62"/></g>`
	glyphStack     = `<g fill="none" stroke="` + glyphOn + `" stroke-width="6" stroke-linejoin="round"><path d="M50 20 L82 34 L50 48 L18 34 Z"/><path d="M18 50 L50 64 L82 50"/><path d="M18 64 L50 78 L82 64"/></g>`
	glyphFilm      = `<rect x="20" y="28" width="60" height="44" rx="5" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g fill="` + glyphOn + `"><rect x="26" y="34" width="8" height="8"/><rect x="26" y="58" width="8" height="8"/><rect x="66" y="34" width="8" height="8"/><rect x="66" y="58" width="8" height="8"/></g><line x1="44" y1="28" x2="44" y2="72" stroke="` + glyphOn + `" stroke-width="5"/><line x1="56" y1="28" x2="56" y2="72" stroke="` + glyphOn + `" stroke-width="5"/>`
	glyphGear      = `<circle cx="50" cy="50" r="13" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g stroke="` + glyphOn + `" stroke-width="8" stroke-linecap="round"><line x1="50" y1="20" x2="50" y2="30"/><line x1="50" y1="70" x2="50" y2="80"/><line x1="20" y1="50" x2="30" y2="50"/><line x1="70" y1="50" x2="80" y2="50"/><line x1="29" y1="29" x2="36" y2="36"/><line x1="64" y1="64" x2="71" y2="71"/><line x1="71" y1="29" x2="64" y2="36"/><line x1="36" y1="64" x2="29" y2="71"/></g>`
	glyphCast      = `<g fill="none" stroke="` + glyphOn + `" stroke-width="6" stroke-linecap="round"><path d="M30 38 A16 16 0 0 1 30 62"/><path d="M42 30 A28 28 0 0 1 42 70"/></g><circle cx="24" cy="50" r="7" fill="` + glyphOn + `"/><line x1="58" y1="50" x2="78" y2="50" stroke="` + glyphOn + `" stroke-width="6"/><circle cx="80" cy="50" r="7" fill="` + glyphOn + `"/>`
	glyphBucket    = `<path d="M22 30 H78 L72 82 H28 Z" fill="none" stroke="` + glyphOn + `" stroke-width="7"/><line x1="22" y1="30" x2="78" y2="30" stroke="` + glyphOn + `" stroke-width="7"/>`
	glyphDisc      = `<ellipse cx="50" cy="30" rx="30" ry="12" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><path d="M20 30 V70 A30 12 0 0 0 80 70 V30" fill="none" stroke="` + glyphOn + `" stroke-width="6"/>`
	glyphDb        = `<ellipse cx="50" cy="26" rx="28" ry="11" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><path d="M22 26 V74 A28 11 0 0 0 78 74 V26 M22 50 A28 11 0 0 0 78 50" fill="none" stroke="` + glyphOn + `" stroke-width="6"/>`
	glyphGauge     = `<path d="M20 66 A30 30 0 0 1 80 66" fill="none" stroke="` + glyphOn + `" stroke-width="7"/><line x1="50" y1="66" x2="66" y2="42" stroke="` + glyphOn + `" stroke-width="7" stroke-linecap="round"/><circle cx="50" cy="66" r="6" fill="` + glyphOn + `"/>`
	glyphShield    = `<path d="M50 14 L80 26 V52 C80 72 66 82 50 88 C34 82 20 72 20 52 V26 Z" fill="none" stroke="` + glyphOn + `" stroke-width="7"/><path d="M38 50 L47 60 L64 40" fill="none" stroke="` + glyphOn + `" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>`
	glyphRepo      = `<rect x="22" y="20" width="56" height="60" rx="6" fill="none" stroke="` + glyphOn + `" stroke-width="6"/><g stroke="` + glyphOn + `" stroke-width="6" stroke-linecap="round"><line x1="34" y1="34" x2="60" y2="34"/><line x1="34" y1="50" x2="60" y2="50"/><line x1="34" y1="66" x2="50" y2="66"/></g>`
	glyphPulse     = `<path d="M14 50 H34 L42 28 L54 72 L62 50 H86" fill="none" stroke="` + glyphOn + `" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>`
)
