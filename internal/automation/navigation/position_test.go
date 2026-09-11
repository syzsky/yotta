package navigation

import "testing"

func TestPositionSourceOrderingIgnoresRepublishedReceipts(t *testing.T) {
	for _, test := range []struct {
		name          string
		before, after WorldPosition
		newer         bool
	}{
		{"replayed sequence", WorldPosition{Sequence: 5, SampleAt: 100, ReceivedAt: 100}, WorldPosition{Sequence: 5, SampleAt: 100, ReceivedAt: 200}, false},
		{"older sequence", WorldPosition{Sequence: 5}, WorldPosition{Sequence: 4}, false},
		{"forward sequence", WorldPosition{Sequence: 5, SampleAt: 100}, WorldPosition{Sequence: 6, SampleAt: 100}, true},
		{"regressed source time", WorldPosition{Sequence: 5, SampleAt: 100}, WorldPosition{Sequence: 6, SampleAt: 99}, false},
		{"timestamp replay", WorldPosition{SampleAt: 100}, WorldPosition{SampleAt: 100, ReceivedAt: 200}, false},
		{"timestamp progress", WorldPosition{SampleAt: 100}, WorldPosition{SampleAt: 101}, true},
		{"manual observation", WorldPosition{ReceivedAt: 100}, WorldPosition{ReceivedAt: 101}, true},
		{"epoch change", WorldPosition{Epoch: "a", Sequence: 5}, WorldPosition{Epoch: "b", Sequence: 6}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.after.NewerThan(test.before); got != test.newer {
				t.Fatalf("got %v", got)
			}
		})
	}
}
