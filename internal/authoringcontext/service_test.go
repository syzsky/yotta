package authoringcontext

import (
	"bytes"
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/automation/target"
	"image"
	"image/png"
	"testing"
)

type testTargets struct {
	slot string
	fail bool
}

func (targets *testTargets) ResolveTarget(context.Context, string) (target.Target, error) {
	return target.Target{DisplayName: "Example", Resolution: target.Size{W: 2400, H: 1200}}, nil
}
func (targets *testTargets) CapturePNG(_ context.Context, slot string) ([]byte, error) {
	targets.slot = slot
	if targets.fail {
		return nil, errors.New("private path and credential must not escape")
	}
	var raw bytes.Buffer
	_ = png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2400, 1200)))
	return raw.Bytes(), nil
}
func TestCapturePreservesCoordinateMappingAndReturnsImage(t *testing.T) {
	targets := &testTargets{}
	service := &Service{Targets: targets}
	capture, err := service.Capture(context.Background(), CaptureRequest{Slot: "game"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(capture.Data))
	if err != nil || targets.slot != "game" || capture.Info.SourceWidth != 2400 || capture.Info.Width != 1920 || decoded.Bounds().Dy() != 960 || capture.Info.CoordinateSpace != "target" {
		t.Fatalf("capture=%+v err=%v", capture.Info, err)
	}
	targets.fail = true
	_, err = service.Capture(context.Background(), CaptureRequest{Slot: "game"})
	if apperr.From(err).ID != "authoring.observation.capture_failed" {
		t.Fatal(err)
	}
}
func TestScreenOriginAndCancellation(t *testing.T) {
	service := &Service{Screen: func(context.Context) (image.Image, image.Point, error) {
		return image.NewRGBA(image.Rect(0, 0, 640, 480)), image.Pt(-640, -100), nil
	}}
	capture, err := service.Capture(context.Background(), CaptureRequest{Screen: true})
	if err != nil || capture.Info.OriginX != -640 || capture.Info.OriginY != -100 || capture.Info.CoordinateSpace != "screen" {
		t.Fatalf("%+v %v", capture.Info, err)
	}
	_, err = service.Capture(context.Background(), CaptureRequest{Screen: true, Slot: "game"})
	if apperr.From(err).ID != "authoring.observation.invalid_capture" {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = service.Capture(ctx, CaptureRequest{Screen: true}); err == nil {
		t.Fatal("cancelled capture succeeded")
	}
}
