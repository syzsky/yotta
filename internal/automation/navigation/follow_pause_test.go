package navigation

import (
	"context"
	"errors"
	"testing"
	"time"
)

// pauseResumeDriver reproduces adapters that release actual input inside Wait
// or Read, wait for resume/freshness, and then return success to the controller.
type pauseResumeDriver struct {
	*followSimulation
	inRead, paused, releasedBeforeTurn bool
	pauseHeading                       float64
	holdCalls                          int
	reholdErr                          error
	waits                              []time.Duration
}

func (d *pauseResumeDriver) releaseOnce(ctx context.Context) error {
	if !d.held || d.paused {
		return nil
	}
	d.paused = true
	if err := d.StopForward(ctx); err != nil {
		return err
	}
	d.advance(200*time.Millisecond, 0)
	d.pose.Heading = Normalize(d.pose.Heading + d.pauseHeading)
	return nil
}

func (d *pauseResumeDriver) Wait(ctx context.Context, duration time.Duration) error {
	d.waits = append(d.waits, duration)
	if !d.inRead {
		if err := d.releaseOnce(ctx); err != nil {
			return err
		}
	}
	return d.followSimulation.Wait(ctx, duration)
}

func (d *pauseResumeDriver) Read(ctx context.Context, after time.Time) (Pose, error) {
	if d.inRead {
		if err := d.releaseOnce(ctx); err != nil {
			return Pose{}, err
		}
	}
	return d.followSimulation.Read(ctx, after)
}

func (d *pauseResumeDriver) HoldForward(ctx context.Context) error {
	d.holdCalls++
	if d.paused && !d.held && d.reholdErr != nil {
		return d.reholdErr
	}
	return d.followSimulation.HoldForward(ctx)
}

func (d *pauseResumeDriver) Turn(ctx context.Context, angle float64) error {
	if d.paused && d.holds == 1 && !d.held {
		d.releasedBeforeTurn = true
	}
	return d.followSimulation.Turn(ctx, angle)
}

func TestFollowReholdsAfterExternalRelease(t *testing.T) {
	for _, inRead := range []bool{false, true} {
		for _, heading := range []float64{0, 12} {
			d := &pauseResumeDriver{followSimulation: newFollowSimulation(), inRead: inRead, pauseHeading: heading}
			r, err := Follow(context.Background(), d, []Waypoint{{0, 0}, {40, 0}}, d.options())
			if err != nil || r.Last != 1 || d.held || !d.paused || d.holds != 2 || d.stops != 2 {
				t.Fatalf("read=%v heading=%v result=%+v err=%v holds/stops=%d/%d", inRead, heading, r, err, d.holds, d.stops)
			}
			if heading != 0 && (d.releasedBeforeTurn || d.movingTurns == 0) {
				t.Fatal("smooth turn did not reacquire forward after resume")
			}
			if d.holdCalls <= d.holds {
				t.Fatal("expected idempotent hold checks between physical holds")
			}
		}
	}
}

func TestFollowReholdFailureStopsBeforeFurtherInput(t *testing.T) {
	want := errors.New("reacquire failed")
	for _, heading := range []float64{0, 12} {
		d := &pauseResumeDriver{followSimulation: newFollowSimulation(), pauseHeading: heading, reholdErr: want}
		_, err := Follow(context.Background(), d, []Waypoint{{0, 0}, {40, 0}}, d.options())
		if !errors.Is(err, want) || d.held || d.turns != 0 || d.holds != 1 {
			t.Fatalf("heading=%v err=%v held=%v turns=%d holds=%d", heading, err, d.held, d.turns, d.holds)
		}
	}
}

func TestFollowCallbackDelayUsesObservationToObservationTime(t *testing.T) {
	d := &pauseResumeDriver{followSimulation: newFollowSimulation(), paused: true}
	o := d.options()
	blocked := false
	var waitsAfterCallback int
	o.OnProgress = func(p Progress) error {
		if !blocked && p.Pose.X >= 5 {
			blocked = true
			// Forward continues during a synchronous periodic action. The
			// next velocity sample must include this delay in its dt.
			d.advance(200*time.Millisecond, 0)
			waitsAfterCallback = len(d.waits)
		}
		return nil
	}
	r, err := Follow(context.Background(), d, []Waypoint{{0, 0}, {60, 0}}, o)
	if err != nil || r.Last != 1 || !blocked || d.holds != 1 || d.stops != 1 {
		t.Fatalf("result=%+v err=%v holds/stops=%d/%d", r, err, d.holds, d.stops)
	}
	// Far from the endpoint, an inflated peak velocity would collapse all
	// following waits. Allow controller adaptation but reject that collapse.
	for _, wait := range d.waits[waitsAfterCallback+1 : waitsAfterCallback+4] {
		if wait < o.Pulse/2 {
			t.Fatalf("callback time was omitted from velocity dt: wait=%v", wait)
		}
	}
}
