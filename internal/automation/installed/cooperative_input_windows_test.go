//go:build windows

package installed

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/inputcoord"
)

func assertCooperativeCameraError(t *testing.T, err error) {
	t.Helper()
	var failure *Failure
	if !errors.Is(err, ErrCooperativeCameraInput) || !errors.As(err, &failure) || failure.Code != CodeInvalidRequest || !strings.Contains(err.Error(), "paused marker") {
		t.Fatalf("expected actionable cooperative camera conflict, got %v", err)
	}
}

func TestWindowsCooperativeCameraRejectedBeforeTargetAcquisition(t *testing.T) {
	ctx := inputcoord.WithOwner(context.Background(), inputcoord.NewCooperatingOwner(inputcoord.NewOwner()))
	// No profile or backend: rejection must precede resolution and input ownership.
	d := &windowsDriver{}
	for _, operation := range []string{
		OperationClick, OperationMove, OperationDrag, OperationScroll, OperationMoveRelative,
		OperationTurnView, OperationTypeText, OperationActivate, OperationCloseWindow,
		OperationMoveResizeWindow, OperationSetWindowState,
	} {
		t.Run(operation, func(t *testing.T) { assertCooperativeCameraError(t, d.Execute(ctx, operation, nil)) })
	}
	_, err := d.ActivateAndResolveTarget(ctx)
	assertCooperativeCameraError(t, err)
	_, err = d.OpenPlayback(ctx)
	assertCooperativeCameraError(t, err)
	held := &windowsHeldInput{backend: &claimPhysicalFake{}}
	assertCooperativeCameraError(t, held.Execute(ctx, OperationHoldButton, HoldButtonRequest{Button: "right"}))
	if held.lease != nil {
		t.Fatal("rejected pointer hold acquired input")
	}
	playback := &windowsPlayback{}
	for _, kind := range []string{
		PlaybackKeyDown, PlaybackKeyUp, PlaybackClick, PlaybackButtonDown, PlaybackButtonUp,
		PlaybackMove, PlaybackDrag, PlaybackMoveRelative, PlaybackScroll,
	} {
		t.Run(kind, func(t *testing.T) {
			assertCooperativeCameraError(t, playback.PlayEvent(ctx, PlaybackEvent{Kind: kind}))
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := d.Execute(canceled, OperationMoveRelative, RelativeMoveRequest{DeltaX: 20}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cooperative conflict hid branch cancellation: %v", err)
	}
	if err := playback.PlayEvent(canceled, PlaybackEvent{Kind: PlaybackMoveRelative, DeltaX: 20}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cooperative playback conflict hid branch cancellation: %v", err)
	}
}

func TestWindowsCooperatingOwnerCompositionCallSite(t *testing.T) {
	d, ctx := compositionDriver(t, "postmessage")
	if err := d.keys.physical.Close(); err != nil {
		t.Fatal(err)
	}
	physical := &claimPhysicalFake{global: true}
	d.keys.physical = physical
	pointer := &claimPhysicalFake{}
	wrapped := d.backend.(*claimedInput)
	if err := wrapped.Backend.Close(); err != nil {
		t.Fatal(err)
	}
	wrapped.Backend = pointer
	parent := inputcoord.FromContext(ctx)
	child := inputcoord.NewCooperatingOwner(parent)
	childCtx := inputcoord.WithOwner(ctx, child)
	held, err := d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := held.Execute(ctx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W"}}); err != nil {
		t.Fatal(err)
	}
	assertW := func() {
		t.Helper()
		if !physical.down[87] || len(physical.down) != 1 {
			t.Fatalf("parent W disturbed: %v", physical.down)
		}
	}
	for _, keys := range [][]string{{"W", "Space"}, {"Shift"}, {"F"}} {
		if err := d.Execute(childCtx, OperationPressKeys, PressKeysRequest{Keys: keys}); err != nil {
			t.Fatal(err)
		}
		assertW()
	}
	child.Freeze()
	canceled, cancel := context.WithTimeout(childCtx, 20*time.Millisecond)
	err = d.Execute(canceled, OperationPressKeys, PressKeysRequest{Keys: []string{"F"}})
	cancel()
	child.Thaw()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("frozen child: %v", err)
	}
	assertW()
	childHeld, err := d.OpenHeldInput()
	if err != nil {
		t.Fatal(err)
	}
	defer childHeld.Close()
	if err := childHeld.Execute(childCtx, OperationHoldKeys, HoldKeysRequest{Keys: []string{"W", "Shift"}}); err != nil {
		t.Fatal(err)
	}
	if err := child.Pause(childCtx); err != nil {
		t.Fatal(err)
	}
	assertW()
	if err := child.Resume(childCtx); err != nil {
		t.Fatal(err)
	}
	if !physical.down[16] {
		t.Fatal("child resume failed to restore Shift")
	}
	if err := childHeld.Close(); err != nil {
		t.Fatal(err)
	}
	assertW()
	assertCooperativeCameraError(t, d.Execute(childCtx, OperationMoveRelative, RelativeMoveRequest{DeltaX: 20}))
	// Exercise the real provider lowering of turn-view into relative input.
	p := &provider{profile: d.profile, driver: d, runtimeMouseCounts360: 360}
	session := openInputSession(t, p, OperationTurnView)
	defer p.Close(ctx, session)
	_, err = p.Invoke(childCtx, session, OperationTurnView, []byte(`{"degrees":20,"durationMilliseconds":0}`))
	assertCooperativeCameraError(t, err)
	if pointer.moves != 0 {
		t.Fatal("cooperative branch moved the camera")
	}
	if err := d.Execute(ctx, OperationMoveRelative, RelativeMoveRequest{DeltaX: 20}); err != nil {
		t.Fatal(err)
	}
	if pointer.moves != 1 {
		t.Fatal("parent camera correction was blocked")
	}
	assertW()
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	// Recovery/paused markers use independent owners after navigation stops.
	exclusive := inputcoord.WithOwner(ctx, inputcoord.NewOwner())
	if err := d.Execute(exclusive, OperationMoveRelative, RelativeMoveRequest{DeltaX: 20}); err != nil {
		t.Fatal(err)
	}
	if pointer.moves != 2 || len(physical.down) != 0 || len(d.keys.counts) != 0 {
		t.Fatal("exclusive recovery or cleanup failed")
	}
}
