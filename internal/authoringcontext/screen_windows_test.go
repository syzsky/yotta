//go:build windows

package authoringcontext

import (
	"context"
	"os"
	"testing"
)

func TestNativeScreenCapture(t *testing.T) {
	if os.Getenv("YOTTA_SCREEN_SMOKE") != "1" {
		t.Skip("explicit desktop capture smoke")
	}
	service := &Service{Screen: CaptureScreen}
	result, err := service.Capture(context.Background(), CaptureRequest{Screen: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Info.Width < 100 || result.Info.Height < 100 || len(result.Data) < 100 {
		t.Fatalf("invalid screenshot: %+v", result.Info)
	}
	t.Logf("captured real desktop: %+v, %d bytes", result.Info, len(result.Data))
}
