// window_layout.go — 定位成功后把 TOTKnavi 窗口挪到副屏并最大化，游戏退出时还原。
//
// 主/副屏按 Windows 的显示器定义，与游戏窗口所在屏无关：
//  1. 主屏 = 显示设置里勾了「设为主显示器」的那块（MONITORINFOF_PRIMARY）。
//  2. 副屏 = 其余显示器。设置里指定了目标设备名就用它；否则相对主屏优先右侧相邻
//     （垂直重叠 ≥30%），其次左侧、下方、上方；都不共边时取中心距离最近的，
//     同分按左上角 X/Y 稳定排序。
//  3. 只有一块显示器 → 不动。
//  4. 窗口已最小化 → 不动（避免把缩在任务栏的窗口弹出来抢游戏焦点）。
//
// 移动用 SetWindowPlacement(SW_SHOWMAXIMIZED)，不抢游戏焦点。
// 还原只做一件事：把窗口放回移屏前的位置与状态（含最大化）。快照只在「真的还原成功」
// 或「用户自己把窗口挪走了」时才丢弃——跳过还原绝不能丢快照，否则下一局会把副屏
// 当成原始位置，从此再也回不到主屏。

package main

import (
	"os"
	"sync"
	"syscall"
	"unsafe"
)

var (
	wlUser32 = syscall.NewLazyDLL("user32.dll")

	wlEnumWindows         = wlUser32.NewProc("EnumWindows")
	wlGetWindowThreadPID  = wlUser32.NewProc("GetWindowThreadProcessId")
	wlIsWindowVisible     = wlUser32.NewProc("IsWindowVisible")
	wlIsIconic            = wlUser32.NewProc("IsIconic")
	wlIsZoomed            = wlUser32.NewProc("IsZoomed")
	wlGetWindowRect       = wlUser32.NewProc("GetWindowRect")
	wlSetWindowPos        = wlUser32.NewProc("SetWindowPos")
	wlEnumDisplayMonitors = wlUser32.NewProc("EnumDisplayMonitors")
	wlGetMonitorInfoW     = wlUser32.NewProc("GetMonitorInfoW")
	wlMonitorFromWindow   = wlUser32.NewProc("MonitorFromWindow")
	wlGetWindowPlacement  = wlUser32.NewProc("GetWindowPlacement")
	wlSetWindowPlacement  = wlUser32.NewProc("SetWindowPlacement")
)

const (
	monitorDefaultToNearest = 2
	monitorInfoPrimary      = 1 // MONITORINFOF_PRIMARY

	swShowMinimized = 2
	swShowMaximized = 3

	// WPF_ASYNCWINDOWPLACEMENT：调用线程不是窗口所属线程时，把请求异步投递过去。
	// Go 侧调用都在任意 goroutine 上，带上它可避免偶发阻塞/失败。
	wpfAsyncWindowPlacement = 0x0004

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

type wlRect struct {
	Left, Top, Right, Bottom int32
}

func (r wlRect) width() int32  { return r.Right - r.Left }
func (r wlRect) height() int32 { return r.Bottom - r.Top }

// contains 判断点是否落在矩形内（左闭右开，与显示器矩形语义一致）。
func (r wlRect) contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

// intersects 判断两个矩形是否有面积重叠。
func (r wlRect) intersects(o wlRect) bool {
	return r.Left < o.Right && r.Right > o.Left && r.Top < o.Bottom && r.Bottom > o.Top
}

type wlPoint struct {
	X, Y int32
}

type wlWindowPlacement struct {
	Length         uint32
	Flags          uint32
	ShowCmd        uint32
	MinPosition    wlPoint
	MaxPosition    wlPoint
	NormalPosition wlRect
}

type wlMonitorInfo struct {
	Size       uint32
	Monitor    wlRect
	Work       wlRect
	Flags      uint32
	DeviceName [32]uint16
}

type wlMonitor struct {
	Handle  uintptr
	Info    wlMonitorInfo
	Name    string
	Primary bool
}

type wlWindowFind struct {
	pid         uint32
	allowIconic bool
	best        uintptr
	bestArea    int64
}

// 枚举上下文用包级变量传递（EnumWindows/EnumDisplayMonitors 都是同步回调），
// 再用一把锁把"装填上下文 → 枚举 → 取结果"整段串起来：ListMonitors 等入口
// 可能被并发调用，共享变量不能裸用。
var (
	wlEnumMu     sync.Mutex
	wlWindowCtx  wlWindowFind
	wlMonitorCtx wlMonitorEnumCtx
)

// wlEnumWindowProc：只看指定 PID 的可见顶层窗口，取面积最大的作为主窗口。
func wlEnumWindowProc(hwnd uintptr, _ uintptr) uintptr {
	ctx := &wlWindowCtx
	var pid uint32
	wlGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != ctx.pid {
		return 1
	}
	if r, _, _ := wlIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	iconic := false
	if r, _, _ := wlIsIconic.Call(hwnd); r != 0 {
		if !ctx.allowIconic {
			return 1
		}
		iconic = true
	}
	var rect wlRect
	if iconic {
		// 最小化窗口的 GetWindowRect 是 (-32000,-32000)，面积要用还原后的位置算。
		var wp wlWindowPlacement
		wp.Length = uint32(unsafe.Sizeof(wp))
		if r, _, _ := wlGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); r == 0 {
			return 1
		}
		rect = wp.NormalPosition
	} else if r, _, _ := wlGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r == 0 {
		return 1
	}
	if area := int64(rect.width()) * int64(rect.height()); area > ctx.bestArea {
		ctx.best = hwnd
		ctx.bestArea = area
	}
	return 1
}

var wlEnumWindowCallback = syscall.NewCallback(wlEnumWindowProc)

// wlFindMainWindow 返回进程面积最大的可见顶层窗口；allowIconic 为 false 时跳过最小化窗口。
func wlFindMainWindow(pid uint32, allowIconic bool) uintptr {
	if pid == 0 {
		return 0
	}
	wlEnumMu.Lock()
	defer wlEnumMu.Unlock()
	wlWindowCtx = wlWindowFind{pid: pid, allowIconic: allowIconic}
	wlEnumWindows.Call(wlEnumWindowCallback, 0)
	return wlWindowCtx.best
}

type wlMonitorEnumCtx struct {
	monitors []wlMonitor
}

func wlEnumMonitorProc(hMonitor uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
	mi := wlMonitorInfo{Size: uint32(unsafe.Sizeof(wlMonitorInfo{}))}
	if r, _, _ := wlGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); r != 0 {
		wlMonitorCtx.monitors = append(wlMonitorCtx.monitors, wlMonitor{
			Handle:  hMonitor,
			Info:    mi,
			Name:    syscall.UTF16ToString(mi.DeviceName[:]),
			Primary: mi.Flags&monitorInfoPrimary != 0,
		})
	}
	return 1
}

var wlEnumMonitorCallback = syscall.NewCallback(wlEnumMonitorProc)

// wlMonitors 枚举当前所有显示器（顺序为系统枚举顺序，不保证稳定）。
func wlMonitors() []wlMonitor {
	wlEnumMu.Lock()
	defer wlEnumMu.Unlock()
	wlMonitorCtx = wlMonitorEnumCtx{}
	wlEnumDisplayMonitors.Call(0, 0, wlEnumMonitorCallback, 0)
	return wlMonitorCtx.monitors
}

// wlPrimaryMonitor 返回 Windows 定义的主显示器。
func wlPrimaryMonitor(monitors []wlMonitor) (wlMonitor, bool) {
	for _, m := range monitors {
		if m.Primary {
			return m, true
		}
	}
	return wlMonitor{}, false
}

// ListMonitors 供设置下拉框列出所有显示器（id 用设备名，跨启动稳定）。
func (a *App) ListMonitors() []map[string]any {
	ms := wlMonitors()
	out := make([]map[string]any, 0, len(ms))
	for i, m := range ms {
		short := m.Name
		if len(short) > 4 && short[:4] == `\\.\` {
			short = short[4:]
		}
		out = append(out, map[string]any{
			"id":      m.Name,
			"index":   i,
			"name":    short,
			"x":       m.Info.Monitor.Left,
			"y":       m.Info.Monitor.Top,
			"width":   m.Info.Monitor.width(),
			"height":  m.Info.Monitor.height(),
			"primary": m.Primary,
		})
	}
	return out
}

// wlOverlapRatio 两段区间重叠长度 / 较短区间长度。
func wlOverlapRatio(a1, a2, b1, b2 int32) float64 {
	lo, hi := a1, a2
	if b1 > lo {
		lo = b1
	}
	if b2 < hi {
		hi = b2
	}
	if hi <= lo {
		return 0
	}
	shortest := a2 - a1
	if b2-b1 < shortest {
		shortest = b2 - b1
	}
	if shortest <= 0 {
		return 0
	}
	return float64(hi-lo) / float64(shortest)
}

func wlCenter(r wlRect) (float64, float64) {
	return float64(r.Left) + float64(r.width())/2, float64(r.Top) + float64(r.height())/2
}

func wlDistance(a, b wlRect) float64 {
	ax, ay := wlCenter(a)
	bx, by := wlCenter(b)
	dx, dy := ax-bx, ay-by
	return dx*dx + dy*dy
}

// wlNeighbourPriority 相对基准屏的邻接优先级：0 右、1 左、2 下、3 上、99 不共边。
// 返回的 dist 在同优先级里用于挑最近的（共边时为间隙，不共边时为中心距离）。
func wlNeighbourPriority(base, m wlRect) (int, float64) {
	ovY := wlOverlapRatio(base.Top, base.Bottom, m.Top, m.Bottom)
	ovX := wlOverlapRatio(base.Left, base.Right, m.Left, m.Right)
	switch {
	case ovY >= 0.30 && m.Left >= base.Right:
		return 0, float64(m.Left - base.Right)
	case ovY >= 0.30 && m.Right <= base.Left:
		return 1, float64(base.Left - m.Right)
	case ovX >= 0.30 && m.Top >= base.Bottom:
		return 2, float64(m.Top - base.Bottom)
	case ovX >= 0.30 && m.Bottom <= base.Top:
		return 3, float64(base.Top - m.Bottom)
	}
	return 99, wlDistance(base, m)
}

// wlPickTarget 从显示器里选"副屏"。preferred 为用户指定的显示器设备名（空/auto 表示自动）。
// 主屏固定以 Windows 的「主显示器」为准，不再看游戏窗口在哪块屏。
func wlPickTarget(monitors []wlMonitor, preferred string) (wlMonitor, bool) {
	if len(monitors) < 2 {
		return wlMonitor{}, false
	}
	if preferred != "" && preferred != "auto" {
		for _, m := range monitors {
			if m.Name == preferred {
				return m, true
			}
		}
	}
	primary, ok := wlPrimaryMonitor(monitors)
	if !ok {
		return wlMonitor{}, false
	}
	others := make([]wlMonitor, 0, len(monitors)-1)
	for _, m := range monitors {
		if m.Name != primary.Name {
			others = append(others, m)
		}
	}
	if len(others) == 0 {
		return wlMonitor{}, false
	}
	if len(others) == 1 {
		return others[0], true
	}

	best := others[0]
	bestPrio, bestDist := wlNeighbourPriority(primary.Info.Monitor, best.Info.Monitor)
	for _, m := range others[1:] {
		prio, dist := wlNeighbourPriority(primary.Info.Monitor, m.Info.Monitor)
		if prio < bestPrio ||
			(prio == bestPrio && dist < bestDist) ||
			(prio == bestPrio && dist == bestDist && (m.Info.Monitor.Left < best.Info.Monitor.Left ||
				(m.Info.Monitor.Left == best.Info.Monitor.Left && m.Info.Monitor.Top < best.Info.Monitor.Top))) {
			best, bestPrio, bestDist = m, prio, dist
		}
	}
	return best, true
}

// wlMoveMaximize 在目标显示器的 work area 上最大化窗口，不抢焦点。
func wlMoveMaximize(hwnd uintptr, work wlRect) bool {
	var wp wlWindowPlacement
	wp.Length = uint32(unsafe.Sizeof(wp))
	if r, _, _ := wlGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); r == 0 {
		return false
	}
	wp.Flags = wpfAsyncWindowPlacement
	wp.ShowCmd = swShowMaximized
	wp.NormalPosition = work
	r, _, _ := wlSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	return r != 0
}

// wlApplyPlacement 把快照原样写回窗口（保留原来的 ShowCmd，最大化/普通都还原）。
func wlApplyPlacement(hwnd uintptr, wp wlWindowPlacement) bool {
	wp.Flags = wpfAsyncWindowPlacement
	wp.Length = uint32(unsafe.Sizeof(wp))
	r, _, _ := wlSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
	return r != 0
}

// wlMonitorOf 返回窗口当前所在的显示器。
func wlMonitorOf(monitors []wlMonitor, hwnd uintptr) (wlMonitor, bool) {
	if iconic, _, _ := wlIsIconic.Call(hwnd); iconic == 0 {
		var rect wlRect
		if r, _, _ := wlGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r != 0 {
			cx := rect.Left + rect.width()/2
			cy := rect.Top + rect.height()/2
			for _, m := range monitors {
				if m.Info.Monitor.contains(cx, cy) {
					return m, true
				}
			}
		}
	}
	// 最小化窗口或矩形异常：退回 MonitorFromWindow 再按句柄匹配设备名。
	h, _, _ := wlMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
	for _, m := range monitors {
		if m.Handle == h {
			return m, true
		}
	}
	return wlMonitor{}, false
}

// wlPlacementOnAnyMonitor 判断快照里的普通位置是否落在某块仍然存在的显示器上。
func wlPlacementOnAnyMonitor(wp wlWindowPlacement, monitors []wlMonitor) bool {
	cx := wp.NormalPosition.Left + wp.NormalPosition.width()/2
	cy := wp.NormalPosition.Top + wp.NormalPosition.height()/2
	for _, m := range monitors {
		if m.Info.Monitor.contains(cx, cy) {
			return true
		}
	}
	return false
}

// ArrangeWindowForGame 定位成功后调用：把 TOTKnavi 移到副屏（Windows 主显示器之外的屏）
// 并最大化。只做窗口布局，不改变定位状态；前端按设置决定是否调用。
// 返回的 reason 供前端判断是否值得重试。
func (a *App) ArrangeWindowForGame() map[string]any {
	a.mu.Lock()
	enabled := a.cfg == nil || a.cfg.AutoMoveToSecondary == nil || *a.cfg.AutoMoveToSecondary
	preferred := ""
	if a.cfg != nil {
		preferred = a.cfg.AutoMoveTarget
	}
	a.mu.Unlock()

	if !enabled {
		a.guiLog("窗口联动：未启用自动移屏，跳过")
		return map[string]any{"ok": false, "reason": "disabled"}
	}

	monitors := wlMonitors()
	if len(monitors) < 2 {
		a.guiLog("窗口联动：只检测到 %d 块显示器，无需移屏", len(monitors))
		return map[string]any{"ok": false, "reason": "single-monitor", "monitors": len(monitors)}
	}
	target, ok := wlPickTarget(monitors, preferred)
	if !ok {
		a.guiLog("窗口联动：找不到可用的副屏（已指定 %q），跳过", preferred)
		return map[string]any{"ok": false, "reason": "target-not-found"}
	}

	naviHwnd := wlFindMainWindow(uint32(os.Getpid()), false)
	if naviHwnd == 0 {
		a.guiLog("窗口联动：找不到 TOTKnavi 主窗口，跳过（稍后由前端重试）")
		return map[string]any{"ok": false, "reason": "navi-window-not-found"}
	}

	var before wlWindowPlacement
	before.Length = uint32(unsafe.Sizeof(before))
	if r, _, _ := wlGetWindowPlacement.Call(naviHwnd, uintptr(unsafe.Pointer(&before))); r == 0 {
		a.guiLog("窗口联动：读取当前窗口位置失败，跳过")
		return map[string]any{"ok": false, "reason": "navi-window-not-found"}
	}

	// 已经在目标屏且已最大化：幂等返回，且不再记快照（避免把副屏当成原始位置）。
	movedAlready := false
	if mon, ok := wlMonitorOf(monitors, naviHwnd); ok && mon.Name == target.Name {
		if zoomed, _, _ := wlIsZoomed.Call(naviHwnd); zoomed != 0 {
			movedAlready = true
		}
	}
	if movedAlready {
		a.guiLog("窗口联动：TOTKnavi 已在 %s 上最大化，无需移动", target.Name)
		return map[string]any{"ok": true, "moved": false, "reason": "already-there", "targetId": target.Name}
	}

	if r, _, _ := wlIsIconic.Call(naviHwnd); r != 0 {
		a.guiLog("窗口联动：TOTKnavi 窗口已最小化，跳过移屏（不打扰游戏）")
		return map[string]any{"ok": false, "reason": "navi-minimized"}
	}

	if !wlMoveMaximize(naviHwnd, target.Info.Work) {
		a.guiLog("窗口联动：移动到 %s 失败（SetWindowPlacement 返回 0）", target.Name)
		return map[string]any{"ok": false, "reason": "move-failed"}
	}

	a.mu.Lock()
	if !a.windowSaved {
		a.savedPlacement = before
		a.windowSaved = true
	}
	a.moveTargetName = target.Name
	a.mu.Unlock()

	primary, _ := wlPrimaryMonitor(monitors)
	a.guiLog("窗口联动：主屏 %s [%d,%d %dx%d] → TOTKnavi 移到副屏 %s [%d,%d %dx%d] 并最大化",
		primary.Name, primary.Info.Monitor.Left, primary.Info.Monitor.Top, primary.Info.Monitor.width(), primary.Info.Monitor.height(),
		target.Name, target.Info.Monitor.Left, target.Info.Monitor.Top, target.Info.Monitor.width(), target.Info.Monitor.height())
	return map[string]any{
		"ok":       true,
		"moved":    true,
		"monitors": len(monitors),
		"target":   target.Info.Monitor,
		"targetId": target.Name,
	}
}

// restoreNaviWindowIfNeeded 游戏退出时还原自动移屏前的窗口位置/大小/最大化状态。
//
// 只有两种情况会丢弃快照：
//  1. 真的写回成功；
//  2. 窗口已经不在目标副屏上（用户自己挪走了，尊重用户选择）。
//
// 其余情况（窗口暂时找不到、SetWindowPlacement 失败）一律保留快照，下次再试——
// 否则窗口会永远留在副屏，且下一局会把副屏记成"原始位置"。
func (a *App) restoreNaviWindowIfNeeded() {
	a.mu.Lock()
	if !a.windowSaved {
		a.mu.Unlock()
		return
	}
	wp := a.savedPlacement
	targetName := a.moveTargetName
	a.mu.Unlock()

	hwnd := wlFindMainWindow(uint32(os.Getpid()), true)
	if hwnd == 0 {
		a.guiLog("窗口联动：游戏退出，但找不到 TOTKnavi 主窗口，保留还原快照待下次")
		return
	}

	iconic, _, _ := wlIsIconic.Call(hwnd)
	mons := wlMonitors()
	// 窗口已被用户手动挪到别的屏：尊重用户，丢弃快照。
	if iconic == 0 && targetName != "" && len(mons) > 0 {
		if cur, ok := wlMonitorOf(mons, hwnd); ok && cur.Name != targetName {
			a.clearWindowSnapshot()
			a.guiLog("窗口联动：游戏退出，但窗口已被手动移到 %s，跳过还原", cur.Name)
			return
		}
	}

	if len(mons) > 0 && !wlPlacementOnAnyMonitor(wp, mons) {
		// 原位置所在显示器已经不在（拔线/关屏）：退回主屏最大化。
		if primary, ok := wlPrimaryMonitor(mons); ok {
			wp.ShowCmd = swShowMaximized
			wp.NormalPosition = primary.Info.Work
			a.guiLog("窗口联动：原位置所在显示器已不存在，改为在主屏 %s 最大化", primary.Name)
		}
	}
	if iconic != 0 {
		// 用户把窗口缩到了任务栏：只更新"还原后该出现的位置"，不把它弹出来打扰游戏。
		wp.ShowCmd = swShowMinimized
	}

	if !wlApplyPlacement(hwnd, wp) {
		a.guiLog("窗口联动：游戏退出，还原窗口位置失败（保留快照待下次）")
		return
	}
	a.clearWindowSnapshot()
	a.guiLog("窗口联动：游戏退出，TOTKnavi 已还原到自动移屏前的位置")
}

// clearWindowSnapshot 丢弃还原快照（只在真正用掉或用户接管时调用）。
func (a *App) clearWindowSnapshot() {
	a.mu.Lock()
	a.windowSaved = false
	a.moveTargetName = ""
	a.mu.Unlock()
}
