package noderuntime

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/yottaapp/yotta/internal/automation/navigation"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/run"
)

func TestPathClassificationPreservesReleaseFailure(t *testing.T) {
	release := errors.New("release failed")
	for _, cause := range []error{errPathMarkerPause, navigation.ErrStuck, context.Canceled, context.DeadlineExceeded, errPathHeight} {
		if !pathOnlyCause(fmt.Errorf("wrapped: %w", cause), cause) {
			t.Fatal("wrapped outcome not classified")
		}
		if pathOnlyCause(errors.Join(cause, release), cause) {
			t.Fatal("release failure hidden by navigation outcome")
		}
		if got := pathCancellationRemainder(errors.Join(context.Canceled, release)); !errors.Is(got, release) {
			t.Fatal("cancel hid release failure")
		}
	}
}

func TestPathProgressCountersSignedRoundTripAndJournal(t *testing.T) {
	for _, value := range []float64{-123.456, -0.0004, 0, 0.0004, 123.456} {
		at := navigation.Progress{
			Pose:   navigation.Pose{X: value, Y: -value, Heading: -90},
			Target: navigation.Waypoint{X: -value, Y: value}, HeadingError: value,
			Speed: 2.125, Distance: 12.1, Last: -1, Current: 0,
		}
		counters := pathProgressCounters(at, 0, 2, 23)
		for _, field := range []struct {
			name, unit string
			want       float64
		}{
			{"x", "milli", value}, {"y", "milli", -value},
			{"target_x", "milli", -value}, {"target_y", "milli", value},
			{"heading_error", "mdeg", value},
		} {
			magnitude, ok := counters[field.name+"_"+field.unit+"_abs"]
			negative, signOK := counters[field.name+"_negative"]
			if !ok || !signOK || (negative != 0 && negative != 1) || (negative == 1) != (field.want < 0) {
				t.Fatalf("invalid signed field %s: %v", field.name, counters)
			}
			decoded := float64(magnitude) / 1000
			if negative == 1 {
				decoded = -decoded
			}
			if math.Abs(decoded-field.want) > 0.000500001 {
				t.Fatalf("%s round trip: got %v want %v", field.name, decoded, field.want)
			}
		}
		if len(counters) != 17 || len(counters) > 64 {
			t.Fatalf("counter budget: %d", len(counters))
		}
		for name, value := range counters {
			if value < 0 {
				t.Fatalf("negative journal counter %s=%d", name, value)
			}
		}
		if counters["heading_mdeg"] != 270000 || counters["speed_milli"] != 2125 || counters["sample_age_ms"] != 23 || counters["last_point"] != 0 || counters["current_point"] != 1 || counters["remaining_distance"] != 13 || counters["recovery_attempt"] != 2 {
			t.Fatalf("unexpected progress evidence: %v", counters)
		}
		if _, err := run.NewRedactedSummary(nodes.PathProgressStatus, counters, nil); err != nil {
			t.Fatalf("actual journal validator rejected progress: %v", err)
		}
	}
}

func TestPathProgressCountersNormalizeAndBound(t *testing.T) {
	for _, heading := range []struct {
		value float64
		want  int64
	}{{-450, 270000}, {0, 0}, {360, 0}, {725, 5000}, {359.9999, 0}} {
		at := navigation.Progress{
			Pose: navigation.Pose{X: -math.MaxFloat64, Y: math.MaxFloat64, Heading: heading.value},
			Last: -1, Speed: -1,
		}
		counters := pathProgressCounters(at, 3, 0, -5)
		if counters["heading_mdeg"] != heading.want || counters["x_milli_abs"] != 1e15 || counters["x_negative"] != 1 || counters["y_milli_abs"] != 1e15 || counters["y_negative"] != 0 || counters["speed_milli"] != 0 || counters["sample_age_ms"] != 0 || counters["last_point"] != 3 || counters["current_point"] != 4 {
			t.Fatalf("incorrect normalized/bounded evidence: %v", counters)
		}
		if _, err := run.NewRedactedSummary(nodes.PathProgressStatus, counters, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(-1), -1, 0, math.Inf(1)} {
		if got := pathMetric(value); got < 0 || got > 1e15 {
			t.Fatalf("unbounded metric %v: %d", value, got)
		}
	}
}
