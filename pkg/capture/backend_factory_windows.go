//go:build windows

package capture

import (
	"errors"
	"fmt"
	"sync"
)

var (
	wgcInitOnce sync.Once
	wgcInitErr  error
)

func ensureWGCInit() error {
	wgcInitOnce.Do(func() {
		wgcInitErr = initWGC()
	})
	return wgcInitErr
}

// NewIBackend 区分 auto 与显式 WGC：
//   - Win10 (build < 22000): 强制 GDI，不尝试加载 capture_wgc.dll
//   - Win11 (build >= 22000): auto 走 WGC，失败回退 GDI
func NewIBackend(name string) (b IBackend, warning string, err error) {
	isWin10 := WindowsBuild() > 0 && WindowsBuild() < 22000

	switch name {
	case "", "auto":
		if isWin10 {
			// Win10 强制 GDI，避免 DXGI 兼容问题
			if gb, e := newGDIBackend(); e == nil {
				return gb, "", nil
			}
			return nil, "", errors.New("auto: GDI init 失败")
		}
		switch AutoBackend() {
		case BackendWGC:
			if wb, e := newWGCBackend(); e == nil {
				return wb, "", nil
			}
			if gb, e := newGDIBackend(); e == nil {
				return gb, "", nil
			}
			return nil, "", errors.New("auto: WGC + GDI 都初始化失败")
		default:
			if gb, e := newGDIBackend(); e == nil {
				return gb, "", nil
			}
			return nil, "", errors.New("auto: GDI init 失败")
		}
	case "wgc":
		if isWin10 {
			if gb, e := newGDIBackend(); e == nil {
				return gb, fmt.Sprintf("WGC 不支持 Win10 (build %d), 强制回退到 GDI", WindowsBuild()), nil
			}
			return nil, "", errors.New("wgc: Win10 上 GDI fallback 失败")
		}
		if WindowsBuild() >= 18362 {
			if wb, e := newWGCBackend(); e == nil {
				return wb, "", nil
			}
			if gb, e := newGDIBackend(); e == nil {
				return gb, "WGC 初始化失败, fallback 到 GDI", nil
			}
			return nil, "", errors.New("wgc: WGC + GDI 都失败")
		}
		if gb, e := newGDIBackend(); e == nil {
			return gb, fmt.Sprintf("WGC 要求 Windows 10 1903+, 当前 build %d, fallback 到 GDI", WindowsBuild()), nil
		}
		return nil, "", errors.New("wgc: unsupported OS 且 GDI fallback 也失败")
	case "gdi":
		if gb, e := newGDIBackend(); e == nil {
			return gb, "", nil
		}
		return nil, "", errors.New("gdi: init 失败 (无 fallback for explicit gdi)")
	case "mock":
		if mb, e := newMockBackend(); e == nil {
			return mb, "", nil
		} else {
			return nil, "", e
		}
	default:
		return nil, "", fmt.Errorf("unknown capture backend %q (supported: auto/wgc/gdi/mock)", name)
	}
}
