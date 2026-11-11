// probe_chain.go — 静态指针链现场诊断（--probe）。
//
// 背景：knownStaticChains 的偏移来自主模块镜像 0x472b000 的 Recomp 构建，
// 而不同构建的主模块大小不同（实机 0.3.13 为 0x3adf000），旧偏移全部越界，
// L1 静态直读因此永远失败，只能退回 L2 扫描。
//
// 本模式做三件事，全部只读：
//  1. 打印主模块映射区大小，逐个校验已知链偏移是否越界、指针是否可读、坐标是否合法；
//  2. 全范围扫一遍 28B 物理 RingBuffer 长队列（Run>=4）槽，列出当前真实运动槽候选；
//  3. 在主模块里反查"指向这些运动槽"的全局指针，给出 (指针偏移, 结构偏移) 候选，
//     供重新标定 knownStaticChains。

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

const (
	probePtrSpan   = uintptr(0x400) // 指针目标允许落在坐标槽前 0x400 内
	probeMaxShow   = 24
	probeReadChunk = 8 << 20
)

// probeMainRegionSize 返回主模块起始处映射区大小。
func probeMainRegionSize(h uintptr, base uintptr) uintptr {
	if h == 0 || base == 0 {
		return 0
	}
	var mbi MemoryBasicInformation
	if virtualQueryEx(h, base, &mbi) == 0 {
		return 0
	}
	return mbi.RegionSize
}

// probeChain 执行静态链现场诊断。anchor 非空时按距离排序并只对近处候选做指针反查。
func probeChain(p Platform, plat *totkpcPlatform, anchor *[3]float32) {
	h := p.Handle()
	mb := plat.MainBase()
	ms := probeMainRegionSize(h, mb)
	fmt.Printf("\n  [probe] pid=%d main=0x%X regionSize=0x%X (%.1fMB)\n", p.PID(), mb, ms, float64(ms)/1048576.0)
	if mb == 0 || ms == 0 {
		fmt.Println("  [probe] 主模块不可用，终止。")
		return
	}
	if head := readMem(h, mb, 16); len(head) >= 12 {
		fmt.Printf("  [probe] 头 16B: % x\n", head)
	}

	fmt.Println("\n  === 已知静态链校验 ===")
	for _, c := range knownStaticChains {
		if c.ptrOffset+8 > ms {
			fmt.Printf("  [chain] ptrOff=0x%-8X structOff=0x%-3X  越界（模块 0x%X）\n", c.ptrOffset, c.structOffset, ms)
			continue
		}
		pb := readMem(h, mb+c.ptrOffset, 8)
		if len(pb) < 8 {
			fmt.Printf("  [chain] ptrOff=0x%-8X  指针不可读\n", c.ptrOffset)
			continue
		}
		ptr := uintptr(binary.LittleEndian.Uint64(pb))
		if ptr == 0 || ptr < 0x100000000 {
			fmt.Printf("  [chain] ptrOff=0x%-8X  ptr=0x%X（空/非法）\n", c.ptrOffset, ptr)
			continue
		}
		addr := ptr + c.structOffset
		cb := readMem(h, addr, 12)
		if len(cb) < 12 {
			fmt.Printf("  [chain] ptrOff=0x%-8X  ptr=0x%X 目标不可读\n", c.ptrOffset, ptr)
			continue
		}
		x := math.Float32frombits(binary.LittleEndian.Uint32(cb[0:]))
		zs := math.Float32frombits(binary.LittleEndian.Uint32(cb[4:]))
		yn := math.Float32frombits(binary.LittleEndian.Uint32(cb[8:]))
		ok := ptkCoordOK(x, zs, yn)
		fmt.Printf("  [chain] ptrOff=0x%-8X  ptr=0x%X  coords=(%.1f, %.1f, %.1f) hud=(%.1f, %.1f, %.1f) valid=%v\n",
			c.ptrOffset, ptr, x, zs, yn, x, -yn, zs-elevBias, ok)
	}

	fmt.Println("\n  === 模块收缩平移假设（旧模块 0x472b000 → 本机模块 0x3adf000）===")
	const oldMainSize = uintptr(0x472b000)
	const curMainSize = uintptr(0x3adf000)
	if oldMainSize > curMainSize {
		delta := oldMainSize - curMainSize
		fmt.Printf("  [shift] delta=0x%X，逐个平移旧偏移验证：\n", delta)
		for _, c := range knownStaticChains {
			if c.ptrOffset < delta {
				continue
			}
			off := c.ptrOffset - delta
			pb := readMem(h, mb+off, 8)
			if len(pb) < 8 {
				fmt.Printf("  [shift] 旧 0x%-8X → 新 0x%-8X  指针不可读\n", c.ptrOffset, off)
				continue
			}
			ptr := uintptr(binary.LittleEndian.Uint64(pb))
			if ptr < 0x100000000 {
				fmt.Printf("  [shift] 旧 0x%-8X → 新 0x%-8X  ptr=0x%X（空/非法）\n", c.ptrOffset, off, ptr)
				continue
			}
			cb := readMem(h, ptr+c.structOffset, 12)
			if len(cb) < 12 {
				fmt.Printf("  [shift] 旧 0x%-8X → 新 0x%-8X  ptr=0x%X 目标不可读\n", c.ptrOffset, off, ptr)
				continue
			}
			x := math.Float32frombits(binary.LittleEndian.Uint32(cb[0:]))
			zs := math.Float32frombits(binary.LittleEndian.Uint32(cb[4:]))
			yn := math.Float32frombits(binary.LittleEndian.Uint32(cb[8:]))
			fmt.Printf("  [shift] 旧 0x%-8X → 新 0x%-8X  ptr=0x%X  hud=(%.1f, %.1f, %.1f) valid=%v\n",
				c.ptrOffset, off, ptr, x, -yn, zs-elevBias, ptkCoordOK(x, zs, yn))
		}
	}

	// 2. 全范围结构槽扫描（Run>=2），可选按锚点距离排序
	blks := p.Blocks(0.0625)
	fmt.Printf("\n  === 全范围结构槽扫描（%d 块）===\n", len(blks))
	hits := ptkScanNear(h, blks, nil, 0)
	var cand []ptkHit
	for _, x := range hits {
		if x.Run >= 2 {
			cand = append(cand, x)
		}
	}
	groups := ptkGroupHits(cand)
	if anchor != nil {
		for i := range groups {
			groups[i].Dist = dist3(groups[i].P, *anchor)
		}
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].Dist < groups[j].Dist })
	}
	show := groups
	if len(show) > probeMaxShow {
		show = show[:probeMaxShow]
	}
	for i, g := range show {
		fmt.Printf("  #%-2d addr=0x%-12X Run=%-2d copies=%-2d dist=%-7.1fm hud=(%.1f, %.1f, %.1f)\n",
			i+1, g.Addr, g.Run, g.Copies, g.Dist, g.P[0], -g.P[2], g.P[1]-elevBias)
	}
	if len(groups) == 0 {
		fmt.Println("  [probe] 没有结构槽（可能不在 3D 世界 / 镜头遮挡 / 结构已变）")
	}

	// 3. 在主模块 / exe 数据段里反查指向玩家槽的全局指针。
	//    做法：取最近的槽作为参考坐标，收集所有指向该槽附近（±2MB）的堆地址的 qword，
	//    再把目标附近 0x4000 字节读出来，找与参考坐标吻合的 12 字节三元组，
	//    从而同时得到 (指针偏移, 结构内坐标偏移)，不依赖结构布局假设。
	fmt.Println("\n  === 全局指针反查（主模块 + exe 数据段）===")
	var ref [3]float32
	haveRef := false
	if len(groups) > 0 && anchor != nil {
		ref = groups[0].P
		haveRef = true
	} else if anchor != nil {
		ref = *anchor
		haveRef = true
	}
	if !haveRef || len(groups) == 0 {
		fmt.Println("  [probe] 缺少参考坐标或没有结构槽，跳过指针反查（可用 --coords 指定游戏内坐标）")
		return
	}
	ringAddr := groups[0].Addr
	const win = uintptr(0x200000)
	const structSpan = 0x4000
	lo, hi := ringAddr-win, ringAddr+win

	type coordHit struct {
		PtrOff    uintptr // 指针所在的模块内偏移
		StructOff uintptr // 坐标相对指针目标的偏移
		Val       uintptr // 指针值
		Pos       [3]float32
		Dist      float32
	}
	var found []coordHit

	searchBlocks := func(blocks []MemBlock, offBase uintptr) {
		for _, b := range blocks {
			for off := uintptr(0); off < b.Size; off += probeReadChunk {
				n := uintptr(probeReadChunk)
				if b.Size-off < n {
					n = b.Size - off
				}
				buf := readMem(h, b.Base+off, int(n))
				for i := 0; i+8 <= len(buf); i += 8 {
					v := uintptr(binary.LittleEndian.Uint64(buf[i:]))
					if v < lo || v > hi || v < 0x100000000 {
						continue
					}
					tb := readMem(h, v, structSpan)
					if len(tb) < 12 {
						continue
					}
					for k := 0; k+12 <= len(tb); k += 4 {
						x := math.Float32frombits(binary.LittleEndian.Uint32(tb[k:]))
						zs := math.Float32frombits(binary.LittleEndian.Uint32(tb[k+4:]))
						yn := math.Float32frombits(binary.LittleEndian.Uint32(tb[k+8:]))
						if !ptkCoordOK(x, zs, yn) {
							continue
						}
						p := [3]float32{x, zs, yn}
						if d := dist3(p, ref); d < 5.0 {
							found = append(found, coordHit{
								PtrOff:    b.Base + off + uintptr(i) - offBase,
								StructOff: uintptr(k),
								Val:       v,
								Pos:       p,
								Dist:      d,
							})
						}
					}
				}
			}
		}
	}

	var mainBlks, exeBlks []MemBlock
	eb, es := plat.ExeBase()
	for _, b := range blks {
		if b.Base >= mb && b.Base < mb+0x10000000 {
			mainBlks = append(mainBlks, b)
		} else if eb != 0 && b.Base >= eb && b.Base < eb+es {
			exeBlks = append(exeBlks, b)
		}
	}
	searchBlocks(mainBlks, mb)
	searchBlocks(exeBlks, eb)

	fmt.Printf("  [probe] 参考坐标 hud=(%.1f, %.1f, %.1f)，命中指针 %d 个\n", ref[0], -ref[2], ref[1]-elevBias, len(found))
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Dist != found[j].Dist {
			return found[i].Dist < found[j].Dist
		}
		return found[i].StructOff < found[j].StructOff
	})
	n := len(found)
	if n > 40 {
		n = 40
	}
	for i := 0; i < n; i++ {
		f := found[i]
		fmt.Printf("  #%-2d ptrOff=0x%-8X structOff=0x%-5X ptr=0x%-12X pos=(%.1f, %.1f, %.1f) d=%.1fm\n",
			i+1, f.PtrOff, f.StructOff, f.Val, f.Pos[0], -f.Pos[2], f.Pos[1]-elevBias, f.Dist)
	}
	if len(found) == 0 {
		fmt.Println("  [probe] 未找到指向玩家槽的全局指针")
		return
	}

	// 4. 透视玩家 Actor 结构：列出前 0x800 内所有合法坐标三元组，
	//    便于判断"当前坐标"字段与历史轨迹环各自的位置。
	base := found[0].Val
	fmt.Printf("\n  === 玩家结构透视 ptr=0x%X（前 0x800）===\n", base)
	db := readMem(h, base, 0x800)
	fmt.Printf("  [dump] 读到 %d 字节\n", len(db))
	for k := 0; k+12 <= len(db); k += 4 {
		x := math.Float32frombits(binary.LittleEndian.Uint32(db[k:]))
		zs := math.Float32frombits(binary.LittleEndian.Uint32(db[k+4:]))
		yn := math.Float32frombits(binary.LittleEndian.Uint32(db[k+8:]))
		if !ptkCoordOK(x, zs, yn) {
			continue
		}
		p := [3]float32{x, zs, yn}
		d := dist3(p, ref)
		if d < 200 {
			fmt.Printf("  off=+0x%-4X hud=(%.1f, %.1f, %.1f) d=%.1fm\n", k, x, -yn, zs-elevBias, d)
		}
	}
}
