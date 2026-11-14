// launchgame.go — 从 GUI 直接启动 PC 重编译版游戏。
//
// 目的：把「启动游戏 → 检测进程 → 自动定位 → 锁定后切地图」连成一条链。
// 只负责拉起游戏进程；核心仍由 gamewatch.go 检测到游戏进程后启动。
// 游戏路径优先用配置，其次按 play.ps1 的目录约定探测（GUI 与 totk-pc-recomp 同级）。

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// gameExeCandidates 常见安装布局（相对 GUI 工作目录）。
func (a *App) gameExeCandidates() []string {
	rels := []string{
		filepath.Join("..", "totk-pc-recomp", "game", "totk.exe"),
		filepath.Join("..", "TotK-Recomp", "game", "totk.exe"),
		filepath.Join("..", "TOTK-PC", "game", "totk.exe"),
		filepath.Join("..", "TotKPC", "game", "totk.exe"),
		filepath.Join("..", "totk", "game", "totk.exe"),
		filepath.Join("game", "totk.exe"),
	}
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		p := filepath.Join(a.workDir, r)
		if abs, err := filepath.Abs(p); err == nil {
			out = append(out, abs)
		}
	}
	return out
}

// resolveGameExe 返回可用的游戏 exe 路径；找不到返回空串。
// 顺序：配置 → 常见目录 → 已在运行的进程路径。
func (a *App) resolveGameExe() string {
	a.mu.Lock()
	cfgPath := ""
	if a.cfg != nil {
		cfgPath = a.cfg.GameExe
	}
	a.mu.Unlock()

	if cfgPath != "" && pathExists(cfgPath) {
		return cfgPath
	}
	for _, p := range a.gameExeCandidates() {
		if pathExists(p) {
			return p
		}
	}
	if pid, _ := findGameProcess(a.emuMode()); pid != 0 {
		if p := gwProcessPath(pid); p != "" {
			return p
		}
	}
	return ""
}

// GameExe 供前端初始化时查询当前可用路径。
func (a *App) GameExe() map[string]any {
	p := a.resolveGameExe()
	return map[string]any{"path": p, "ok": p != ""}
}

// PickGameExe 选择 totk.exe 并持久化。
func (a *App) PickGameExe() string {
	defaultDir := a.workDir
	if p := a.resolveGameExe(); p != "" {
		defaultDir = filepath.Dir(p)
	}
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "选择 totk.exe",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "游戏程序 (totk.exe)", Pattern: "totk.exe"},
			{DisplayName: "可执行文件 (*.exe)", Pattern: "*.exe"},
		},
	})
	if err != nil || p == "" {
		return ""
	}
	a.rememberGameExe(p)
	return p
}

// LaunchGame 启动游戏本体。返回值给前端做提示/二次选择：
//
//	{ok:true, pid, exe}            已启动
//	{ok:true, already:true, pid}   已在运行
//	{ok:false, needPath:true}      找不到路径，需要用户选择
//	{ok:false, error}              启动失败
func (a *App) LaunchGame() map[string]any {
	exe := a.resolveGameExe()
	// 只有「当前跑着的进程就是本配置的游戏本体」才算已在运行：auto 模式下
	// Ryujinx/Eden 常驻托盘很常见，只按进程名匹配会把它们误判成游戏，
	// 表现为点了「启动游戏」却没有反应。
	if pid, name := findGameProcess(a.emuMode()); pid != 0 {
		running := strings.ToLower(gwProcessPath(pid))
		if exe == "" || running == "" || running == strings.ToLower(exe) {
			return map[string]any{"ok": true, "already": true, "pid": pid, "name": name}
		}
	}
	if exe == "" {
		return map[string]any{"ok": false, "needPath": true, "error": "未找到游戏程序，请先选择 totk.exe 路径"}
	}
	cmd := exec.Command(exe)
	cmd.Dir = filepath.Dir(exe) // 游戏按工作目录找 romfs / 配置
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return map[string]any{"ok": false, "error": "启动失败: " + err.Error()}
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	a.rememberGameExe(exe)
	a.guiLog("启动游戏：%s (PID %d)", exe, pid)
	return map[string]any{"ok": true, "pid": pid, "exe": exe}
}

// rememberGameExe 把路径写回配置，下次启动不再探测。
func (a *App) rememberGameExe(p string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		a.cfg = a.loadConfig()
	}
	if a.cfg.GameExe == p {
		return
	}
	a.cfg.GameExe = p
	buf, err := json.MarshalIndent(a.cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(a.configPath(), buf, 0644)
	}
}
