package desktopapp

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/yottaapp/yotta/internal/blob"
	"github.com/yottaapp/yotta/internal/services/tools"
)

type previewCapture struct {
	data  []byte
	calls int
}

func (capture *previewCapture) CapturePNG(context.Context, string) ([]byte, error) {
	capture.calls++
	return capture.data, nil
}

func TestTemplatePreviewCapturesFreshFramesAndUsesRealMatcher(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			frame.SetRGBA(x, y, color.RGBA{R: uint8((x*13 + y*31) % 255), G: uint8((x*43 + y*7) % 255), B: uint8((x*17 + y*53) % 255), A: 255})
		}
	}
	encode := func(img image.Image) []byte {
		var out bytes.Buffer
		if err := png.Encode(&out, img); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	capture := &previewCapture{data: encode(frame)}
	store, err := blob.Open(t.TempDir(), blob.Limits{MaxBlobBytes: 1 << 20, MaxTotalBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := store.Put(context.Background(), "image/png", bytes.NewReader(encode(frame.SubImage(image.Rect(20, 12, 30, 20)))))
	if err != nil {
		t.Fatal(err)
	}
	preview, err := newTemplatePreview(store, capture)
	if err != nil {
		t.Fatal(err)
	}
	request := tools.TemplateMatchRequest{TargetSlot: "fixture", Template: ref, Region: tools.TemplateMatchRegion{Width: 1, Height: 1, Unit: "ratio"}, Threshold: 0.99}
	for i := 0; i < 2; i++ {
		result, err := preview(context.Background(), request)
		if err != nil || !result.Matched || result.Score < 0.99 || result.FrameWidth != 64 || result.FrameHeight != 48 {
			t.Fatalf("preview=%+v err=%v", result, err)
		}
	}
	if capture.calls != 2 {
		t.Fatal("preview reused a stale capture")
	}
	request.Region = tools.TemplateMatchRegion{Width: 5, Height: 5, Unit: "px"}
	if _, err := preview(context.Background(), request); err == nil {
		t.Fatal("invalid template/search geometry accepted")
	}
}
