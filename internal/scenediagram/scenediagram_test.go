package scenediagram

import (
	"strings"
	"testing"
)

func TestDetectOrdersByAppearanceAndDedups(t *testing.T) {
	got := Detect("From GitHub webhook to Agent Run: EventBridge to Lambda path", 0)
	want := []string{"github", "webhook", "eventbridge", "lambda"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Detect = %v, want %v", got, want)
	}
	// "step functions" wins over a partial and dedups its alias.
	if g := Detect("Step Functions orchestrates the Step Functions run", 0); len(g) != 1 || g[0] != "step functions" {
		t.Errorf("dedup/specific = %v", g)
	}
	// cap
	if g := Detect("eventbridge lambda s3 sqs ec2", 2); len(g) != 2 {
		t.Errorf("cap = %v", g)
	}
}

func TestSVGContainsNodesOrEmpty(t *testing.T) {
	svg := SVG("EventBridge triggers Lambda", 1080, 1920)
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Fatalf("not an svg: %.40q", svg)
	}
	// Two official AWS icons are composed (nested <svg viewBox="0 0 80 80">) and
	// Lambda's official compute-orange background colour is present.
	if strings.Count(svg, `viewBox="0 0 80 80"`) < 2 {
		t.Errorf("expected two nested official icons, got %d", strings.Count(svg, `viewBox="0 0 80 80"`))
	}
	if !strings.Contains(svg, "#ED7100") {
		t.Errorf("expected Lambda official compute-orange in svg")
	}
	if Has("just some prose with no services") || SVG("no services here", 1080, 1920) != "" {
		t.Errorf("expected empty diagram when nothing is detected")
	}
}

func TestOfficialIconIDsAreNamespaced(t *testing.T) {
	// The same icon twice in one scene must not produce duplicate ids.
	svg := SVG("Lambda calls another Lambda... and one more Lambda", 1080, 1920)
	if strings.Contains(svg, `id="Icon-Architecture`) {
		t.Errorf("official icon ids must be namespaced, not raw")
	}
}
