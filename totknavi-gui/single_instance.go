// single_instance.go — 单实例锁：重复打开时把已在运行的窗口拉到前台。
//
// 为什么需要：TOTKNavi 每次启动都会拉起定位核心，并清理"上一次残留的核心进程"。
// 同时跑两个实例时，后启动的会把前一个实例的核心杀掉、再抢 8766 端口，用户看到的
// 是定位莫名中断、网页端时好时坏。这里让第二个实例只做一件事：把第一个实例的窗口
// 还原到前台，然后自己退出——不碰核心、不碰端口、不初始化 WebView。
//
// 身份判定用命名互斥量 + exe 全路径，不按窗口标题匹配：标题随文案改动，exe 路径稳定，
// 而且能顺手把同目录的 totknavi-core.exe 排除在外（核心没有窗口）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// 命名互斥量：Local\ 表示仅当前登录会话，多用户同时登录互不干扰。
	singleInstanceMutex = `Local\totknavi-gui`

	swRestore = 9 // SW_RESTORE：把最小化的窗口还原到最小化前的状态
)

var (
	siUser32 = syscall.NewLazyDLL("user32.dll")

	siEnumWindows         = siUser32.NewProc("EnumWindows")
	siGetWindowThreadPID  = siUser32.NewProc("GetWindowThreadProcessId")
	siIsWindowVisible     = siUser32.NewProc("IsWindowVisible")
	siGetWindowRect       = siUser32.NewProc("GetWindowRect")
	siIsIconic            = siUser32.NewProc("IsIconic")
	siShowWindow          = siUser32.NewProc("ShowWindow")
	siBringWindowToTop    = siUser32.NewProc("BringWindowToTop")
	siSetForegroundWindow = siUser32.NewProc("SetForegroundWindow")
	siGetForegroundWindow = siUser32.NewProc("GetForegroundWindow")
	siAttachThreadInput   = siUser32.NewProc("AttachThreadInput")

	siKernel32           = syscall.NewLazyDLL("kernel32.dll")
	siGetCurrentThreadId = siKernel32.NewProc("GetCurrentThreadId")
)

// siMutexHandle 由本进程持有到退出，不主动关闭：关掉就等于放行下一个实例。
var siMutexHandle windows.Handle

// acquireSingleInstance 创建命名互斥量。返回 false 表示已有实例在运行。
// 除"已存在"以外的异常（权限受限等）不拦截启动——宁可多开一次，也不能开不了。
func acquireSingleInstance() bool {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(singleInstanceMutex))
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return true
	}
	siMutexHandle = h
	return true
}

// siWindowSearch EnumWindows 的回调上下文（回调同步执行，包级变量即可）。
type siWindowSearch struct {
	exePath  string
	best     uintptr
	bestArea int64
}

var siSearchCtx siWindowSearch

// siEnumWindowProc 收集"其它进程的、可见的、exe 路径与本程序一致"的顶层窗口，
// 取面积最大的那个作为主窗口。
func siEnumWindowProc(hwnd uintptr, _ uintptr) uintptr {
	ctx := &siSearchCtx
	if r, _, _ := siIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	var pid uint32
	siGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 || pid == uint32(os.Getpid()) {
		return 1
	}
	if !strings.EqualFold(gwProcessPath(pid), ctx.exePath) {
		return 1
	}
	var rect wlRect
	if r, _, _ := siGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r == 0 {
		return 1
	}
	if area := int64(rect.width()) * int64(rect.height()); area > ctx.bestArea {
		ctx.best, ctx.bestArea = hwnd, area
	}
	return 1
}

var siEnumWindowCallback = syscall.NewCallback(siEnumWindowProc)

// siFindRunningWindow 返回已运行实例的主窗口句柄；找不到返回 0。
func siFindRunningWindow(exePath string) uintptr {
	siSearchCtx = siWindowSearch{exePath: exePath}
	siEnumWindows.Call(siEnumWindowCallback, 0)
	return siSearchCtx.best
}

// siOwnExePath 本程序 exe 全路径（解析符号链接后），用于和对方进程的路径比对。
func siOwnExePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return filepath.Clean(exe)
}

func siWindowThreadID(hwnd uintptr) uintptr {
	if hwnd == 0 {
		return 0
	}
	tid, _, _ := siGetWindowThreadPID.Call(hwnd, 0)
	return tid
}

func siCurrentThreadID() uintptr {
	tid, _, _ := siGetCurrentThreadId.Call()
	return tid
}

// siForceForeground 还原并激活窗口。
//
// 只在窗口已最小化时调 ShowWindow(SW_RESTORE)。已显示的窗口不再调 ShowWindow：
// 对最大化中的 Wails 无边框窗口调用 SW_SHOW 会把它取消最大化（实测），
// 而最大化正是"定位成功后自动移屏"的常态。
//
// 前台窗口由系统锁定，直接 SetForegroundWindow 常被拒绝；此时临时挂到当前前台
// 窗口与目标窗口的输入队列上再置前，完成后立即解除挂接。
func siForceForeground(hwnd uintptr) bool {
	if r, _, _ := siIsIconic.Call(hwnd); r != 0 {
		siShowWindow.Call(hwnd, swRestore)
	}
	siBringWindowToTop.Call(hwnd)
	if r, _, _ := siSetForegroundWindow.Call(hwnd); r != 0 {
		return true
	}

	cur := siCurrentThreadID()
	fg, _, _ := siGetForegroundWindow.Call()
	fgTid := siWindowThreadID(fg)
	tgtTid := siWindowThreadID(hwnd)
	if fgTid != 0 && fgTid != cur {
		siAttachThreadInput.Call(cur, fgTid, 1)
	}
	if tgtTid != 0 && tgtTid != cur {
		siAttachThreadInput.Call(cur, tgtTid, 1)
	}
	siBringWindowToTop.Call(hwnd)
	r, _, _ := siSetForegroundWindow.Call(hwnd)
	if tgtTid != 0 && tgtTid != cur {
		siAttachThreadInput.Call(cur, tgtTid, 0)
	}
	if fgTid != 0 && fgTid != cur {
		siAttachThreadInput.Call(cur, fgTid, 0)
	}
	return r != 0
}

// focusRunningInstance 把已在运行的同类实例主窗口拉到前台。
// 对方可能还在初始化（窗口尚未创建），这里短暂轮询等待，最多约 3 秒。
func focusRunningInstance() bool {
	exe := siOwnExePath()
	if exe == "" {
		return false
	}
	for i := 0; i < 20; i++ {
		if hwnd := siFindRunningWindow(exe); hwnd != 0 {
			return siForceForeground(hwnd)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}
