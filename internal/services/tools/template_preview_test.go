package tools

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/blob"
)

func TestTemplatePreviewReturnsBelowThresholdScoreAndKeepsCausePrivate(t *testing.T) {
	digest, _ := artifact.ParseDigest("sha256:" + strings.Repeat("a", 64))
	request := TemplateMatchRequest{TargetSlot: "game", Template: blob.BlobRef{MediaType: "image/png", Digest: digest, Size: 8}, Region: TemplateMatchRegion{Width: 1, Height: 1, Unit: "ratio"}, Threshold: 0.85}
	calls := 0
	cause := errors.New("private native capture path")
	s := NewServiceWithOptions(nil, nil, Options{TemplateMatcher: func(ctx context.Context, got TemplateMatchRequest) (TemplateMatchResult, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("preview has no deadline")
		}
		if calls == 2 {
			return TemplateMatchResult{}, cause
		}
		return TemplateMatchResult{Score: 0.82, Matched: false, FrameWidth: 1920, FrameHeight: 1080}, nil
	}})
	result, err := s.PreviewTemplate(request)
	if err != nil || result.Score != 0.82 || result.Matched {
		t.Fatalf("preview=%+v err=%v", result, err)
	}
	_, err = s.PreviewTemplate(request)
	envelope := apperr.From(err)
	if envelope.ID != "tools.template_preview.failed" || envelope.Category != apperr.CategoryAdapter || !envelope.Retryable || envelope.OperationID == "" || !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("error=%v envelope=%+v", err, envelope)
	}
	if strings.Contains(string(apperr.Marshal(err)), cause.Error()) {
		t.Fatal("raw capture cause leaked")
	}
	request.Threshold = math.NaN()
	if _, err := s.PreviewTemplate(request); err == nil || calls != 2 {
		t.Fatal("invalid threshold reached matcher")
	}
}
