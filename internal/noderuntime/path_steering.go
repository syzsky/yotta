package noderuntime

import (
	"context"
	"errors"
	"time"

	"github.com/yottaapp/yotta/internal/automation/installed"
)

// StopForward requires the next path observation to have been sampled after
// physical release. Repeated cleanup must not keep moving that freshness gate.
func (d *pathDriver) StopForward(ctx context.Context) error {
	streamErr := d.stream.stop()
	held := d.held.Validate() == nil
	if err := d.navigationDriver.StopForward(ctx); err != nil {
		return errors.Join(streamErr, err)
	}
	if held {
		d.freshAfter = d.wallTime().UnixMilli() + 1
	}
	return streamErr
}

// Steer is the path-only timed turn seam. Calibration and fractional mouse
// counts remain owned by the installed target; the caller owns the next wait/read.
func (d *pathDriver) Steer(ctx context.Context, angle float64, duration time.Duration) error {
	if duration <= 0 || duration%time.Millisecond != 0 {
		return errors.New("path steering duration must be a positive whole number of milliseconds")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := d.Wait(ctx, 0); err != nil {
		return err
	}
	if err := navInvoke(ctx, d.i, d.turn, installed.OperationTurnView, installed.TurnViewRequest{
		Degrees: angle, DurationMilliseconds: duration.Milliseconds(),
	}); err != nil {
		return err
	}
	d.freshAfter = d.wallTime().UnixMilli() + 1
	return nil
}
