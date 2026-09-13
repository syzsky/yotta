//go:build windows

package installed

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/lxn/win"
	"github.com/yottaapp/yotta/internal/appcontrol"
	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

func compositionDriver(t *testing.T, backend string) (*windowsDriver, context.Context) {
	t.Helper()
	if os.Getenv("YOTTA_WINDOWS_NATIVE_SMOKE") != "1" {
		t.Skip("requires dedicated desktop fixture")
	}
	fixture := startNativeFixture(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := SealProfile(NewDesktopProfileDraft(DesktopProfilePayload{
		Application: appcontrol.ProfileDraft{Executable: executable}, WindowTitle: nativeFixtureTitle,
		WindowTitleMatch: "exact", WindowClass: fixture.className, WindowSelection: "unique",
		InputBackend: backend, CaptureBackend: "gdi", ResolveTimeoutMilliseconds: 500,
	}))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := newPlatformDriver(profile)
	if err != nil {
		t.Fatal(err)
	}
	d := raw.(*windowsDriver)
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return d, inputcoord.WithOwner(ctx, inputcoord.NewOwner())
}

func TestWindowsHeldCompositionCallSite(t *testing.T) {
	d, ctx := compositionDriver(t, "postmessage")
	if err := d.keys.physical.Close(); err != nil {
		t.Fatal(err)
	}
	physical := &claimPhysicalFake{global: true}
	d.keys.physical = physical
	held, err := d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Close(); err != nil {
			t.Error(err)
		}
	})
	hold := func() {
		t.Helper()
		if err := held.Execute(ctx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W"}}); err != nil {
			t.Fatal(err)
		}
	}
	hold()
	hold()
	for _, keys := range [][]string{{"W", "Space"}, {"Shift"}, {"F"}} {
		if err := d.Execute(ctx, OperationPressKeys, PressKeysRequest{Keys: keys}); err != nil {
			t.Fatal(err)
		}
		if !physical.down[87] || len(physical.down) != 1 {
			t.Fatalf("child %v changed held keys: %v", keys, physical.down)
		}
	}
	for _, mode := range []string{"down failure", "up failure", "cancel"} {
		child, cancel := context.WithCancel(ctx)
		switch mode {
		case "down failure":
			physical.failDown = 32
		case "up failure":
			physical.failUp = 32
		case "cancel":
			physical.onDown = func(code uint32) {
				if code == 32 {
					cancel()
				}
			}
		}
		err := d.Execute(child, OperationPressKeys, PressKeysRequest{Keys: []string{"W", "Space"}, DurationMilliseconds: 1})
		cancel()
		physical.failDown = 0
		physical.onDown = nil
		if err == nil {
			t.Fatalf("%s succeeded", mode)
		}
		if mode == "cancel" && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if !physical.down[87] || len(physical.down) != 1 {
			t.Fatalf("%s disturbed W: %v", mode, physical.down)
		}
	}
	// A failed held chord must release only its own claims, even on shared W.
	child, err := d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	physical.failDown = 32
	if err := child.Execute(ctx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W", "Space"}}); err == nil {
		t.Fatal("expected held failure")
	}
	physical.failDown = 0
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] {
		t.Fatal("failed held child released W")
	}
	child, err = d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	physical.onDown = func(code uint32) {
		if code == 32 {
			cancel()
		}
	}
	err = child.Execute(canceled, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W", "Space"}})
	cancel()
	physical.onDown = nil
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("held cancellation: %v", err)
	}
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] || len(physical.down) != 1 {
		t.Fatal("canceled held child disturbed W")
	}
	// Two live held handles compose independently, including one handle pausing.
	child, err = d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Execute(ctx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"w"}}); err != nil {
		t.Fatal(err)
	}
	if err := child.(*windowsHeldInput).PauseInput(ctx); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] {
		t.Fatal("pausing child released parent W")
	}
	if err := child.(*windowsHeldInput).ResumeInput(ctx); err != nil {
		t.Fatal(err)
	}
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.ReleaseInput(); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] {
		t.Fatal("ReleaseInput released held W")
	}
	owner := inputcoord.FromContext(ctx)
	if err := owner.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	if len(physical.down) != 0 {
		t.Fatal("pause retained keys")
	}
	// Failed resume remains resumable, without a leaked partial claim or lease.
	physical.failDown = 87
	if err := owner.Resume(ctx); err == nil {
		t.Fatal("expected resume failure")
	}
	physical.failDown = 0
	if err := owner.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if !physical.down[87] {
		t.Fatal("resume did not restore W")
	}
	physical.failUp = 87
	if err := held.Close(); err == nil {
		t.Fatal("expected held close failure")
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if len(physical.down) != 0 || len(d.keys.counts) != 0 {
		t.Fatal("close leaked keys")
	}
	// A different owner can acquire after all held claims finish.
	if err := d.Execute(inputcoord.WithOwner(ctx, inputcoord.NewOwner()), OperationPressKeys, PressKeysRequest{Keys: []string{"F"}}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeHeldKeyComposition(t *testing.T) {
	for _, backend := range []string{"postmessage", "sendinput"} {
		t.Run(backend, func(t *testing.T) { nativeHeldKeyComposition(t, backend) })
	}
}

func nativeHeldKeyComposition(t *testing.T, backend string) {
	d, ctx := compositionDriver(t, backend)
	held, err := d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	mark := nativeFixtureMark()
	for range 2 {
		if err := held.Execute(ctx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W"}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, keys := range [][]string{{"W", "Space"}, {"Shift"}, {"F"}} {
		if err := d.Execute(ctx, OperationPressKeys, PressKeysRequest{Keys: keys, DurationMilliseconds: 10}); err != nil {
			t.Fatal(err)
		}
	}
	waitNativeFixtureEvents(t, mark, func(events []nativeFixtureEvent) bool { return hasNativeFixtureEvent(events, win.WM_KEYUP, 'F') })
	nativeFixtureState.Lock()
	events := append([]nativeFixtureEvent(nil), nativeFixtureState.events[mark:]...)
	nativeFixtureState.Unlock()
	downs := 0
	for _, event := range events {
		// An active IME replaces WM_KEYDOWN's VK with VK_PROCESSKEY (229),
		// while retaining W's scan code. Observe the physical key in either form.
		isW := event.wParam == 'W' || (event.wParam == 229 && (event.lParam>>16)&0xff == 0x11)
		if isW && event.message == win.WM_KEYUP {
			t.Fatal("child physically released held W")
		}
		if isW && event.message == win.WM_KEYDOWN {
			downs++
		}
	}
	if downs != 3 {
		t.Fatalf("repeated downs lost: %d; events=%+v", downs, events)
	}
	asyncKeyState := syscall.NewLazyDLL("user32.dll").NewProc("GetAsyncKeyState")
	if backend == "sendinput" {
		state, _, _ := asyncKeyState.Call('W')
		if state&0x8000 == 0 {
			t.Fatal("physical W state was released during composition")
		}
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	waitNativeFixtureEvents(t, mark, func(events []nativeFixtureEvent) bool { return hasNativeFixtureEvent(events, win.WM_KEYUP, 'W') })
	if backend == "sendinput" {
		state, _, _ := asyncKeyState.Call('W')
		if state&0x8000 != 0 {
			t.Fatal("physical W remained held after Close")
		}
	}
}
