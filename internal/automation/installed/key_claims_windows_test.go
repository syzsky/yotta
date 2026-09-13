package installed

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	pkginput "github.com/yottaapp/yotta/pkg/input"
)

type claimPhysicalFake struct {
	pkginput.Backend
	events           []string
	down             map[uint32]bool
	failDown, failUp uint32
	onDown           func(uint32)
	global           bool
	moves            int
}

func (f *claimPhysicalFake) Name() string { return "postmessage" }
func (f *claimPhysicalFake) MouseMoveRel(_ pkginput.Handle, _, _, _ int) error {
	f.moves++
	return nil
}

func (f *claimPhysicalFake) Capabilities() pkginput.Capabilities {
	return pkginput.Capabilities{GlobalInput: f.global}
}
func (f *claimPhysicalFake) KeyDownCode(_ pkginput.Handle, code uint32) error {
	if f.failDown == code {
		return errors.New("injected down failure")
	}
	if f.down == nil {
		f.down = map[uint32]bool{}
	}
	f.down[code] = true
	f.events = append(f.events, fmt.Sprintf("down:%d", code))
	if f.onDown != nil {
		f.onDown(code)
	}
	return nil
}
func (f *claimPhysicalFake) KeyUpCode(_ pkginput.Handle, code uint32) error {
	if f.failUp == code {
		f.failUp = 0
		return errors.New("injected up failure")
	}
	delete(f.down, code)
	f.events = append(f.events, fmt.Sprintf("up:%d", code))
	return nil
}
func (f *claimPhysicalFake) ReleaseAll() error {
	for code := range f.down {
		if err := f.KeyUpCode(0, code); err != nil {
			return err
		}
	}
	return nil
}
func (f *claimPhysicalFake) Close() error { return f.ReleaseAll() }

func TestKeyClaimsRepeatedDownAndRetry(t *testing.T) {
	physical := &claimPhysicalFake{global: true}
	keys := &keyClaims{physical: physical}
	a, b := keys.input(&claimPhysicalFake{}), keys.input(&claimPhysicalFake{})
	for _, input := range []*claimedInput{a, a, b} {
		if err := input.KeyDown(1, "w"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.KeyUpCode(2, 87); err != nil {
		t.Fatal(err)
	}
	if err := b.ReleaseAll(); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] {
		t.Fatal("child released live W")
	}
	physical.failUp = 87
	if err := a.Close(); err == nil {
		t.Fatal("expected release failure")
	}
	if len(a.held) != 1 || keys.counts[claimedKey{0, 87}] != 1 {
		t.Fatal("failed release lost claim")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if len(physical.down) != 0 || len(keys.counts) != 0 {
		t.Fatal("claims leaked")
	}
	if want := []string{"down:87", "down:87", "down:87", "up:87"}; !reflect.DeepEqual(physical.events, want) {
		t.Fatalf("events %v", physical.events)
	}
}

func TestKeyClaimsWindowIdentityAndDriverClose(t *testing.T) {
	physical := &claimPhysicalFake{}
	keys := &keyClaims{physical: physical}
	a, b := keys.input(&claimPhysicalFake{}), keys.input(&claimPhysicalFake{})
	if err := a.KeyDown(1, "W"); err != nil {
		t.Fatal(err)
	}
	if err := b.KeyDownCode(2, 87); err != nil {
		t.Fatal(err)
	}
	if len(keys.counts) != 2 {
		t.Fatal("targeted windows conflated")
	}
	if err := a.KeyUp(2, "W"); err != nil {
		t.Fatal(err)
	}
	if len(a.held) != 1 {
		t.Fatal("unowned up removed claim")
	}
	if err := keys.close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.KeyDown(2, "W"); err == nil {
		t.Fatal("closed driver accepted input")
	}
	if len(keys.counts) != 0 || len(a.held) != 0 || len(b.held) != 0 {
		t.Fatal("teardown leaked claims")
	}
}

func TestKeyClaimsConcurrentRelease(t *testing.T) {
	physical := &claimPhysicalFake{global: true}
	keys := &keyClaims{physical: physical}
	held := keys.input(&claimPhysicalFake{})
	if err := held.KeyDown(1, "W"); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			child := keys.input(&claimPhysicalFake{})
			for range 20 {
				if err := child.KeyDown(1, "W"); err != nil {
					t.Error(err)
				}
				if err := child.ReleaseAll(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	workers.Wait()
	for _, event := range physical.events {
		if event == "up:87" {
			t.Fatal("concurrent child released held W")
		}
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if len(keys.counts) != 0 || len(physical.down) != 0 {
		t.Fatal("concurrent claims leaked")
	}
}
