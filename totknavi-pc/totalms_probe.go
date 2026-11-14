// totalms_probe.go — 读取 Recomp 的累计游玩毫秒（--totalms）。
//
// 反汇编 playtime_get 确认：全局 total_ms 在 RVA 0x0fd347c0（uint64 毫秒），
// 每秒 +1000。运行时地址 = ExeBase + RVA。
package main

import (
	"encoding/binary"
	"fmt"
	"time"
)

func runTotalMsProbe(p Platform, plat *totkpcPlatform) {
	base, size := plat.ExeBase()
	if base == 0 {
		fmt.Println("  [!] 拿不到 exe 基址")
		return
	}
	addr := base + 0x0fd347c0
	fmt.Printf("  [totalms] exeBase=0x%X size=%.1fMB  total_ms@0x%X\n",
		base, float64(size)/1048576.0, addr)

	var prev uint64
	for i := 0; i < 6; i++ {
		buf := readMem(p.Handle(), addr, 8)
		if len(buf) < 8 {
			fmt.Println("  [!] 读取失败")
			return
		}
		v := binary.LittleEndian.Uint64(buf)
		if i == 0 {
			fmt.Printf("  %s total_ms=%d (%.3f 小时)\n", time.Now().Format("15:04:05"), v, float64(v)/3600000.0)
		} else {
			fmt.Printf("  %s total_ms=%d  Δ=%d (%.0f/s)\n",
				time.Now().Format("15:04:05"), v, v-prev, float64(v-prev))
		}
		prev = v
		time.Sleep(time.Second)
	}
}
