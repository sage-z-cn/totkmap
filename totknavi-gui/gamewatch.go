package main

// gamewatch.go — 游戏进程 watcher。
//
// 职责：按 cfg.Emu 轮询系统进程，识别游戏本体（PC 重编译版 totk.exe / Ryujinx /
// Eden），把"游戏在跑"这一事实映射到 GUI 状态，并在需要时自动拉起/重启定位核心。
//
// 为什么需要：核心退出后 GUI 不再自动恢复，用户每次都得先开游戏再回来点开始；
// 自动启动又常发生在主菜单/加载界面，核心那时扫不到真身。这里只负责"游戏起来了"
// 这个信号，真身锁定由核心自己的延迟重试完成。
//
// 去抖：连续 2 次命中才算启动、连续 3 次未命中才算退出——避免启动器/过场
// （游戏进程短时不可见）把核心反复拉起又杀掉。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	gwKernel32 = syscall.NewLazyDLL("kernel32.dll")

	gwSnapshot         = gwKernel32.NewProc("CreateToolhelp32Snapshot")
	gwProcess32FirstW  = gwKernel32.NewProc("Process32FirstW")
	gwProcess32NextW   = gwKernel32.NewProc("Process32NextW")
	gwCloseHandle      = gwKernel32.NewProc("CloseHandle")
	gwOpenProcess      = gwKernel32.NewProc("OpenProcess")
	gwTerminateProcess = gwKernel32.NewProc("TerminateProcess")
	gwQueryFullPath    = gwKernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	th32csSnapProcess              = 0x00000002
	processTerminate               = 0x0001
	processQueryLimitedInformation = 0x1000

	// 轮询与去抖
	gamePollInterval = 1 * time.Second
	gameHitConfirm   = 2 // 连续命中 2 次 → 判定游戏启动
	gameMissConfirm  = 3 // 连续未命中 3 次 → 判定游戏退出

	coreProcName = "totknavi-core.exe"
)

// gwProcessPath 取进程完整 exe 路径（用于游戏已在运行时反推路径）。
func gwProcessPath(pid uint32) string {
	h, _, _ := gwOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer gwClose(h)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	r, _, _ := gwQueryFullPath.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	return string(utf16.Decode(buf[:size]))
}

// ProcessEntry32W 与 Windows PROCESSENTRY32W 布局一致（x64 = 568 字节）。
type ProcessEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

func init() {
	if unsafe.Sizeof(ProcessEntry32W{}) != 568 {
		panic("ProcessEntry32W size mismatch")
	}
}

// guiLog 往核心日志文件追加一行（PollLogs 会增量读到，前端日志面板可见）。
// 与核心共用 xnavi-gui-core.log，统一加时间戳与 [game] 前缀便于区分来源。
var guiLogMu sync.Mutex

func (a *App) guiLog(format string, args ...any) {
	line := fmt.Sprintf("%s [game] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
	guiLogMu.Lock()
	defer guiLogMu.Unlock()
	f, err := os.OpenFile(filepath.Join(a.workDir, "xnavi-gui-core.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()
}

func gwUTF16ToString(u []uint16) string {
	n := 0
	for n < len(u) && u[n] != 0 {
		n++
	}
	return string(utf16.Decode(u[:n]))
}

func gwClose(h uintptr) {
	if h != 0 {
		gwCloseHandle.Call(h)
	}
}

// matchGameProcRank 进程名（已转小写）与当前配置下游戏本体的匹配优先级：
// 0 = PC 重编译版 totk.exe（最优），1 = 模拟器，-1 = 不匹配。
//
// auto 模式必须区分优先级：Ryujinx/Eden 常驻托盘时若按枚举顺序先命中模拟器，
// watcher 会把模拟器当成"游戏在跑"——游戏退出后永远不判定退出，移屏的窗口还原
// 就再也不触发。指定平台时只认那一个。
func matchGameProcRank(name, emu string) int {
	isPC := name == "totk.exe"
	isRyujinx := strings.HasPrefix(name, "ryujinx")
	isEden := strings.HasPrefix(name, "eden") && !strings.HasPrefix(name, "eden-core")
	switch emu {
	case "ryujinx":
		if isRyujinx {
			return 0
		}
	case "eden":
		if isEden {
			return 0
		}
	case "auto", "":
		if isPC {
			return 0
		}
		if isRyujinx || isEden {
			return 1
		}
	default: // "pc" 及未知值：按 PC 重编译版处理
		if isPC {
			return 0
		}
	}
	return -1
}

// findGameProcess 枚举进程，返回优先级最高的游戏进程（排除自身与核心进程）。
// 同优先级取先枚举到的那个。
func findGameProcess(emu string) (uint32, string) {
	snap, _, _ := gwSnapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == uintptr(^uintptr(0)) {
		return 0, ""
	}
	defer gwClose(snap)

	myPid := uint32(os.Getpid())
	var e ProcessEntry32W
	e.Size = uint32(unsafe.Sizeof(e))
	bestPid, bestName, bestRank := uint32(0), "", 99
	r, _, _ := gwProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		name := gwUTF16ToString(e.ExeFile[:])
		lower := strings.ToLower(name)
		if e.ProcessID != myPid && lower != coreProcName {
			if rank := matchGameProcRank(lower, emu); rank >= 0 && rank < bestRank {
				bestPid, bestName, bestRank = e.ProcessID, name, rank
				if rank == 0 {
					break // 已是最优优先级，不必继续扫
				}
			}
		}
		r, _, _ = gwProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return bestPid, bestName
}

// killOrphanCore 结束系统里残留的 totknavi-core.exe（GUI 重启后不认识的老进程），
// 避免它继续占着 8766 端口。返回结束的进程数。
func killOrphanCore() int {
	snap, _, _ := gwSnapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == uintptr(^uintptr(0)) {
		return 0
	}
	defer gwClose(snap)

	myPid := uint32(os.Getpid())
	var e ProcessEntry32W
	e.Size = uint32(unsafe.Sizeof(e))
	n := 0
	r, _, _ := gwProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		name := strings.ToLower(gwUTF16ToString(e.ExeFile[:]))
		if e.ProcessID != myPid && name == coreProcName {
			h, _, _ := gwOpenProcess.Call(processTerminate, 0, uintptr(e.ProcessID))
			if h != 0 {
				gwTerminateProcess.Call(h, 1)
				gwClose(h)
				n++
			}
		}
		r, _, _ = gwProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return n
}

// watchGame 每秒轮询游戏进程，带去抖，驱动自动启动/停止核心。随 ctx 退出。
func (a *App) watchGame(ctx context.Context) {
	ticker := time.NewTicker(gamePollInterval)
	defer ticker.Stop()

	hits, misses := 0, 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pid, name := findGameProcess(a.emuMode())
			if pid != 0 {
				misses = 0
				hits++
				if hits < gameHitConfirm {
					continue
				}
				a.onGameFound(pid, name)
				continue
			}
			hits = 0
			misses++
			a.mu.Lock()
			running := a.gameRunning
			a.mu.Unlock()
			if running && misses >= gameMissConfirm {
				a.onGameGone()
			}
		}
	}
}

// emuMode 当前配置的平台（auto/ryujinx/eden/pc）。
func (a *App) emuMode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg != nil && a.cfg.Emu != "" {
		return a.cfg.Emu
	}
	return "auto"
}

// onGameFound 游戏进程已确认启动：更新状态，按配置决定是否自动启动核心。
// 同一局游戏里每秒都会调用一次，因此只在状态变化时写日志；核心意外退出时在这里补拉。
func (a *App) onGameFound(pid uint32, name string) {
	a.mu.Lock()
	same := a.gameRunning && a.gamePid == pid
	if !same {
		a.gamePid = pid
		a.gameName = name
		a.gameRunning = true
	}
	autoStart := a.cfg == nil || a.cfg.AutoStartOnGame == nil || *a.cfg.AutoStartOnGame
	stopped := a.userStopped
	coreGamePid := a.coreGamePid
	coreOn := a.cmd != nil && a.cmd.Process != nil
	a.mu.Unlock()

	if !same {
		a.guiLog("检测到游戏进程：%s (PID %d)", name, pid)
	}

	if !autoStart || stopped {
		if !same {
			if stopped {
				a.guiLog("跳过自动启动：本局已手动停止核心，不再自动拉起 (PID %d)", pid)
			} else {
				a.guiLog("跳过自动启动：随游戏启动已关闭，仅更新状态 (PID %d)", pid)
			}
		}
		return
	}

	if !coreOn {
		a.guiLog("自动启动定位核心（游戏 %s PID %d）", name, pid)
		if err := a.startCoreInternal(); err != nil {
			a.guiLog("自动启动失败：%v", err)
			return
		}
		a.mu.Lock()
		a.coreGamePid = pid
		a.mu.Unlock()
		return
	}

	if coreGamePid != pid {
		a.guiLog("游戏进程已更换（PID %d → %d），重启定位核心", coreGamePid, pid)
		a.mu.Lock()
		a.stopCoreLocked()
		a.mu.Unlock()
		if err := a.startCoreInternal(); err != nil {
			a.guiLog("自动启动失败：%v", err)
			return
		}
		a.mu.Lock()
		a.coreGamePid = pid
		a.mu.Unlock()
		return
	}
}

// onGameGone 游戏进程连续 3 次未命中：停核心并复位自动启动的抑制位。
func (a *App) onGameGone() {
	a.mu.Lock()
	pid, name := a.gamePid, a.gameName
	hadCore := a.cmd != nil
	a.gamePid = 0
	a.gameName = ""
	a.gameRunning = false
	a.userStopped = false
	a.coreGamePid = 0
	a.stopCoreLocked()
	a.mu.Unlock()

	if hadCore {
		a.guiLog("游戏已退出（%s PID %d），定位核心已停止", name, pid)
	} else {
		a.guiLog("游戏已退出（%s PID %d）", name, pid)
	}
	a.restoreNaviWindowIfNeeded()
}
