package archspec

import "testing"

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
