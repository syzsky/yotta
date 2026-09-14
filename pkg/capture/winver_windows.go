// OS build 号检测，给"auto 选 backend"用：
//   - Win11 (build >= 22000) 走 WGC（黄框可关 + 部分场景帧更稳）
//   - Win10 及以下走 GDI（兼容性最好 + 避开 DLL 依赖）
//
// Win10 强制 GDI 原因：capture_wgc.dll 用了 DXGI_INTERFACE_API，Win10 上
// 接口版本不一致会导致黑帧或 crash。纯 GDI 不依赖任何外部 DLL。
//
// 用 ntdll.RtlGetVersion 而不是 GetVersionEx：后者从 Win8.1 起被 Microsoft 限制，
// 不带兼容性 manifest 时会一直返回 6.2（Win8）。RtlGetVersion 是 NT 内部 API，
// 不受兼容性 shim 影响，永远拿真实版本。
//
// WindowsBuild 是 var 而不是 func，测试里可以临时覆盖它来模拟不同 OS 版本。

package capture

import (
	"syscall"
	"unsafe"
)

// osVersionInfoEx 跟 Windows OSVERSIONINFOEXW 对齐（284 字节）。
type osVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserved          byte
}

var procRtlGetVersion = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")

// WindowsBuild 返回当前 Windows build 号（如 19045 = Win10 22H2，22000 = Win11 21H2）。
// 声明为 var 而不是 func，这样测试可以临时覆盖它来模拟不同 OS 版本。
// 生产代码里它永远调用 RtlGetVersion，不受兼容性 manifest 影响。
var WindowsBuild = func() uint32 {
	var info osVersionInfoEx
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return 0
	}
	return info.BuildNumber
}

// AutoBackend 根据 OS 选最优后端：
//   - Win11 (build >= 22000): WGC
//   - Win10 及以下: GDI（强制，不依赖 capture_wgc.dll）
func AutoBackend() Backend {
	if WindowsBuild() >= 22000 {
		return BackendWGC
	}
	return BackendGDI
}

// ForceGDI 让调用方判断当前是否强制 GDI 模式（日志/诊断用）
func ForceGDI() bool {
	return WindowsBuild() < 22000
}
