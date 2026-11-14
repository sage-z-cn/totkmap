package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 核心未启动时地图可用性依赖 GUI 自带托管：确认 app/index.html 能被正常访问。
func TestLocalMapServerServesIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, "app", "index.html")
	if err := os.WriteFile(index, []byte("<title>totk map</title>"), 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{workDir: dir}
	a.startLocalMapServer()
	defer a.stopLocalMapServer()

	base := a.LocalMapURL()
	if base == "" {
		t.Fatal("内置地图托管未启动")
	}
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("请求内置地图失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "totk map") {
		t.Fatalf("index.html 内容不符: %q", string(body))
	}
}
