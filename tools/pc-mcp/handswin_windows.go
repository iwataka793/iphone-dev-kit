//go:build windows

package main

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	hwUser32          = syscall.NewLazyDLL("user32.dll")
	pWindowFromPoint  = hwUser32.NewProc("WindowFromPoint")
	pGetAncestor      = hwUser32.NewProc("GetAncestor")
	pGetWindowTextW   = hwUser32.NewProc("GetWindowTextW")
	pSetThreadDpiAwar = hwUser32.NewProc("SetThreadDpiAwarenessContext")
	pFindWindowW      = hwUser32.NewProc("FindWindowW")
	pIsWindowVisible  = hwUser32.NewProc("IsWindowVisible")
	pShowWindow       = hwUser32.NewProc("ShowWindow")
)

// liveTitle：LIVE の窓の題名（tools/hands-window/main.go の title と同じ）
const liveTitle = "claude-hands LIVE"

// hideLive：Claude が画面を撮る・位置で押す間だけ、LIVE の窓を隠す（ASK-DESIGN.md の 3「一瞬隠す」）。
// 返す関数で元に戻す（前に出さない・フォーカスを取らない）。LIVE が無い・隠れているときは何もしない
func hideLive(wait time.Duration) func() {
	t, _ := syscall.UTF16PtrFromString(liveTitle)
	h, _, _ := pFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
	if h == 0 {
		return func() {}
	}
	if v, _, _ := pIsWindowVisible.Call(h); v == 0 {
		return func() {}
	}
	pShowWindow.Call(h, 0)                   // SW_HIDE
	time.Sleep(wait)                         // 画面の描き直しを待つ
	return func() { pShowWindow.Call(h, 4) } // SW_SHOWNOACTIVATE
}

// windowTitleAt：画面の (x, y)（画素）にある窓の、いちばん上の親の題名
func windowTitleAt(x, y int) string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if pSetThreadDpiAwar.Find() == nil {
		old, _, _ := pSetThreadDpiAwar.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2（-4）：画素のまま
		defer pSetThreadDpiAwar.Call(old)
	}
	pt := uintptr(uint32(int32(x))) | uintptr(uint32(int32(y)))<<32
	h, _, _ := pWindowFromPoint.Call(pt)
	if h == 0 {
		return ""
	}
	if r, _, _ := pGetAncestor.Call(h, 2); r != 0 { // GA_ROOT
		h = r
	}
	buf := make([]uint16, 256)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}
