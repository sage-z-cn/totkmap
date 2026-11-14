// playtime_probe.go — 在内存中定位游戏累计运行秒数（--playtime）。
//
// playtime.txt 的 total_seconds 是低频写入；内存里应有一份实时递增的计数。
// 同时扫描 uint32/uint64/float32/float64 四种表示，间隔 3 秒复读找增量。
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func readPlaytimeFile(p Platform) (int64, string) {
	var candidates []string
	for _, r := range p.SaveRoots() {
		dir := r
		for i := 0; i < 5; i++ {
			if filepath.Base(dir) == "profile" {
				candidates = append(candidates, filepath.Join(dir, "playtime.txt"))
				break
			}
			dir = filepath.Dir(dir)
		}
	}
	candidates = append(candidates, `D:\GameLibray\totk\totk-pc-recomp\game\profile\playtime.txt`)
	for _, c := range candidates {
		b, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		fields := strings.Fields(string(b))
		if len(fields) >= 2 && fields[0] == "total_seconds" {
			v, _ := strconv.ParseInt(fields[1], 10, 64)
			return v, c
		}
	}
	return 0, ""
}

const (
	pkU32 = iota
	pkU64
	pkF32
	pkF64
)

type playtimeCand struct {
	addr uintptr
	kind int
	num  float64
}

func runPlaytimeProbe(p Platform) {
	base, path := readPlaytimeFile(p)
	fmt.Printf("  [playtime] file=%s total_seconds=%d\n", path, base)
	if base == 0 {
		fmt.Println("  [!] 读不到 playtime.txt")
		return
	}
	lo := float64(base - 180)
	hi := float64(base + 600)

	blks := dragonBlocks(p)
	var hits []playtimeCand
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
					hits = append(hits, playtimeCand{a, pkF32, f32})
				}
				u64 := binary.LittleEndian.Uint64(buf[i:])
				f64 := math.Float64frombits(u64)
				if f64 >= lo && f64 <= hi {
					hits = append(hits, playtimeCand{a, pkF64, f64})
				}
				// uint 表示单独再做一次（float 位模式通常不落在小整数区间）
				if float64(u32) >= lo && float64(u32) <= hi {
					hits = append(hits, playtimeCand{a, pkU32, float64(u32)})
				}
				if float64(u64) >= lo && float64(u64) <= hi {
					hits = append(hits, playtimeCand{a, pkU64, float64(u64)})
				}
			}
		}
	}
	fmt.Printf("  [playtime] 扫描 %.1fs，命中 %d 个候选，3 秒后复读…\n", time.Since(t0).Seconds(), len(hits))
	time.Sleep(3 * time.Second)

	found := 0
	for _, c := range hits {
		var sz int
		switch c.kind {
		case pkU32, pkF32:
			sz = 4
		default:
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
		delta := v - c.num
		if math.Abs(delta) >= 0.5 {
			kindName := []string{"u32", "u64", "f32", "f64"}[c.kind]
			fmt.Printf("  [playtime] ★ addr=0x%X %s %.1f → %.1f (Δ%.1f)\n", c.addr, kindName, c.num, v, delta)
			found++
		}
	}
	if found == 0 {
		fmt.Println("  [!] 没有找到递增候选（游戏可能暂停，或计数单位不是秒）")
	}
}
