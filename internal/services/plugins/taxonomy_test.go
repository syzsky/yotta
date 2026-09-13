package plugins

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/registryclient"
)

type taxonomyRegistry struct {
	Registry
	kind string
	err  error
}

func TestRegistryTaxonomyPreservesCancellationAndTimeout(t *testing.T) {
	for _, test := range []struct {
		cause     error
		id        string
		retryable bool
	}{{context.Canceled, "plugins.market_cancelled", false}, {context.DeadlineExceeded, "plugins.market_timeout", true}} {
		t.Run(test.id, func(t *testing.T) {
			s := &Service{registry: &taxonomyRegistry{err: fmt.Errorf("private-token /private/path: %w", test.cause)}}
			_, err := s.RegistryTaxonomy(context.Background())
			problem := apperr.Project(err)
			if problem.ID != test.id || problem.Retryable != test.retryable {
				t.Fatalf("problem=%+v", problem)
			}
			if !errors.Is(err, test.cause) {
				t.Fatal("lost transport cause")
			}
			raw := string(apperr.Marshal(err))
			if strings.Contains(raw, "private-token") || strings.Contains(raw, "/private/path") {
				t.Fatal("raw transport cause leaked")
			}
		})
	}
}

func (r *taxonomyRegistry) TaxonomyProfile(_ context.Context, kind string) (registryclient.TaxonomyProfile, error) {
	r.kind = kind
	return registryclient.TaxonomyProfile{Kind: kind, Revision: 2}, r.err
}

func TestRegistryTaxonomySelectsPluginProfileAndProjectsFailures(t *testing.T) {
	registry := &taxonomyRegistry{}
	s := &Service{registry: registry}
	profile, err := s.RegistryTaxonomy(context.Background())
	if err != nil || registry.kind != "node-pack" || profile.Kind != "node-pack" {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	registry.err = errors.New("private transport failure")
	_, err = s.RegistryTaxonomy(context.Background())
	if problem := apperr.Project(err); problem.ID != "plugins.market_unavailable" || !problem.Retryable {
		t.Fatalf("problem=%+v", problem)
	}
	s.registry = nil
	_, err = s.RegistryTaxonomy(context.Background())
	if problem := apperr.Project(err); problem.ID != "plugins.market_unavailable" {
		t.Fatalf("problem=%+v", problem)
	}
}
