// gametime5.go — 按 5 分钟单位定位游戏内时钟（--gametime5）。
//
// 20:40 = 248 个 5 分钟单位。扫描 0~400 的 uint32，间隔 5 秒，
// 打印 +1 的候选（HUD 每次跳 5 分钟）。
package main

import (
	"encoding/binary"
	"fmt"
	"time"
)

func runGametime5Probe(p Platform) {
	lo, hi := uint32(0), uint32(400)
	fmt.Printf("  [gametime5] 扫描 0~%d 的 uint32，5 秒后找 +1 候选…\n", hi)

	blks := dragonBlocks(p)
	type cand struct {
		addr uintptr
		val  uint32
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
			for i := 0; i+4 <= len(buf); i += 4 {
				v := binary.LittleEndian.Uint32(buf[i:])
				if v >= lo && v <= hi {
					hits = append(hits, cand{b.Base + off + uintptr(i), v})
				}
			}
		}
	}
	fmt.Printf("  [gametime5] 扫描 %.1fs，命中 %d 个候选，5 秒后复读…\n", time.Since(t0).Seconds(), len(hits))
	time.Sleep(5 * time.Second)

	found := 0
	for _, c := range hits {
		buf := readMem(p.Handle(), c.addr, 4)
		if len(buf) < 4 {
			continue
		}
		v := binary.LittleEndian.Uint32(buf)
		delta := int64(v) - int64(c.val)
		if delta >= 1 && delta <= 2 {
			fmt.Printf("  [gametime5] ★ addr=0x%X %d → %d (Δ%d)\n", c.addr, c.val, v, delta)
			found++
		}
	}
	fmt.Printf("  [gametime5] 共 %d 个 +1 候选\n", found)
}
