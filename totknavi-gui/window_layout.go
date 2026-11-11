// window_layout.go — 定位成功后把 TOTKnavi 窗口挪到游戏所在显示器之外的屏幕上。
//
// 规则：
//  1. 游戏窗口所在显示器视为"主屏"，TOTKnavi 要去的显示器视为"副屏"。
//  2. 只有一个显示器 → 不动。
//  3. 两个显示器 → 直接去另一个。
//  4. 三个及以上 → 优先右侧相邻（垂直重叠 ≥30%），其次左侧、下方、上方；
//     都不共边时按中心距离最近的；同分按左上角 X/Y 稳定排序。
//
// 移动使用 SetWindowPlacement(SW_SHOWMAXIMIZED)，不抢游戏焦点，窗口在目标屏最大化。

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	wlUser32 = syscall.NewLazyDLL("user32.dll")

	wlEnumWindows         = wlUser32.NewProc("EnumWindows")
	wlGetWindowThreadPID  = wlUser32.NewProc("GetWindowThreadProcessId")
	wlIsWindowVisible     = wlUser32.NewProc("IsWindowVisible")
	wlGetWindowRect       = wlUser32.NewProc("GetWindowRect")
	wlEnumDisplayMonitors = wlUser32.NewProc("EnumDisplayMonitors")
	wlGetMonitorInfoW     = wlUser32.NewProc("GetMonitorInfoW")
	wlMonitorFromWindow   = wlUser32.NewProc("MonitorFromWindow")
	wlGetWindowPlacement  = wlUser32.NewProc("GetWindowPlacement")
	wlSetWindowPlacement  = wlUser32.NewProc("SetWindowPlacement")
	wlIsZoomed            = wlUser32.NewProc("IsZoomed")
)

const (
	monitorDefaultToNearest = 2
	swShowMaximized         = 3
)

type wlRect struct {
	Left, Top, Right, Bottom int32
}

func (r wlRect) width() int32  { return r.Right - r.Left }
func (r wlRect) height() int32 { return r.Bottom - r.Top }

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
	pid      uint32
	best     uintptr
	bestArea int64
}

// EnumWindows 回调：只看指定 PID 的可见顶层窗口，取面积最大的作为主窗口。
// 上下文用包级变量传递（EnumWindows 同步回调），避免 uintptr 持有栈指针。
var wlFindCtx wlWindowFind

func wlEnumWindowProc(hwnd uintptr, _ uintptr) uintptr {
	ctx := &wlFindCtx
	var pid uint32
	wlGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != ctx.pid {
		return 1
	}
	if r, _, _ := wlIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	var rect wlRect
	if r, _, _ := wlGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r == 0 {
		return 1
	}
	area := int64(rect.width()) * int64(rect.height())
	if area > ctx.bestArea {
		ctx.best = hwnd
		ctx.bestArea = area
	}
	return 1
}

var wlEnumWindowCallback = syscall.NewCallback(wlEnumWindowProc)

// wlFindTopWindow 返回进程面积最大的可见顶层窗口。
func wlFindTopWindow(pid uint32) uintptr {
	if pid == 0 {
		return 0
	}
	wlFindCtx = wlWindowFind{pid: pid}
	wlEnumWindows.Call(wlEnumWindowCallback, 0)
	return wlFindCtx.best
}

type wlMonitorEnumCtx struct {
	monitors []wlMonitor
}

var wlMonitorCtx wlMonitorEnumCtx

func wlEnumMonitorProc(hMonitor uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
	mi := wlMonitorInfo{Size: uint32(unsafe.Sizeof(wlMonitorInfo{}))}
	if r, _, _ := wlGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); r != 0 {
		wlMonitorCtx.monitors = append(wlMonitorCtx.monitors, wlMonitor{
			Handle:  hMonitor,
			Info:    mi,
			Name:    syscall.UTF16ToString(mi.DeviceName[:]),
			Primary: mi.Flags&1 != 0,
		})
	}
	return 1
}

var wlEnumMonitorCallback = syscall.NewCallback(wlEnumMonitorProc)

func wlMonitors() []wlMonitor {
	wlMonitorCtx = wlMonitorEnumCtx{}
	wlEnumDisplayMonitors.Call(0, 0, wlEnumMonitorCallback, 0)
	return wlMonitorCtx.monitors
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

// overlapRatio 两段区间重叠长度 / 较短区间长度。
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

// wlPickTarget 从所有显示器里选"副屏"。preferred 为用户在设置里指定的显示器设备名；
// 为空/auto 或指定屏不存在、或指定屏正好是游戏所在屏时，回退到自动策略。
func wlPickTarget(monitors []wlMonitor, gameHandle uintptr, preferred string) (wlMonitor, bool) {
	if len(monitors) < 2 {
		return wlMonitor{}, false
	}
	gameIdx := -1
	for i, m := range monitors {
		if m.Handle == gameHandle {
			gameIdx = i
			break
		}
	}
	if gameIdx < 0 {
		return wlMonitor{}, false
	}
	if preferred != "" && preferred != "auto" {
		for _, m := range monitors {
			if m.Name == preferred && m.Handle != gameHandle {
				return m, true
			}
		}
	}
	others := make([]wlMonitor, 0, len(monitors)-1)
	for i, m := range monitors {
		if i != gameIdx {
			others = append(others, m)
		}
	}
	if len(others) == 1 {
		return others[0], true
	}

	game := monitors[gameIdx].Info.Monitor
	best := others[0]
	bestPrio, bestDist := 99, wlDistance(game, best.Info.Monitor)
	for _, m := range others[1:] {
		ovY := wlOverlapRatio(game.Top, game.Bottom, m.Info.Monitor.Top, m.Info.Monitor.Bottom)
		ovX := wlOverlapRatio(game.Left, game.Right, m.Info.Monitor.Left, m.Info.Monitor.Right)
		prio := 99
		dist := wlDistance(game, m.Info.Monitor)
		switch {
		case ovY >= 0.30 && m.Info.Monitor.Left >= game.Right:
			prio, dist = 0, float64(m.Info.Monitor.Left-game.Right) // 右侧
		case ovY >= 0.30 && m.Info.Monitor.Right <= game.Left:
			prio, dist = 1, float64(game.Left-m.Info.Monitor.Right) // 左侧
		case ovX >= 0.30 && m.Info.Monitor.Top >= game.Bottom:
			prio, dist = 2, float64(m.Info.Monitor.Top-game.Bottom) // 下方
		case ovX >= 0.30 && m.Info.Monitor.Bottom <= game.Top:
			prio, dist = 3, float64(game.Top-m.Info.Monitor.Bottom) // 上方
		}
		if prio < bestPrio || (prio == bestPrio && dist < bestDist) ||
			(prio == bestPrio && dist == bestDist && m.Info.Monitor.Left < best.Info.Monitor.Left) {
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
	wp.ShowCmd = swShowMaximized
	wp.NormalPosition = work
	if r, _, _ := wlSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); r == 0 {
		return false
	}
	return true
}

// ArrangeWindowForGame 定位成功后调用：把 TOTKnavi 移到游戏所在屏之外的屏幕上并最大化。
// 只做窗口布局，不改变定位状态；前端按设置决定是否调用。
func (a *App) ArrangeWindowForGame() map[string]any {
	a.mu.Lock()
	enabled := a.cfg == nil || a.cfg.AutoMoveToSecondary == nil || *a.cfg.AutoMoveToSecondary
	preferred := ""
	if a.cfg != nil {
		preferred = a.cfg.AutoMoveTarget
	}
	gamePid := a.gamePid
	a.mu.Unlock()
	if !enabled {
		return map[string]any{"ok": false, "reason": "disabled"}
	}
	if gamePid == 0 {
		gamePid, _ = findGameProcess(a.emuMode())
	}
	if gamePid == 0 {
		return map[string]any{"ok": false, "reason": "game-not-found"}
	}

	monitors := wlMonitors()
	if len(monitors) < 2 {
		return map[string]any{"ok": false, "reason": "single-monitor", "monitors": len(monitors)}
	}
	gameHwnd := wlFindTopWindow(gamePid)
	if gameHwnd == 0 {
		return map[string]any{"ok": false, "reason": "game-window-not-found"}
	}
	gameMon, _, _ := wlMonitorFromWindow.Call(gameHwnd, monitorDefaultToNearest)
	target, ok := wlPickTarget(monitors, gameMon, preferred)
	if !ok {
		return map[string]any{"ok": false, "reason": "target-not-found"}
	}
	naviHwnd := wlFindTopWindow(uint32(os.Getpid()))
	if naviHwnd == 0 {
		return map[string]any{"ok": false, "reason": "navi-window-not-found"}
	}
	// 移动前先记下原始 placement（位置/大小/最大化状态），游戏退出时还原。
	var before wlWindowPlacement
	before.Length = uint32(unsafe.Sizeof(before))
	savedOK := false
	if r, _, _ := wlGetWindowPlacement.Call(naviHwnd, uintptr(unsafe.Pointer(&before))); r != 0 {
		savedOK = true
	}
	if !wlMoveMaximize(naviHwnd, target.Info.Work) {
		return map[string]any{"ok": false, "reason": "move-failed"}
	}
	a.mu.Lock()
	if !a.windowSaved && savedOK {
		a.savedPlacement = before
		a.windowSaved = true
	}
	a.moveTargetHandle = target.Handle
	a.mu.Unlock()
	var gameInfo wlMonitor
	for _, m := range monitors {
		if m.Handle == gameMon {
			gameInfo = m
			break
		}
	}
	a.guiLog("窗口联动：游戏屏 %s [%d,%d %dx%d] → TOTKnavi 移到 %s [%d,%d %dx%d] 并最大化",
		gameInfo.Name, gameInfo.Info.Monitor.Left, gameInfo.Info.Monitor.Top, gameInfo.Info.Monitor.width(), gameInfo.Info.Monitor.height(),
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
// 只还原"仍在目标副屏上、仍处于最大化"的窗口；用户手动动过窗口就不还原。
func (a *App) restoreNaviWindowIfNeeded() {
	a.mu.Lock()
	if !a.windowSaved {
		a.mu.Unlock()
		return
	}
	wp := a.savedPlacement
	target := a.moveTargetHandle
	a.windowSaved = false
	a.moveTargetHandle = 0
	a.mu.Unlock()

	hwnd := wlFindTopWindow(uint32(os.Getpid()))
	if hwnd == 0 {
		return
	}
	if r, _, _ := wlIsZoomed.Call(hwnd); r == 0 {
		return
	}
	if target != 0 {
		mon, _, _ := wlMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
		if mon != target {
			return
		}
	}
	if r, _, _ := wlSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); r != 0 {
		a.guiLog("窗口联动：游戏退出，TOTKnavi 已还原到自动移屏前的位置")
	}
}
