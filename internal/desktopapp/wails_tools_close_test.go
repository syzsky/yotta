package desktopapp

import (
	"context"
	"testing"
)

func TestToolsWindowCloseQueuesWithoutWaitingForUIThread(t *testing.T) {
	// The UI thread may be inside the shutdown hook waiting for tools.Shutdown.
	// Dispatch must return without waiting for that thread to process the close.
	var pending func()
	closed := false
	requestToolsWindowClose(context.Background(), func(f func()) { pending = f }, func() { closed = true })
	if closed || pending == nil {
		t.Fatal("window close ran synchronously instead of being queued")
	}
	pending()
	if !closed {
		t.Fatal("queued close did not reach the window")
	}
}

func TestToolsWindowCloseLeavesAppShutdownWindowsToWails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requestToolsWindowClose(ctx, func(func()) { t.Error("queued UI work during app shutdown") }, func() {
		t.Error("closed window synchronously during app shutdown")
	})
}

func TestToolsWindowCloseWithoutAttachedAppDoesNotDispatch(t *testing.T) {
	requestToolsWindowClose(nil, func(func()) { t.Error("queued UI work without an app") }, func() {
		t.Error("closed a window without an app")
	})
}
