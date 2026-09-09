//go:build !windows

package authoringcontext

import (
	"context"
	"image"
)

func CaptureScreen(context.Context) (image.Image, image.Point, error) {
	return nil, image.Point{}, problem("screen_unavailable")
}
