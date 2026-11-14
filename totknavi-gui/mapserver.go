package main

// GUI 内置网页地图静态托管。
//
// 内嵌地图此前只有核心（totknavi-core.exe）在 8766 上托管 app/ 时才可用：
// 定位服务没启动 → 8766 无监听 → 地图页只能显示“定位服务未启动”。
// 这里由 GUI 自己在 127.0.0.1 上常驻托管同一份 app/，核心开不开都能看图；
// 网页里的 /pos 轮询仍按原逻辑连 8766，核心起来后自动接上，无需重载 iframe。

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// localMapPort 固定端口：地图的 localStorage / Service Worker 按 origin 隔离，
// 每次换端口会丢跟随、层级等地图设置。被占用时顺延 8768、8769…
const localMapPort = 8767

// localMapRoot 定位网页地图目录：
// exe 同级 app/（发布布局）→ exe 上级 app/ → 工作目录 app/ 与 ../app（wails dev / 仓库内运行）。
func (a *App) localMapRoot() string {
	roots := []string{filepath.Join(a.workDir, "app")}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(exeDir, "app"),
			filepath.Join(exeDir, "..", "app"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots,
			filepath.Join(wd, "app"),
			filepath.Join(wd, "..", "app"),
		)
	}
	for _, r := range roots {
		if pathExists(filepath.Join(r, "index.html")) {
			return r
		}
	}
	return ""
}

// startLocalMapServer 启动 GUI 自带地图托管；没有 app/ 时静默跳过（地图回退在线站点）。
func (a *App) startLocalMapServer() {
	root := a.localMapRoot()
	if root == "" {
		return
	}
	var ln net.Listener
	var err error
	for port := localMapPort; port < localMapPort+10; port++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if ln == nil {
		a.guiLog("内置地图托管失败：%v", err)
		return
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(root))}
	url := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)

	a.mu.Lock()
	a.mapRoot = root
	a.localMapURL = url
	a.mapSrv = srv
	a.mu.Unlock()

	go srv.Serve(ln)
	a.guiLog("内置地图已托管：%s -> %s", url, root)
}

// stopLocalMapServer 随 GUI 退出关闭内置地图托管。
func (a *App) stopLocalMapServer() {
	a.mu.Lock()
	srv := a.mapSrv
	a.mapSrv = nil
	a.localMapURL = ""
	a.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}
}

// LocalMapURL 内置地图托管的 origin（无 app/ 目录时为空串），前端挂 iframe 用。
func (a *App) LocalMapURL() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.localMapURL
}
