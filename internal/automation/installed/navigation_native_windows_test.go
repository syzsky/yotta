//go:build windows

package installed

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
)

// Opt-in real-target smoke. The caller supplies a concrete desktop profile and
// inspects a capture before requesting a bounded turn or forward pulse.
func TestNavigationDesktopSmoke(t *testing.T) {
	path := os.Getenv("YOTTA_NAVIGATION_SMOKE_PROFILE")
	if path == "" {
		t.Skip("requires an explicitly selected live desktop target")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var desktop DesktopProfilePayload
	if err := json.Unmarshal(raw, &desktop); err != nil {
		t.Fatal(err)
	}
	profile, err := SealProfile(NewDesktopProfileDraft(desktop))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := newPlatformDriver(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if action := os.Getenv("YOTTA_NAVIGATION_SMOKE_ACTION"); action != "" {
		if err := driver.Execute(ctx, OperationActivate, struct{}{}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond)
		p := &provider{profile: profile, driver: driver}
		operation := OperationTurnView
		var request any
		if action == "turn" {
			angle, err := strconv.ParseFloat(os.Getenv("YOTTA_NAVIGATION_SMOKE_ANGLE"), 64)
			if err != nil || angle < -360 || angle > 360 {
				t.Fatal("smoke angle must be -360..360")
			}
			duration := 200
			if value := os.Getenv("YOTTA_NAVIGATION_SMOKE_DURATION"); value != "" {
				duration, err = strconv.Atoi(value)
				if err != nil || duration < 0 || duration > 5000 {
					t.Fatal("smoke duration must be 0..5000 ms")
				}
			}
			request = TurnViewRequest{Degrees: angle, DurationMilliseconds: int64(duration)}
		} else if action == "forward" {
			operation = OperationPressKeys
			request = PressKeysRequest{Keys: []string{"W"}, DurationMilliseconds: 200}
		} else {
			t.Fatal("unknown smoke action")
		}
		session := openInputSession(t, p, operation)
		defer p.Close(context.Background(), session)
		payload, err := artifact.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		if _, err := p.Invoke(ctx, session, operation, payload); err != nil {
			t.Fatal(err)
		}
		t.Logf("input start=%d end=%d elapsed=%s", started.UnixMilli(), time.Now().UnixMilli(), time.Since(started))
	}
	frame, err := driver.Capture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("YOTTA_NAVIGATION_SMOKE_CAPTURE"); out != "" {
		if err := os.WriteFile(out, frame, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Decode the actual adapter result as part of the smoke, including read-only runs.
	if _, err := png.Decode(bytes.NewReader(frame)); err != nil {
		t.Fatal(err)
	}
	if len(frame) == 0 {
		t.Fatal("empty desktop capture")
	}
	t.Logf("captured %d bytes", len(frame))
}
