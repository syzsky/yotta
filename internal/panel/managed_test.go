package panel

import (
	"context"
	"errors"
	contract "github.com/yottaapp/yotta/sdk/plugin/panel"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestManagedDefinitionsPersistAndReferencesDoNotCollide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panels.json")
	s, e := Open(nil, path)
	if e != nil {
		t.Fatal(e)
	}
	d, e := s.Save(Draft{Title: "Same name", Components: []ComponentDraft{{Kind: "toggle", Title: "Switch", Icon: "i-tabler-star", Initial: false}}})
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Save(Draft{Title: "Same name", Components: []ComponentDraft{{Kind: "toggle", Title: "Switch", Initial: false}}})
	if e != nil {
		t.Fatal(e)
	}
	if d.ID == second.ID || d.Components[0].ID == second.Components[0].ID {
		t.Fatal("identities collided")
	}
	ref, _ := s.Resolve(d.ID)
	c, e := s.ResolveComponent(ref, d.Components[0].ID, "boolean")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Write(c, true); e != nil {
		t.Fatal(e)
	}
	s.MarkUpdated(d.ID, "run")
	s.EndRun("run", "succeeded")
	state, e := s.Read(d.ID)
	if e != nil || state.Values[c.ID] != true || state.Status != "ready" {
		t.Fatalf("run termination changed panel: %+v %v", state, e)
	}
	d.Title = "Renamed"
	d.Components[0].Icon = "i-tabler-bolt"
	if d, e = s.Save(d); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ResolveComponent(ref, c.ID, "boolean"); e != nil {
		t.Fatal("rename invalidated a live reference", e)
	}
	if v, e := s.Value(c); e != nil || v != true {
		t.Fatal("rename lost the current value", v, e)
	}
	if e = s.Write(c, "wrong"); !errors.Is(e, ErrInvalidValue) {
		t.Fatal(e)
	}
	if _, e = s.ResolveComponent(ref, c.ID, "number"); !errors.Is(e, ErrInvalidValue) {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(nil, path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	got, e := s.Edit(d.ID)
	if e != nil || got.Components[0].ID != c.ID || got.Components[0].Icon != "i-tabler-bolt" {
		t.Fatalf("definition lost: %+v %v", got, e)
	}
	if _, e = s.Save(Draft{ID: d.ID, Revision: 0, Title: "stale"}); e == nil {
		t.Fatal("stale edit accepted")
	}
}
func TestManagedWaitBroadcastCancellationAndLiveControls(t *testing.T) {
	s := New(nil)
	defer s.Close()
	d, e := s.Save(Draft{Title: "Panel", Components: []ComponentDraft{{Kind: "toggle", Title: "Switch", Initial: false}}})
	if e != nil {
		t.Fatal(e)
	}
	ref, _ := s.Resolve(d.ID)
	component := d.Components[0].ID
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, e := s.Wait(ctx, ref, component); first <- e }()
	await := func(count int) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			sources := s.List()
			for _, p := range sources {
				if p.ID == d.ID && p.Waiting == count {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatal("listener state missing")
			}
			time.Sleep(time.Millisecond)
		}
	}
	await(1)
	cancel()
	if e = <-first; !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	await(0)
	results := make(chan Interaction, 2)
	for i := 0; i < 2; i++ {
		go func() {
			e, err := s.Wait(context.Background(), ref, component)
			if err != nil {
				first <- err
			}
			results <- e
		}()
	}
	await(2)
	snapshot, _ := s.Read(d.ID)
	event := contract.Event{SessionID: snapshot.SessionID, EventID: "clicked", ComponentID: component, Name: component + ".change", Value: true}
	if _, e = s.Dispatch(d.ID, event); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.Value != true {
				t.Fatal(result)
			}
		case <-time.After(time.Second):
			t.Fatal("interaction was consumed by another workflow")
		}
	}
	if _, e = s.Dispatch(d.ID, event); e != nil {
		t.Fatal(e)
	}
	state, _ := s.Read(d.ID)
	if state.Values[component] != true {
		t.Fatal(state)
	}
}

func TestPluginPanelIdentityFitsWorkflowTargetSlots(t *testing.T) {
	id := PluginSourceID("org.example/position", "Main.Panel")
	if len(id) > 128 || !regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_][a-z0-9]+)*$`).MatchString(id) {
		t.Fatal("panel ID cannot be saved as a Workflow target default", id)
	}
	if id != PluginSourceID("org.example/position", "Main.Panel") || id == PluginSourceID("another.plugin", "Main.Panel") {
		t.Fatal("panel identity is unstable or collides across owners")
	}
	p := fixture()
	p.ID = PluginSourceID(p.OwnerID, p.Definition.ID)
	s := New(&catalogStub{sources: []Source{p}})
	defer s.Close()
	ref, err := s.Resolve(p.OwnerID + "#" + p.Definition.ID)
	if err != nil || ref.ID != p.ID {
		t.Fatal("legacy explicit selection was lost", ref, err)
	}
}
