package registryclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTaxonomyAndSystemFacetSearchKeepKindBoundaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/taxonomy/profiles/node-pack":
			_, _ = w.Write([]byte(`{"kind":"node-pack","revision":3,"categories":[{"key":"plugins"}],"dimensions":[],"systemFacets":[{"id":"node-pack.operating-system","source":"system"}]}`))
		case "/v1/catalog/search":
			if r.URL.Query().Get("kinds") == "node-pack" {
				want := []string{"node-pack.operating-system:windows", "node-pack.architecture:amd64"}
				if !reflect.DeepEqual(r.URL.Query()["systemFacet"], want) {
					t.Errorf("system facets: %v", r.URL.Query()["systemFacet"])
				}
				_, _ = w.Write([]byte(`{"items":[],"total":0,"facets":{"categories":[],"tags":[],"systemFacets":[]}}`))
				return
			}
			if r.URL.Query().Get("kinds") != "workflow,node-pack" {
				t.Error("lost kinds")
			}
			_, _ = w.Write([]byte(`{"items":[],"total":3,"totalByKind":{"workflow":1,"node-pack":2},"facetsByKind":{"workflow":{"categories":["flows"],"tags":[]},"node-pack":{"profileRevision":7,"countUnit":"node-pack","categoryCounts":[{"value":"plugins","count":2}],"dimensions":[{"id":"purpose","values":[{"value":"tools","count":2}]}],"categories":["plugins"],"tags":[],"systemFacets":[{"id":"node-pack.operating-system","source":"system","values":[{"value":"windows","count":2}]}]}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := mustClient(t, server.URL, nil)
	profile, err := client.TaxonomyProfile(context.Background(), "node-pack")
	if err != nil || profile.Kind != "node-pack" || len(profile.SystemFacets) != 1 || profile.Categories[0].Key != "plugins" {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	if _, err := client.SearchCatalog(context.Background(), SearchOptions{Kinds: []string{"node-pack"}, SystemFacets: []string{"node-pack.operating-system:windows", "node-pack.architecture:amd64"}}); err != nil {
		t.Fatal(err)
	}
	page, err := client.SearchCatalog(context.Background(), SearchOptions{Kinds: []string{"workflow", "node-pack"}})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalByKind["workflow"] != 1 || page.TotalByKind["node-pack"] != 2 || page.FacetsByKind["node-pack"].ProfileRevision != 7 || page.FacetsByKind["node-pack"].CategoryCounts[0].Count != 2 || page.FacetsByKind["node-pack"].Dimensions[0].Values[0].Count != 2 {
		t.Fatalf("lost query facet metadata: %+v", page)
	}
	if len(page.Facets.Categories) != 0 || page.FacetsByKind["workflow"].Categories[0] != "flows" || page.FacetsByKind["node-pack"].SystemFacets[0].Values[0].Count != 2 {
		t.Fatalf("mixed facets=%+v", page)
	}
}
