package archspec

import (
	"strings"
	"testing"
)

// The prompt must (a) pin the exact Component bullet shape so the diagram parser
// reads component names deterministically, and (b) forbid the degenerate
// single-node / no-connection specs that render as an empty diagram.
func TestPromptPinsComponentShapeAndMinimumGraph(t *testing.T) {
	p := (&Generator{}).prompt(ReleasePackage{Context: richContext()})
	for _, want := range []string{
		"- **<canonical Name>**", // exact Component header shape
		"  - AWS Service:",       // field sub-bullet shape
		"one node is not an architecture",
		"you MUST populate the Connections section",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing required guidance: %q", want)
		}
	}
}

func TestReleaseTopicStripsPlatformSuffix(t *testing.T) {
	cases := map[string]string{
		// The permanent-platform classification is removed; the release change stays.
		"Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs for an Event-Driven AI Agent Platform": "Optimizing Spot Startup on AWS: Pre-Baked Custom AMIs",
		"Fast Spot Startup on a Hybrid AI platform":                                                   "Fast Spot Startup",
		"Webhook automation within an AWS-native AI platform":                                         "Webhook automation",
		// No platform suffix → unchanged.
		"Optimizing Spot startup on AWS": "Optimizing Spot startup on AWS",
		"Pre-Baked Custom AMIs":          "Pre-Baked Custom AMIs",
	}
	for in, want := range cases {
		if got := releaseTopic(in); got != want {
			t.Errorf("releaseTopic(%q) = %q, want %q", in, got, want)
		}
	}
}
