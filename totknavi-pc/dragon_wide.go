// dragon_wide.go — 龙坐标宽扫描（--dragons-wide）。
//
// 与 dragon_probe.go 的区别：不要求 0x48/Quat 结构门禁，直接按坐标合法性 +
// 轨道距离 + 两次差分找"以 5m/s 沿轨道移动"的坐标。用于验证龙实体是否以
// 其他结构存放在内存中。
package main

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type railGridPoint struct {
	X, Y, Z float64
	Rail    int
}

type railGrid struct {
	cell  float64
	cells map[[2]int][]railGridPoint
}

func buildRailGrid(rails []dragonRail, cell float64) *railGrid {
	g := &railGrid{cell: cell, cells: make(map[[2]int][]railGridPoint)}
	for ri := range rails {
		for _, p := range rails[ri].Points {
			k := [2]int{int(math.Floor(p.X / cell)), int(math.Floor(p.Y / cell))}
			g.cells[k] = append(g.cells[k], railGridPoint{X: p.X, Y: p.Y, Z: p.Z, Rail: ri})
		}
	}
	return g
}

func (g *railGrid) nearest(x, y, z float64) (railGridPoint, float64) {
	cx := int(math.Floor(x / g.cell))
	cy := int(math.Floor(y / g.cell))
	best := math.MaxFloat64
	var bp railGridPoint
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for _, p := range g.cells[[2]int{cx + dx, cy + dy}] {
				ddx := x - p.X
				ddy := y - p.Y
				ddz := z - p.Z
				d := ddx*ddx + ddy*ddy + ddz*ddz
				if d < best {
					best, bp = d, p
				}
			}
		}
	}
	return bp, math.Sqrt(best)
}

type wideHit struct {
	Addr uintptr
	X, Y, Z float64
	Rail int
	Dist float64
}

func scanWideCoords(h uintptr, blks []MemBlock, grid *railGrid, maxDist float64) map[uintptr]wideHit {
	out := make(map[uintptr]wideHit, 4096)
	const chunk = 8 << 20
	for _, b := range blks {
		for off := uintptr(0); off < b.Size; off += chunk {
			n := int(chunk)
			if remain := int(b.Size - off); remain < n {
				n = remain
			}
			buf := readMem(h, b.Base+off, n)
			for i := 0; i+12 <= len(buf); i += 4 {
				x := math.Float32frombits(uint32(buf[i]) | uint32(buf[i+1])<<8 | uint32(buf[i+2])<<16 | uint32(buf[i+3])<<24)
				zs := math.Float32frombits(uint32(buf[i+4]) | uint32(buf[i+5])<<8 | uint32(buf[i+6])<<16 | uint32(buf[i+7])<<24)
				yn := math.Float32frombits(uint32(buf[i+8]) | uint32(buf[i+9])<<8 | uint32(buf[i+10])<<16 | uint32(buf[i+11])<<24)
				if !ptkCoordOK(x, zs, yn) {
					continue
				}
				z := float64(zs) - elevBias
				if z < -700 || z > 2500 {
					continue
				}
				p, d := grid.nearest(float64(x), float64(yn), z)
				if d > maxDist {
					continue
				}
				out[b.Base+off+uintptr(i)] = wideHit{
					Addr: b.Base + off + uintptr(i),
					X: float64(x), Y: float64(yn), Z: z,
					Rail: p.Rail, Dist: d,
				}
			}
		}
	}
	return out
}

// scanDragonStrings 在已扫内存块里找龙 actor 名称，确认资源是否加载。
func scanDragonStrings(p Platform, blks []MemBlock) {
	names := []string{"Enemy_Dragon_Fire", "Enemy_Dragon_Ice", "Enemy_Dragon_Electric", "Enemy_Dragon_Light"}
	const chunk = 8 << 20
	found := map[string]int{}
	for _, b := range blks {
		for off := uintptr(0); off < b.Size; off += chunk {
			n := int(chunk)
			if remain := int(b.Size - off); remain < n {
				n = remain
			}
			buf := readMem(p.Handle(), b.Base+off, n)
			for _, name := range names {
				nb := []byte(name)
				for pos := 0; pos < len(buf); {
					idx := bytes.Index(buf[pos:], nb)
					if idx < 0 {
						break
					}
					addr := b.Base + off + uintptr(pos+idx)
					if found[name] < 20 {
						fmt.Printf("  [str] %-22s @ 0x%X\n", name, addr)
					}
					found[name]++
					pos += idx + 1
				}
			}
		}
	}
	fmt.Printf("  [str] 命中统计：")
	for _, n := range names {
		fmt.Printf("%s=%d  ", n, found[n])
	}
	fmt.Println()
}

func parseAddrList(s string) []uintptr {
	var out []uintptr
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(strings.TrimPrefix(part, "0x"), "0X")
		if v, err := strconv.ParseUint(part, 16, 64); err == nil {
			out = append(out, uintptr(v))
		}
	}
	return out
}

// runDragonTrack 锁定地址连续采样，打印每拍速度与坐标。
func runDragonTrack(p Platform, addrs []uintptr, interval time.Duration, rounds int) {
	if len(addrs) == 0 {
		fmt.Println("  [!] 没有有效地址")
		return
	}
	fmt.Printf("  [track] %d 个地址，%v 间隔 ×%d\n", len(addrs), interval, rounds)
	prev := map[uintptr][3]float64{}
	prevT := map[uintptr]time.Time{}
	for r := 0; r < rounds; r++ {
		if r > 0 {
			time.Sleep(interval)
		}
		now := time.Now()
		for _, addr := range addrs {
			buf := readMem(p.Handle(), addr, 12)
			if len(buf) < 12 {
				continue
			}
			x := math.Float32frombits(uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24)
			zs := math.Float32frombits(uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24)
			yn := math.Float32frombits(uint32(buf[8]) | uint32(buf[9])<<8 | uint32(buf[10])<<16 | uint32(buf[11])<<24)
			if !ptkCoordOK(x, zs, yn) {
				continue
			}
			w := [3]float64{float64(x), float64(yn), float64(zs) - elevBias}
			if t, ok := prevT[addr]; ok {
				dt := now.Sub(t).Seconds()
				dx, dy, dz := w[0]-prev[addr][0], w[1]-prev[addr][1], w[2]-prev[addr][2]
				d := math.Sqrt(dx*dx + dy*dy + dz*dz)
				fmt.Printf("  %s 0x%-12X speed=%5.2f m/s  pos=(%.1f, %.1f, %.1f)  d=(%.1f, %.1f, %.1f)\n",
					now.Format("15:04:05.000"), addr, d/dt, w[0], w[1], w[2], dx, dy, dz)
			} else {
				fmt.Printf("  %s 0x%-12X 初始  pos=(%.1f, %.1f, %.1f)\n",
					now.Format("15:04:05.000"), addr, w[0], w[1], w[2])
			}
			prev[addr] = w
			prevT[addr] = now
		}
	}
}

// runDragonWideProbe --dragons-wide 主流程。
func runDragonWideProbe(p Platform, appRoot string) {
	rails, path, err := loadDragonRails(appRoot)
	if err != nil {
		fmt.Printf("  [!] %v\n", err)
		return
	}
	fmt.Printf("  [ok] 轨道数据：%s（%d 条）\n", path, len(rails))

	// 先确认龙资源是否在内存里（actor 名称字符串）。
	preBlks := p.Blocks(0.0625)
	scanDragonStrings(p, preBlks)

	grid := buildRailGrid(rails, 100)

	blks := p.Blocks(0.0625)
	total := uintptr(0)
	for _, b := range blks {
		total += b.Size
	}
	fmt.Printf("  [wide] 宽扫描 %d 块 / %.1fMB，距轨道 <50m 的坐标…\n", len(blks), float64(total)/1048576.0)

	t0 := time.Now()
	first := scanWideCoords(p.Handle(), blks, grid, 100)
	fmt.Printf("  [wide] 第一次完成 %.1fs：%d 个坐标\n", time.Since(t0).Seconds(), len(first))

	const gap = 3 * time.Second
	fmt.Printf("  [wide] 等待 %.0fs 后第二次扫描…\n", gap.Seconds())
	time.Sleep(gap)
	t1 := time.Now()
	second := scanWideCoords(p.Handle(), blks, grid, 100)
	fmt.Printf("  [wide] 第二次完成 %.1fs：%d 个坐标\n", time.Since(t1).Seconds(), len(second))

	type moved struct {
		h     wideHit
		delta float64
	}
	var movedList []moved
	for addr, a := range first {
		b, ok := second[addr]
		if !ok {
			continue
		}
		dx := b.X - a.X
		dy := b.Y - a.Y
		dz := b.Z - a.Z
		d := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if d < 4 || d > 60 {
			continue
		}
		movedList = append(movedList, moved{h: b, delta: d})
	}
	sort.Slice(movedList, func(i, j int) bool { return movedList[i].h.Dist < movedList[j].h.Dist })

	fmt.Printf("\n  === 移动坐标（4~60m/%.0fs）：%d 个 ===\n", gap.Seconds(), len(movedList))
	n := len(movedList)
	if n > 40 {
		n = 40
	}
	for i := 0; i < n; i++ {
		m := movedList[i]
		fmt.Printf("  #%-2d %-6s addr=0x%-12X speed=%5.2f m/s d=%5.1fm pos=(%.1f, %.1f, %.1f)\n",
			i+1, rails[m.h.Rail].Name, m.h.Addr, m.delta/gap.Seconds(), m.h.Dist, m.h.X, m.h.Y, m.h.Z)
	}

	// 每条龙最佳移动候选
	best := make([]*moved, len(rails))
	for i := range movedList {
		m := &movedList[i]
		if best[m.h.Rail] == nil || m.h.Dist < best[m.h.Rail].h.Dist {
			best[m.h.Rail] = m
		}
	}
	fmt.Println("\n  === 每条龙的最佳移动候选 ===")
	for i, r := range rails {
		if best[i] == nil {
			fmt.Printf("  %-6s 未找到移动坐标\n", r.Name)
			continue
		}
		m := best[i]
		fmt.Printf("  %-6s addr=0x%-12X speed=%.2f m/s d=%.1fm pos=(%.1f, %.1f, %.1f)\n",
			r.Name, m.h.Addr, m.delta/gap.Seconds(), m.h.Dist, m.h.X, m.h.Y, m.h.Z)
	}

	// 锁定采样：对移动候选按 2s 间隔直读 3 轮，验证真实速度是否 ≈5 m/s。
	fmt.Println("\n  === 锁定采样（2s 间隔 ×3，期望 ≈5 m/s）===")
	type snap struct {
		t    time.Time
		x, y, z float64
		ok   bool
	}
	prev := map[uintptr]snap{}
	limit := len(movedList)
	if limit > 6 {
		limit = 6
	}
	for round := 0; round < 3; round++ {
		if round > 0 {
			time.Sleep(2 * time.Second)
		}
		now := time.Now()
		for k := 0; k < limit; k++ {
			m := movedList[k]
			buf := readMem(p.Handle(), m.h.Addr, 12)
			if len(buf) < 12 {
				continue
			}
			x := math.Float32frombits(uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24)
			zs := math.Float32frombits(uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24)
			yn := math.Float32frombits(uint32(buf[8]) | uint32(buf[9])<<8 | uint32(buf[10])<<16 | uint32(buf[11])<<24)
			if !ptkCoordOK(x, zs, yn) {
				continue
			}
			wx, wy, wz := float64(x), float64(yn), float64(zs)-elevBias
			if pv, ok := prev[m.h.Addr]; ok && pv.ok {
				dt := now.Sub(pv.t).Seconds()
				dx := wx - pv.x
				dy := wy - pv.y
				dz := wz - pv.z
				d := math.Sqrt(dx*dx + dy*dy + dz*dz)
				fmt.Printf("  r%d %-6s addr=0x%-12X speed=%5.2f m/s pos=(%.1f, %.1f, %.1f)\n",
					round+1, rails[m.h.Rail].Name, m.h.Addr, d/dt, wx, wy, wz)
			}
			prev[m.h.Addr] = snap{t: now, x: wx, y: wy, z: wz, ok: true}
		}
	}
}
