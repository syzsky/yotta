package installed

// TurnViewRequest rotates the camera using the configured desktop calibration.
// Positive angles send positive horizontal counts; game inversion is supplied by callers.
type TurnViewRequest struct {
	Degrees              float64 `json:"degrees"`
	DurationMilliseconds int64   `json:"durationMilliseconds"`
}
