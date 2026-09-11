package installed

import (
	"context"
	"github.com/yottaapp/yotta/internal/artifact"
	"testing"
)

func TestTurnUsesProfileCalibrationAndPreservesFractionalCounts(t *testing.T) {
	profile, _ := testProfile(t)
	desktop := desktopPayload(t, profile)
	desktop.MouseCounts360 = 800
	profile, err := SealProfile(NewDesktopProfileDraft(desktop))
	if err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{}
	p := &provider{profile: profile, driver: driver, runtimeMouseCounts360: 4000}
	session := openInputSession(t, p, OperationTurnView)
	defer p.Close(context.Background(), session)
	total := int64(0)
	for range 360 {
		payload, _ := artifact.Marshal(TurnViewRequest{Degrees: 1, DurationMilliseconds: 0})
		if _, err := p.Invoke(context.Background(), session, OperationTurnView, payload); err != nil {
			t.Fatal(err)
		}
		if driver.operation != OperationMoveRelative {
			t.Fatalf("operation=%s", driver.operation)
		}
		total += driver.request.(RelativeMoveRequest).DeltaX
	}
	if total != 800 {
		t.Fatalf("one revolution sent %d counts", total)
	}
}

func TestTurnRejectsUncalibratedTargetBeforeInput(t *testing.T) {
	profile, _ := testProfile(t)
	desktop := desktopPayload(t, profile)
	desktop.MouseCounts360 = 0
	profile, err := SealProfile(NewDesktopProfileDraft(desktop))
	if err != nil {
		t.Fatal(err)
	}
	driver := &fakeDriver{}
	p := &provider{profile: profile, driver: driver}
	session := openInputSession(t, p, OperationTurnView)
	defer p.Close(context.Background(), session)
	if _, err := p.Invoke(context.Background(), session, OperationTurnView, []byte(`{"degrees":10,"durationMilliseconds":100}`)); err == nil || driver.operation != "" {
		t.Fatalf("uncalibrated turn: %v %+v", err, driver)
	}
}
