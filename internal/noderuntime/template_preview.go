package noderuntime

// PreviewTemplateMatch uses the same matcher as template nodes, without opening
// an execution session or dispatching any input operation.
func PreviewTemplateMatch(frame, template []byte, region [4]float64, unit string, threshold float64) (float64, bool, error) {
	match, err := matchTemplateBytes(frame, template, visionRegion{X: region[0], Y: region[1], Width: region[2], Height: region[3], Unit: unit}, threshold)
	return match.Score, match.Matched, err
}
