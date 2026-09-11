package navigation

import "time"

// WorldPosition is a transport-independent observation. AxisHeading and
// AxisSign define how coordinates map to the reported view heading. Receive
// time belongs to the host; source time/sequence/epoch remain source metadata.
type WorldPosition struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Heading     float64 `json:"heading"`
	Frame       string  `json:"frame"`
	Unit        string  `json:"unit"`
	AxisHeading float64 `json:"axisHeading"`
	AxisSign    int     `json:"axisSign"`
	Valid       bool    `json:"valid"`
	ReceivedAt  int64   `json:"receivedAt"`
	SampleAt    int64   `json:"sampleAt"`
	Sequence    int64   `json:"sequence"`
	Epoch       string  `json:"epoch"`
}

// NewerThan compares source observations, not writes to a workflow state slot.
// A provider cannot turn a replay into fresh evidence by publishing it again.
func (p WorldPosition) NewerThan(previous WorldPosition) bool {
	if p.Epoch != previous.Epoch {
		return false
	}
	if p.Sequence > 0 || previous.Sequence > 0 {
		return p.Sequence > previous.Sequence && (previous.SampleAt <= 0 || p.SampleAt >= previous.SampleAt)
	}
	if p.SampleAt > 0 || previous.SampleAt > 0 {
		return p.SampleAt > previous.SampleAt
	}
	return p.ReceivedAt > previous.ReceivedAt
}

func (p WorldPosition) Fresh(now time.Time, age time.Duration) bool {
	elapsed := now.Sub(time.UnixMilli(p.ReceivedAt))
	return p.Valid && Finite(p.X) && Finite(p.Y) && Finite(p.Heading) && Finite(p.AxisHeading) &&
		(p.AxisSign == 1 || p.AxisSign == -1) && p.Frame != "" && p.Unit != "" && p.ReceivedAt > 0 && elapsed >= -100*time.Millisecond && elapsed <= age
}
