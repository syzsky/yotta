package navigation

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type simulation struct {
	pose                Pose
	turns, steps, reads int
	stale               bool
	cancel              context.CancelFunc
}

func (s *simulation) Read(_ context.Context, after time.Time) (Pose, error) {
	s.reads++
	if s.stale {
		return s.pose, ErrStale
	}
	s.pose.Time = time.Now().Add(time.Millisecond)
	if !s.pose.Time.After(after) {
		s.pose.Time = after.Add(time.Millisecond)
	}
	return s.pose, nil
}
func (s *simulation) Turn(_ context.Context, angle float64) error {
	s.turns++
	s.pose.Heading = Normalize(s.pose.Heading + angle)
	return nil
}
func (s *simulation) Forward(ctx context.Context, d time.Duration) error {
	s.steps++
	if s.cancel != nil {
		s.cancel()
		return ctx.Err()
	}
	meters := d.Seconds() * 10
	s.pose.X += math.Cos(s.pose.Heading*math.Pi/180) * meters
	s.pose.Y += math.Sin(s.pose.Heading*math.Pi/180) * meters
	return nil
}
func TestMoveCorrectsDirectionAndConfirmsArrival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &simulation{pose: Pose{Heading: 180}}
	p, d, err := MoveTo(ctx, s, Options{X: 10, Y: 0, Tolerance: 0.6, AxisSign: 1, TurnSign: 1, Pulse: 200 * time.Millisecond, StuckTimeout: time.Second})
	if err != nil || d > 0.6 || s.turns < 4 || s.steps == 0 || s.reads < s.turns+s.steps+2 {
		t.Fatalf("pose=%+v distance=%v sim=%+v err=%v", p, d, s, err)
	}
}
func TestUnavailableNeverMoves(t *testing.T) {
	s := &simulation{stale: true}
	_, _, err := MoveTo(context.Background(), s, Options{X: 10, Tolerance: 1, AxisSign: 1, TurnSign: 1, Pulse: 200 * time.Millisecond, StuckTimeout: time.Second})
	if !errors.Is(err, ErrStale) || s.turns != 0 || s.steps != 0 {
		t.Fatalf("err=%v sim=%+v", err, s)
	}
}
func TestCancellationDuringForwardDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &simulation{cancel: cancel}
	_, _, err := MoveTo(ctx, s, Options{X: 10, Tolerance: 1, AxisSign: 1, TurnSign: 1, Pulse: 200 * time.Millisecond, StuckTimeout: time.Second})
	if !errors.Is(err, context.Canceled) || s.steps != 1 {
		t.Fatalf("err=%v sim=%+v", err, s)
	}
}
func TestHeadingWrapUsesShortTurn(t *testing.T) {
	if Delta(1, 359) != 2 || Delta(359, 1) != -2 || Normalize(-1) != 359 {
		t.Fatal("heading wrapping failed")
	}
}

type gradualTurn struct{ simulation }

func (s *gradualTurn) Turn(ctx context.Context, angle float64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(60 * time.Millisecond):
	}
	return s.simulation.Turn(ctx, max(-5, min(5, angle)))
}

func TestTurningProgressDoesNotCountAsStuckMovement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	s := &gradualTurn{simulation{pose: Pose{Heading: 180}}}
	_, distance, err := MoveTo(ctx, s, Options{X: 10, Tolerance: 0.6, AxisSign: 1, TurnSign: 1, Pulse: 200 * time.Millisecond, StuckTimeout: time.Second})
	if err != nil || distance > 0.6 {
		t.Fatalf("turning toward the target must remain progress: distance=%v turns=%d err=%v", distance, s.turns, err)
	}
}

type compassSimulation struct{ simulation }

func (s *compassSimulation) Forward(ctx context.Context, duration time.Duration) error {
	s.pose.Heading -= 90.7879602977622
	err := s.simulation.Forward(ctx, duration)
	s.pose.Heading += 90.7879602977622
	return err
}

func TestMoveInNTECompassFrame(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &compassSimulation{simulation{pose: Pose{X: -260421.33, Y: 135134.2, Heading: 280.2}}}
	_, distance, err := MoveTo(ctx, s, Options{X: -259488, Y: 131920, Tolerance: 20, AxisHeading: 90.7879602977622, AxisSign: 1, TurnSign: 1, Pulse: 200 * time.Millisecond, StuckTimeout: time.Second})
	if err != nil || distance > 20 {
		t.Fatalf("mapped world coordinates did not converge: distance=%v err=%v", distance, err)
	}
}
