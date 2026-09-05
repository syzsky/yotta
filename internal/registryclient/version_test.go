package registryclient

import "testing"

func TestWorkflowUpdatesAreStrictlyNewer(t *testing.T) {
	for _, pair := range [][2]string{{"0.9.0", "1.0.2"}, {"1.0.2", "1.0.2"}, {"invalid", "1.0.0"}} {
		if IsNewerWorkflowVersion(pair[0], pair[1]) {
			t.Fatalf("downgrade considered update: %v", pair)
		}
	}
	if !IsNewerWorkflowVersion("1.10.0", "1.9.0") {
		t.Fatal("numeric ordering failed")
	}
}
