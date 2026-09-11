package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yottaapp/yotta/internal/automation/target"
)

type relativeInputProbe struct {
	fakeWin32Input
	chunks [][3]int
}

func TestRelativeMotionStopsOnCancellationAndInputFailure(t *testing.T) {
	for _, cancellation := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		want := errors.New("input disconnected")
		calls := 0
		start := time.Now()
		err := playRelativeMotion(ctx, RelativeMoveRequest{Dx: -4100, DurationMs: 1000}, func(dx, dy int) error {
			calls++
			if cancellation {
				cancel()
				return nil
			}
			return want
		})
		cancel()
		if cancellation {
			want = context.Canceled
		}
		if !errors.Is(err, want) || calls != 1 || time.Since(start) > 500*time.Millisecond {
			t.Fatalf("did not stop promptly: cancellation=%v err=%v calls=%d elapsed=%s", cancellation, err, calls, time.Since(start))
		}
	}
}

func (p *relativeInputProbe) MouseMoveRel(_ uintptr, x, y, duration int) error {
	p.chunks = append(p.chunks, [3]int{x, y, duration})
	return nil
}

func TestRelativeTurn360IsPacedAndPreservesCounts(t *testing.T) {
	p := &relativeInputProbe{}
	c, err := NewWin32Controller(target.Target{ID: "win32:42", Kind: target.KindWin32Window, Ref: target.TargetRef{HWND: 42}}, Win32Deps{Input: p})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := c.MoveRelative(context.Background(), RelativeMoveRequest{Dx: 4100, Dy: -73, DurationMs: 1000}); err != nil {
		t.Fatal(err)
	}
	if len(p.chunks) < 30 || time.Since(start) < 950*time.Millisecond {
		t.Fatalf("360-degree turn collapsed: chunks=%d elapsed=%s", len(p.chunks), time.Since(start))
	}
	x, y := 0, 0
	for _, chunk := range p.chunks {
		x += chunk[0]
		y += chunk[1]
		if chunk[2] != 0 || chunk[0] > 100 {
			t.Fatalf("unpaced chunk: %v", chunk)
		}
	}
	if x != 4100 || y != -73 {
		t.Fatalf("counts lost: %d,%d", x, y)
	}
}
