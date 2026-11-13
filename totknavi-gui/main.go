package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 单实例：已有实例在跑时，把它的窗口拉到前台，本次启动到此结束。
	// 必须放在最前面——等 wails.Run 之后才拦就晚了：第二个实例会先初始化 WebView、
	// 拉起自己的核心，并把前一个实例的核心当成"残留进程"杀掉。
	if !acquireSingleInstance() {
		focusRunningInstance()
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "TOTKNavi 定位导航 · 王国之泪",
		Width:     960,
		Height:    720,
		MinWidth:  900,
		MinHeight: 600,
		Frameless: true, // 无边框：前端自绘标题栏（品牌区 --wails-draggable 拖拽，右侧窗口控制按钮）
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.onShutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
