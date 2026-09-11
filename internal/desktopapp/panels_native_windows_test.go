//go:build windows

package desktopapp

import (
	"context"
	"github.com/lxn/win"
	"github.com/yottaapp/yotta/pkg/winutil"
	"os"
	"testing"
	"time"
)

// Read-only probe of an explicitly selected running test host. Input is enabled
// through the real panel UI/RPC before invoking this check.
func TestPanelNativeClickThrough(t *testing.T) {
	executable := os.Getenv("YOTTA_PANEL_SMOKE_EXE")
	if executable == "" {
		t.Skip("requires a selected live test host")
	}
	window, err := winutil.ResolveUniqueExecutableWindow(context.Background(), executable, winutil.MatchSpec{Title: "扩展面板", TitleMatch: "exact"}, time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	want := os.Getenv("YOTTA_PANEL_SMOKE_PASSTHROUGH") == "true"
	style := win.GetWindowLong(win.HWND(window.HWND), win.GWL_EXSTYLE)
	got := style&win.WS_EX_TRANSPARENT != 0 && style&win.WS_EX_LAYERED != 0
	if got != want {
		t.Fatalf("native click-through=%v, want %v, style=%x", got, want, style)
	}
	t.Logf("verified native window %d click-through=%v", window.HWND, got)
}
