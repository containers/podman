package system

import "testing"

func TestPruneOptionsSerializeIncludePinned(t *testing.T) {
	params, err := new(PruneOptions).WithIncludePinned(true).ToParams()
	if err != nil {
		t.Fatalf("serializing prune options: %v", err)
	}

	if got := params.Get("includePinned"); got != "true" {
		t.Fatalf("expected includePinned=true, got %q", got)
	}
}
