package desktopapp

import (
	"bytes"
	"context"
	"errors"
	"image/png"

	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/noderuntime"
	"github.com/yottaapp/yotta/internal/runprepare"
	"github.com/yottaapp/yotta/internal/services/tools"
)

type templateCapture interface {
	CapturePNG(context.Context, string) ([]byte, error)
}

func newTemplatePreview(blobs *blob.Store, targets templateCapture) (tools.TemplateMatcher, error) {
	planner, err := runprepare.New(blobs)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, request tools.TemplateMatchRequest) (tools.TemplateMatchResult, error) {
		frame, err := targets.CapturePNG(ctx, request.TargetSlot)
		if err != nil {
			return tools.TemplateMatchResult{}, err
		}
		geometry, err := png.DecodeConfig(bytes.NewReader(frame))
		if err != nil {
			return tools.TemplateMatchResult{}, err
		}
		if int64(geometry.Width)*int64(geometry.Height) > 16_777_216 {
			return tools.TemplateMatchResult{}, errors.New("capture exceeds image size limit")
		}
		ref := request.Template
		if len(request.Variants) > 0 {
			selected, release, err := planner.PrepareTemplate(ctx, request.Variants, [2]int{geometry.Width, geometry.Height})
			if err != nil {
				return tools.TemplateMatchResult{}, err
			}
			defer release()
			ref = selected
		}
		template, err := blobs.ReadRange(ctx, ref, 0, ref.Size)
		if err != nil {
			return tools.TemplateMatchResult{}, err
		}
		r := request.Region
		score, matched, err := noderuntime.PreviewTemplateMatch(frame, template, [4]float64{r.X, r.Y, r.Width, r.Height}, r.Unit, request.Threshold)
		if err != nil {
			return tools.TemplateMatchResult{}, err
		}
		if err := ctx.Err(); err != nil {
			return tools.TemplateMatchResult{}, err
		}
		return tools.TemplateMatchResult{Score: score, Matched: matched, FrameWidth: geometry.Width, FrameHeight: geometry.Height}, nil
	}, nil
}
