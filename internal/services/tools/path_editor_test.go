package tools

import (
	"context"
	"testing"
)

func TestPathEditorReusesWindowAndCleansSampling(t *testing.T) {
	p := &fakePresenter{ready: true}
	stopped := 0
	s := NewServiceWithOptions(nil, p, Options{OnPathEditorClose: func() { stopped++ }})
	if err := s.OpenPathEditor("route-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenPathEditor("route-b"); err != nil {
		t.Fatal(err)
	}
	if len(p.requests) != 1 || p.requests[0].GUID != "route-a" || p.requests[0].Kind != WindowPathEditor {
		t.Fatalf("requests: %+v", p.requests)
	}
	if p.window.focusCalls != 1 || p.emitted[len(p.emitted)-1] != "path-editor:open" {
		t.Fatal("existing editor was not notified/focused")
	}
	if err := s.ClosePathEditor(); err != nil {
		t.Fatal(err)
	}
	if stopped != 1 || p.window.closeCalls != 1 {
		t.Fatalf("close=%d, stopped=%d", p.window.closeCalls, stopped)
	}
	p.window.onClosing()
	if stopped != 1 {
		t.Fatal("duplicate cleanup")
	}
	if err := s.OpenPathEditor(""); err != nil {
		t.Fatal(err)
	}
	if err := Shutdown(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if stopped != 2 {
		t.Fatal("shutdown did not stop sampling")
	}
	if err := s.OpenPathEditor(""); err == nil {
		t.Fatal("reopened after shutdown")
	}
}
