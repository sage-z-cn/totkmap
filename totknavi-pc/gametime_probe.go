// gametime_probe.go — 定位游戏内 HUD 时钟（--gametime <minutes>）。
//
// 游戏内时间 = HH*60+MM。正常 1 现实秒 = 1 游戏分钟；HUD 可能按 5 分钟跳。
// 扫描目标值附近的 uint32/float32，间隔 5 秒复读，打印增量 3~8 的候选。
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

func runGametimeProbe(p Platform, target int) {
	lo := float64(target - 90)
	hi := float64(target + 90)
	fmt.Printf("  [gametime] 目标 %d 分钟，扫描 %.0f ~ %.0f，5 秒后复读…\n", target, lo, hi)

	blks := dragonBlocks(p)
	type cand struct {
		addr uintptr
		kind int
		val  float64
	}
	var hits []cand
	t0 := time.Now()
	for _, b := range blks {
		for off := uintptr(0); off < b.Size; off += 8 << 20 {
			n := int(8 << 20)
			if remain := int(b.Size - off); remain < n {
				n = remain
			}
			buf := readMem(p.Handle(), b.Base+off, n)
			for i := 0; i+8 <= len(buf); i += 4 {
				a := b.Base + off + uintptr(i)
				u32 := binary.LittleEndian.Uint32(buf[i:])
				f32 := float64(math.Float32frombits(u32))
				if f32 >= lo && f32 <= hi {
					hits = append(hits, cand{a, pkF32, f32})
				}
				if float64(u32) >= lo && float64(u32) <= hi {
					hits = append(hits, cand{a, pkU32, float64(u32)})
				}
				f64 := math.Float64frombits(binary.LittleEndian.Uint64(buf[i:]))
				if f64 >= lo && f64 <= hi {
					hits = append(hits, cand{a, pkF64, f64})
				}
			}
		}
	}
	fmt.Printf("  [gametime] 扫描 %.1fs，命中 %d 个候选，5 秒后复读…\n", time.Since(t0).Seconds(), len(hits))
	time.Sleep(5 * time.Second)

	found := 0
	for _, c := range hits {
		sz := 4
		if c.kind == pkF64 {
			sz = 8
		}
		buf := readMem(p.Handle(), c.addr, sz)
		if len(buf) < sz {
			continue
		}
		var v float64
		switch c.kind {
		case pkU32:
			v = float64(binary.LittleEndian.Uint32(buf))
		case pkF32:
			v = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf)))
		case pkF64:
			v = math.Float64frombits(binary.LittleEndian.Uint64(buf))
		}
		delta := v - c.val
		if delta >= 3 && delta <= 8 {
			kindName := []string{"u32", "u64", "f32", "f64"}[c.kind]
			fmt.Printf("  [gametime] ★ addr=0x%X %s %.0f → %.0f (Δ%.0f)\n", c.addr, kindName, c.val, v, delta)
			found++
		}
	}
	if found == 0 {
		fmt.Println("  [!] 没找到 +3~8 的候选")
	}
}
