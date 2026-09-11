package controller

import (
	"context"
	"errors"
	"math"
	"time"
)

// The controller owns pacing and cancellation. Backends receive instantaneous
// relative deltas, so SendInput cannot collapse a timed turn into one packet.
func playRelativeMotion(ctx context.Context, req RelativeMoveRequest, send func(int, int) error) error {
	if req.DurationMs < 0 || req.DurationMs > 86400000 {
		return errors.New("invalid relative movement duration")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.DurationMs == 0 {
		return send(req.Dx, req.Dy)
	}
	duration := time.Duration(req.DurationMs) * time.Millisecond
	steps := int((duration + 16*time.Millisecond - 1) / (16 * time.Millisecond))
	start := time.Now()
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	timer.Stop()
	x, y := 0, 0
	for step := 1; step <= steps; step++ {
		// Fixed absolute deadlines avoid accumulating scheduling overhead. Never
		// combine overdue samples into a single large mouse event.
		deadline := start.Add(duration/time.Duration(steps)*time.Duration(step) + duration%time.Duration(steps)*time.Duration(step)/time.Duration(steps))
		timer.Reset(max(0, time.Until(deadline)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		targetX := int(math.Round(float64(req.Dx) * float64(step) / float64(steps)))
		targetY := int(math.Round(float64(req.Dy) * float64(step) / float64(steps)))
		if targetX != x || targetY != y {
			if err := send(targetX-x, targetY-y); err != nil {
				return err
			}
		}
		x, y = targetX, targetY
	}
	return nil
}
