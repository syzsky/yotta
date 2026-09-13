package desktopapp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/yottaapp/yotta/internal/services/tools"
	"github.com/yottaapp/yotta/pkg/version"
)

const mainWindowMinWidth = 1180

func mainWindowOptions(width, height int) application.WebviewWindowOptions {
	if width < mainWindowMinWidth {
		width = mainWindowMinWidth
	}
	return application.WebviewWindowOptions{
		Title:            "Yotta " + version.Version,
		Width:            width,
		Height:           height,
		MinWidth:         mainWindowMinWidth,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(9, 9, 11),
		Frameless:        true,
		URL:              "/#/workflows",
		KeyBindings:      webviewDebugKeyBindings(),
	}
}

// wailsToolsPresenter adapts the GUI runtime to the narrow tools presentation
// port. It exists in the executable layer so backend packages do not import Wails.
type wailsToolsPresenter struct {
	mu   sync.RWMutex
	app  *application.App
	main *application.WebviewWindow
}

func (p *wailsToolsPresenter) Attach(app *application.App) {
	p.mu.Lock()
	p.app = app
	p.mu.Unlock()
}

func (p *wailsToolsPresenter) Detach() {
	p.mu.Lock()
	p.app = nil
	p.main = nil
	p.mu.Unlock()
}

func (p *wailsToolsPresenter) AttachMain(window *application.WebviewWindow) {
	p.mu.Lock()
	p.main = window
	p.mu.Unlock()
}

func (p *wailsToolsPresenter) Ready() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.app != nil
}

func (p *wailsToolsPresenter) OpenWindow(request tools.WindowRequest) (tools.Window, error) {
	p.mu.RLock()
	app := p.app
	p.mu.RUnlock()
	if app == nil {
		return nil, errors.New("wails application is not ready")
	}

	wailsOptions, err := wailsToolsWindowOptions(request)
	if err != nil {
		return nil, err
	}
	w := &wailsToolsWindow{window: app.Window.NewWithOptions(wailsOptions), appContext: app.Context()}
	if request.Kind == tools.WindowPathEditor {
		w.window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if app.Context().Err() == nil && !w.allowClose.Load() {
				e.Cancel()
				app.Event.Emit("path-editor:close-request", nil)
			}
		})
	}
	return w, nil
}

func (p *wailsToolsPresenter) ShowMain() error {
	p.mu.RLock()
	window := p.main
	p.mu.RUnlock()
	if window == nil {
		return errors.New("main window is not ready")
	}
	window.Show()
	window.Focus()
	return nil
}

func wailsToolsWindowOptions(request tools.WindowRequest) (application.WebviewWindowOptions, error) {
	query := url.Values{}
	withQuery := func(route string) string {
		if encoded := query.Encode(); encoded != "" {
			return route + "?" + encoded
		}
		return route
	}
	darkBackground := application.NewRGB(18, 18, 18)

	switch request.Kind {
	case tools.WindowMouseHUD:
		query.Set("targetSlot", request.TargetSlot)
		return application.WebviewWindowOptions{
			Title: "鼠标位置", Width: 340, Height: 300, MinWidth: 300, MinHeight: 240,
			URL: withQuery("/#/tools/mouse-hud"), Frameless: true, AlwaysOnTop: true,
			BackgroundColour: darkBackground,
		}, nil
	case tools.WindowRecordingHUD:
		return application.WebviewWindowOptions{
			Title: "录制控制", Width: 380, Height: 240, URL: "/#/tools/recording-hud",
			Frameless: true, AlwaysOnTop: true, DisableResize: true,
			BackgroundColour: darkBackground,
		}, nil
	case tools.WindowLauncher:
		return application.WebviewWindowOptions{
			Title: "启动器", Width: 300, Height: 360, MinWidth: 220, MinHeight: 120,
			URL: "/#/tools/launcher", Frameless: true, AlwaysOnTop: true,
			BackgroundColour: darkBackground,
		}, nil
	case tools.WindowPanels:
		return application.WebviewWindowOptions{
			Title: "扩展面板", Width: 480, Height: 520, MinWidth: 240, MinHeight: 160,
			URL: "/#/tools/panels", Frameless: true, AlwaysOnTop: true,
			BackgroundColour: application.NewRGBA(0, 0, 0, 0), BackgroundType: application.BackgroundTypeTransparent,
		}, nil
	case tools.WindowCalibratorHUD:
		query.Set("id", request.RequestID)
		return application.WebviewWindowOptions{
			Title: "鼠标校准", Width: 380, Height: 260, URL: withQuery("/#/tools/calibration-hud"),
			Frameless: true, AlwaysOnTop: true, DisableResize: true,
			BackgroundColour: darkBackground,
		}, nil
	case tools.WindowPathEditor:
		query.Set("id", request.GUID)
		return application.WebviewWindowOptions{
			Title: "路径录制", Width: 1120, Height: 760, MinWidth: 760, MinHeight: 520,
			URL: withQuery("/#/tools/path-editor"), Frameless: true, AlwaysOnTop: true,
			BackgroundColour: darkBackground,
		}, nil
	case tools.WindowScreenPicker:
		query.Set("mode", request.Mode)
		query.Set("id", request.RequestID)
		query.Set("targetSlot", request.TargetSlot)
		query.Set("colorSpace", request.ColorSpace)
		query.Set("guid", request.GUID)
		return application.WebviewWindowOptions{
			Title: "选择屏幕位置", Width: 1360, Height: 860, MinWidth: 760, MinHeight: 520,
			URL: withQuery("/#/tools/screen-picker"), Frameless: true,
		}, nil
	default:
		return application.WebviewWindowOptions{}, fmt.Errorf("unsupported tools window kind %q", request.Kind)
	}
}

func (p *wailsToolsPresenter) Emit(name string, data any) {
	p.mu.RLock()
	app := p.app
	p.mu.RUnlock()
	if app != nil {
		app.Event.Emit(name, data)
	}
}

type wailsToolsWindow struct {
	window     *application.WebviewWindow
	appContext context.Context
	allowClose atomic.Bool
}

func (w *wailsToolsWindow) Focus() { w.window.Focus() }
func (w *wailsToolsWindow) Show()  { w.window.Show() }
func (w *wailsToolsWindow) Hide()  { w.window.Hide() }
func (w *wailsToolsWindow) Close() {
	w.allowClose.Store(true)
	requestToolsWindowClose(w.appContext, application.InvokeAsync, w.window.Close)
}

func requestToolsWindowClose(ctx context.Context, dispatch func(func()), closeWindow func()) {
	// Wails runs shutdown hooks on its UI thread, then closes all windows itself.
	// A tools worker must never synchronously wait for that same UI thread.
	if ctx == nil || ctx.Err() != nil {
		return
	}
	dispatch(closeWindow)
}
func (w *wailsToolsWindow) SetAlwaysOnTop(on bool)       { w.window.SetAlwaysOnTop(on) }
func (w *wailsToolsWindow) SetSize(width, height int)    { w.window.SetSize(width, height) }
func (w *wailsToolsWindow) SetIgnoreMouseEvents(on bool) { w.window.SetIgnoreMouseEvents(on) }
func (w *wailsToolsWindow) OnClosing(callback func()) {
	w.window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		callback()
	})
}
