package promptversion

import "testing"

func TestForKnownAndUnknown(t *testing.T) {
	if got := For("blog"); got != "blog@6" {
		t.Errorf("blog = %q, want blog@6", got)
	}
	if got := For("architecture-diagram-spec"); got != "architecture-diagram-spec@1" {
		t.Errorf("diagram-spec = %q", got)
	}
	// Unregistered kinds default to revision 1.
	if got := For("brand-new-kind"); got != "brand-new-kind@1" {
		t.Errorf("unknown = %q, want brand-new-kind@1", got)
	}
}
