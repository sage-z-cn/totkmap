// worldtime_probe.go — 定位游戏世界运行时间（--worldtime）。
//
// 假设：世界时间以 60fps 帧计数形式累计，值 ≈ playtime_seconds × 60。
// 扫描该范围附近的 uint32/float32/uint64/float64，间隔 3 秒复读，
// 打印增量接近 180（60fps × 3s）的候选。找到后再验证暂停时是否停止。
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

func runWorldtimeProbe(p Platform) {
	base, path := readPlaytimeFile(p)
	fmt.Printf("  [world] playtime=%d (%s)\n", base, path)
	if base == 0 {
		return
	}
	center := float64(base) * 60
	lo, hi := center-60000, center+60000
	fmt.Printf("  [world] 扫描范围 %.0f ~ %.0f（60fps 帧计数假设）\n", lo, hi)

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
				u64 := binary.LittleEndian.Uint64(buf[i:])
				f64 := math.Float64frombits(u64)
				if f64 >= lo && f64 <= hi {
					hits = append(hits, cand{a, pkF64, f64})
				}
				if float64(u32) >= lo && float64(u32) <= hi {
					hits = append(hits, cand{a, pkU32, float64(u32)})
				}
				if float64(u64) >= lo && float64(u64) <= hi {
					hits = append(hits, cand{a, pkU64, float64(u64)})
				}
			}
		}
	}
	fmt.Printf("  [world] 扫描 %.1fs，命中 %d 个候选，3 秒后复读…\n", time.Since(t0).Seconds(), len(hits))
	time.Sleep(3 * time.Second)

	found := 0
	for _, c := range hits {
		sz := 4
		if c.kind == pkU64 || c.kind == pkF64 {
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
		case pkU64:
			v = float64(binary.LittleEndian.Uint64(buf))
		case pkF32:
			v = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf)))
		case pkF64:
			v = math.Float64frombits(binary.LittleEndian.Uint64(buf))
		}
		delta := v - c.val
		// 60fps × 3s ≈ 180；掉帧/变速放宽到 60~300
		if delta >= 60 && delta <= 300 {
			kindName := []string{"u32", "u64", "f32", "f64"}[c.kind]
			fmt.Printf("  [world] ★ addr=0x%X %s %.1f → %.1f (Δ%.1f)\n", c.addr, kindName, c.val, v, delta)
			found++
		}
	}
	if found == 0 {
		fmt.Println("  [!] 没找到 60~300 增量的候选（可能单位不是帧）")
	}
}
