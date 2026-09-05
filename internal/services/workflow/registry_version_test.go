package workflow

import (
	"context"
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
	"testing"
)

func TestMalformedReleaseVersionIsRejectedBeforeAuthentication(t *testing.T) {
	s := &Service{}
	for _, version := range []string{"", "1", "1.2", "1.2.3.4", "-1.0.0", "01.0.0", "1.0.0-beta", "1.0.0+build", "1.0.0\n"} {
		_, err := s.PublishSourceToRegistry(context.Background(), PublishRegistryRequest{ReleaseVersion: version, Title: "Test", Summary: "Test"})
		if apperr.From(err).ID != "workflow.registry.invalid_version" {
			t.Fatalf("%q: %v", version, err)
		}
	}
	for _, version := range []string{"0.0.0", "1.0.0", "12.34.567"} {
		if !registryclient.ValidWorkflowReleaseVersion(version) {
			t.Fatal(version)
		}
	}
}
