package registryclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFilterCatalogAndRepeatedQueryValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/filter-catalog":
			w.Write([]byte(`{"revision":2,"dimensions":[{"id":"platform","name":"Platform","active":true,"values":[{"id":"windows","name":"Windows","parentId":"desktop","active":true}]}]}`))
		case "/v1/catalog/search":
			if !reflect.DeepEqual(r.URL.Query()["filterValue"], []string{"desktop", "reward"}) {
				t.Errorf("query=%s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"items":[],"facets":{"categories":[],"tags":[]}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := New(Options{BaseURL: server.URL, AllowLoopbackHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := client.FilterCatalog(context.Background())
	if err != nil || len(catalog.Dimensions) != 1 || catalog.Dimensions[0].Values[0].ParentID != "desktop" {
		t.Fatalf("catalog=%#v %v", catalog, err)
	}
	if _, err = client.SearchCatalog(context.Background(), SearchOptions{FilterValues: []string{"desktop", "reward"}}); err != nil {
		t.Fatal(err)
	}
}
