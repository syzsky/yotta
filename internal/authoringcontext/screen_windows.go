//go:build windows

package authoringcontext

import (
	"context"
	"github.com/lxn/win"
	"image"
	"unsafe"
)

// CaptureScreen captures the virtual desktop in physical screen pixels.
func CaptureScreen(ctx context.Context) (image.Image, image.Point, error) {
	if err := ctx.Err(); err != nil {
		return nil, image.Point{}, err
	}
	x, y := win.GetSystemMetrics(76), win.GetSystemMetrics(77)
	w, h := win.GetSystemMetrics(78), win.GetSystemMetrics(79)
	if w <= 0 || h <= 0 || int64(w)*int64(h) > 64_000_000 {
		return nil, image.Point{}, problem("screen_unavailable")
	}
	src := win.GetDC(0)
	if src == 0 {
		return nil, image.Point{}, problem("screen_unavailable")
	}
	defer win.ReleaseDC(0, src)
	dst := win.CreateCompatibleDC(src)
	if dst == 0 {
		return nil, image.Point{}, problem("capture_failed")
	}
	defer win.DeleteDC(dst)
	bitmapInfo := win.BITMAPINFO{BmiHeader: win.BITMAPINFOHEADER{BiSize: uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})), BiWidth: w, BiHeight: -h, BiPlanes: 1, BiBitCount: 32, BiCompression: win.BI_RGB}}
	var bits unsafe.Pointer
	bitmap := win.CreateDIBSection(dst, &bitmapInfo.BmiHeader, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bitmap == 0 || bits == nil {
		return nil, image.Point{}, problem("capture_failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))
	old := win.SelectObject(dst, win.HGDIOBJ(bitmap))
	defer win.SelectObject(dst, old)
	if !win.BitBlt(dst, 0, 0, w, h, src, x, y, win.SRCCOPY|0x40000000) {
		return nil, image.Point{}, problem("capture_failed")
	}
	frame := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	raw := unsafe.Slice((*byte)(bits), len(frame.Pix))
	for i := 0; i < len(raw); i += 4 {
		frame.Pix[i] = raw[i+2]
		frame.Pix[i+1] = raw[i+1]
		frame.Pix[i+2] = raw[i]
		frame.Pix[i+3] = 255
	}
	return frame, image.Pt(int(x), int(y)), ctx.Err()
}
