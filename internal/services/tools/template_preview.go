package tools

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

type TemplateMatchRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

type TemplateMatchRequest struct {
	TargetSlot string                        `json:"targetSlot"`
	Template   blob.BlobRef                  `json:"template"`
	Variants   []schema.ImageResourceVariant `json:"variants"`
	Region     TemplateMatchRegion           `json:"region"`
	Threshold  float64                       `json:"threshold"`
}

type TemplateMatchResult struct {
	Score       float64 `json:"score"`
	Matched     bool    `json:"matched"`
	FrameWidth  int     `json:"frameWidth"`
	FrameHeight int     `json:"frameHeight"`
}

type TemplateMatcher func(context.Context, TemplateMatchRequest) (TemplateMatchResult, error)

// PreviewTemplate takes one fresh frame. The UI schedules the next request only
// after this one completes, and stops polling when its inspector is disposed.
func (s *Service) PreviewTemplate(request TemplateMatchRequest) (TemplateMatchResult, error) {
	fail := func(cause error) (TemplateMatchResult, error) {
		return TemplateMatchResult{}, toolError("tools.template_preview.failed", apperr.CategoryAdapter, nil, true, cause)
	}
	if s.closed.Load() || s.templateMatcher == nil {
		return fail(errors.New("template preview is unavailable"))
	}
	validBlob := func(ref blob.BlobRef) bool {
		return ref.Validate() == nil && ref.MediaType == "image/png" && ref.Size > 0 && ref.Size <= 32<<20
	}
	if request.TargetSlot == "" || len(request.TargetSlot) > 128 || !validBlob(request.Template) || len(request.Variants) > 64 || math.IsNaN(request.Threshold) || request.Threshold < 0 || request.Threshold > 1 {
		return fail(errors.New("invalid template preview request"))
	}
	for _, variant := range request.Variants {
		if !validBlob(variant.Blob) || variant.Resolution[0] <= 0 || variant.Resolution[1] <= 0 || variant.Resolution[0] > 100_000 || variant.Resolution[1] > 100_000 {
			return fail(errors.New("invalid template variant"))
		}
	}
	r := request.Region
	for _, value := range []float64{r.X, r.Y, r.Width, r.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fail(errors.New("invalid search region"))
		}
	}
	if r.X < 0 || r.Y < 0 || r.Width <= 0 || r.Height <= 0 || (r.Unit != "px" && r.Unit != "ratio") {
		return fail(errors.New("invalid search region"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case s.templatePreviewGate <- struct{}{}:
		defer func() { <-s.templatePreviewGate }()
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	result, err := s.templateMatcher(ctx, request)
	if err != nil {
		return fail(err)
	}
	return result, nil
}
