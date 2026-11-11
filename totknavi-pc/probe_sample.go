// probe_sample.go — 坐标地址实时采样（--sample）。
//
// 用途：在实机中同时盯多个候选坐标地址，判断哪个才是"跟着角色走"的活坐标。
// 只读内存，200ms 一行，适合边走边看。

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// sampleAddrs 同时采样多个地址的 12 字节坐标三元组。
func sampleAddrs(p Platform, addrs []uintptr, seconds int) {
	h := p.Handle()
	if len(addrs) == 0 {
		fmt.Println("  [sample] 没有地址")
		return
	}
	if seconds <= 0 {
		seconds = 10
	}
	fmt.Printf("  [sample] 采样 %d 个地址，持续 %ds（请让角色走动）\n", len(addrs), seconds)
	fmt.Println("  ------------------------------------------------------------------")
	end := time.Now().Add(time.Duration(seconds) * time.Second)
	for time.Now().Before(end) {
		var sb strings.Builder
		sb.WriteString(time.Now().Format("15:04:05.200"))
		for _, a := range addrs {
			b := readMem(h, a, 12)
			if len(b) < 12 {
				fmt.Fprintf(&sb, "  | 0x%X <unreadable>", a)
				continue
			}
			x := math.Float32frombits(binary.LittleEndian.Uint32(b[0:]))
			zs := math.Float32frombits(binary.LittleEndian.Uint32(b[4:]))
			yn := math.Float32frombits(binary.LittleEndian.Uint32(b[8:]))
			fmt.Fprintf(&sb, "  | (%.2f, %.2f, %.2f)", x, -yn, zs-elevBias)
		}
		fmt.Println(sb.String())
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println("  ------------------------------------------------------------------")
	fmt.Println("  [sample] 采样结束。")
}
