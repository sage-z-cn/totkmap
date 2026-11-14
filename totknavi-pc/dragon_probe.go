// dragon_probe.go — 龙坐标探针（--dragons）。
//
// 目的：验证四条龙在 PC 重编译版内存中能否被现有扫描原语识别。
// 方法：全内存扫描候选槽（Run>=2），用 ROM 官方轨道（app/data/dragon_paths_rom.js）
// 做最近 3D 距离匹配，打印每条龙的最佳候选，并二次采样验证位移是否约 5 m/s。
//
// 只读内存，不注入、不修改游戏。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type dragonRailPoint struct{ X, Y, Z float64 }

type dragonRail struct {
	Id       string
	Name     string
	Layer    int
	Points   []dragonRailPoint
	Cum      []float64
	TotalLen float64
}

type dragonItemJSON struct {
	Id       string       `json:"id"`
	Name     string       `json:"name"`
	Layer    int          `json:"layer"`
	Segments [][2]float64 `json:"segments"`
	Altitude []float64    `json:"altitude"`
}

type dragonFileJSON struct {
	Items []dragonItemJSON `json:"items"`
}

// loadDragonRails 读 app/data/dragon_paths_rom.js，返回四条龙的 3D 采样点。
func loadDragonRails(appRoot string) ([]dragonRail, string, error) {
	var candidates []string
	if appRoot != "" {
		candidates = append(candidates, filepath.Join(appRoot, "data", "dragon_paths_rom.js"))
	}
	if d := findAppDir(""); d != "" {
		candidates = append(candidates, filepath.Join(d, "data", "dragon_paths_rom.js"))
	}
	candidates = append(candidates,
		filepath.Join("app", "data", "dragon_paths_rom.js"),
		filepath.Join("..", "app", "data", "dragon_paths_rom.js"),
	)

	var raw []byte
	var path string
	for _, c := range candidates {
		if b, err := os.ReadFile(c); err == nil {
			raw, path = b, c
			break
		}
	}
	if path == "" {
		return nil, "", fmt.Errorf("找不到 app/data/dragon_paths_rom.js（可用 --app 指定 app 目录）")
	}

	s := string(raw)
	i := strings.Index(s, "window.TOTK_DRAGON_PATHS_ROM")
	if i < 0 {
		return nil, path, fmt.Errorf("数据格式异常：未找到 window.TOTK_DRAGON_PATHS_ROM")
	}
	j := strings.Index(s[i:], "{")
	if j < 0 {
		return nil, path, fmt.Errorf("数据格式异常：未找到 JSON 起点")
	}
	var d dragonFileJSON
	dec := json.NewDecoder(bytes.NewReader([]byte(s[i+j:])))
	if err := dec.Decode(&d); err != nil {
		return nil, path, fmt.Errorf("JSON 解析失败: %w", err)
	}

	var rails []dragonRail
	for _, it := range d.Items {
		n := len(it.Segments)
		if len(it.Altitude) < n {
			n = len(it.Altitude)
		}
		if n == 0 {
			continue
		}
		r := dragonRail{Id: it.Id, Name: it.Name, Layer: it.Layer}
		for k := 0; k < n; k++ {
			r.Points = append(r.Points, dragonRailPoint{
				X: it.Segments[k][0], // X 东
				Y: it.Segments[k][1], // Z 北（= 内存 Y_north）
				Z: it.Altitude[k],    // 真实飞行高度（米，负=地下）
			})
		}
		// 累积弧长（用于按 5m/s 沿轨道推算）。
		r.Cum = make([]float64, len(r.Points))
		for k := 1; k < len(r.Points); k++ {
			dx := r.Points[k].X - r.Points[k-1].X
			dy := r.Points[k].Y - r.Points[k-1].Y
			dz := r.Points[k].Z - r.Points[k-1].Z
			r.Cum[k] = r.Cum[k-1] + math.Sqrt(dx*dx+dy*dy+dz*dz)
		}
		if len(r.Cum) > 0 {
			r.TotalLen = r.Cum[len(r.Cum)-1]
		}
		rails = append(rails, r)
	}
	if len(rails) == 0 {
		return nil, path, fmt.Errorf("轨道数据为空")
	}
	return rails, path, nil
}

type dragonCandidate struct {
	Addr  uintptr
	Rail  int
	Index int
	Dist  float64
	World [3]float64 // X, Z北, 高度
	Run   int
	Copies int
}

// nearestDragonRail 返回 (轨道下标, 采样点下标, 3D 距离)。
func nearestDragonRail(rails []dragonRail, x, y, z float64) (int, int, float64) {
	bestRail, bestIdx := -1, -1
	bestD := math.MaxFloat64
	for i := range rails {
		pts := rails[i].Points
		for j := range pts {
			dx := x - pts[j].X
			dy := y - pts[j].Y
			dz := z - pts[j].Z
			d := dx*dx + dy*dy + dz*dz
			if d < bestD {
				bestD, bestRail, bestIdx = d, i, j
			}
		}
	}
	return bestRail, bestIdx, math.Sqrt(bestD)
}

// runDragonProbe --dragons 主流程。
func runDragonProbe(p Platform, appRoot string) {
	rails, path, err := loadDragonRails(appRoot)
	if err != nil {
		fmt.Printf("  [!] %v\n", err)
		return
	}
	fmt.Printf("  [ok] 轨道数据：%s\n", path)
	for _, r := range rails {
		fmt.Printf("       %-8s %-6s %d 点\n", r.Id, r.Name, len(r.Points))
	}

	blks := p.Blocks(0.0625)
	total := uintptr(0)
	for _, b := range blks {
		total += b.Size
	}
	fmt.Printf("  [scan] %d 块 / %.1fMB，全范围扫描中…\n", len(blks), float64(total)/1048576.0)

	t0 := time.Now()
	hits := ptkScanNear(p.Handle(), blks, nil, 0)
	var ident []ptkHit
	for _, h := range hits {
		if h.Run >= 2 {
			ident = append(ident, h)
		}
	}
	fmt.Printf("  [scan] 完成 %.2fs：hits=%d，Run>=2=%d\n", time.Since(t0).Seconds(), len(hits), len(ident))
	if len(ident) == 0 {
		fmt.Println("  [!] 没有可用候选，确认游戏已进入 3D 画面。")
		return
	}

	var cands []dragonCandidate
	for _, h := range ident {
		x := float64(h.P[0])
		y := float64(h.P[2]) // Z 北
		z := float64(h.P[1]) - elevBias
		ri, idx, d := nearestDragonRail(rails, x, y, z)
		if d > 300 {
			continue
		}
		cands = append(cands, dragonCandidate{
			Addr: h.Addr, Rail: ri, Index: idx, Dist: d,
			World: [3]float64{x, y, z}, Run: h.Run, Copies: h.Copies,
		})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Dist < cands[j].Dist })

	fmt.Printf("\n  [match] 距轨道 <300m 的候选：%d 个\n", len(cands))
	n := len(cands)
	if n > 20 {
		n = 20
	}
	for i := 0; i < n; i++ {
		c := cands[i]
		fmt.Printf("  #%-2d %-6s addr=0x%-12X d=%6.1fm run=%-3d copies=%-3d world=(%.1f, %.1f, %.1f)\n",
			i+1, rails[c.Rail].Name, c.Addr, c.Dist, c.Run, c.Copies, c.World[0], c.World[1], c.World[2])
	}

	// 每条龙取最佳候选
	best := make([]*dragonCandidate, len(rails))
	for i := range cands {
		c := &cands[i]
		if best[c.Rail] == nil || c.Dist < best[c.Rail].Dist {
			best[c.Rail] = c
		}
	}
	fmt.Println("\n  === 每条龙的最佳候选 ===")
	var picked []*dragonCandidate
	for i, r := range rails {
		if best[i] == nil {
			fmt.Printf("  %-6s 未匹配到（可能在高空轨/未加载区域）\n", r.Name)
			continue
		}
		c := best[i]
		fmt.Printf("  %-6s addr=0x%-12X d=%.1fm world=(%.1f, %.1f, %.1f)\n",
			r.Name, c.Addr, c.Dist, c.World[0], c.World[1], c.World[2])
		picked = append(picked, c)
	}

	if len(picked) == 0 {
		return
	}

	// 两次全量扫描差分：只有正在飞的龙才会在 3 秒内位移约 15m（5 m/s）。
	const gap = 3 * time.Second
	fmt.Printf("\n  [verify] %.0fs 后第二次全量扫描…\n", gap.Seconds())
	time.Sleep(gap)
	t1 := time.Now()
	hits2 := ptkScanNear(p.Handle(), blks, nil, 0)
	fmt.Printf("  [verify] 完成 %.2fs：hits=%d\n", time.Since(t1).Seconds(), len(hits2))

	prev := make(map[uintptr][3]float32, len(ident))
	for _, h := range ident {
		prev[h.Addr] = h.P
	}
	type movedCand struct {
		dragonCandidate
		delta float64
	}
	var moved []movedCand
	for _, h := range hits2 {
		old, ok := prev[h.Addr]
		if !ok {
			continue
		}
		dx := float64(h.P[0] - old[0])
		dy := float64(h.P[2] - old[2])
		dz := float64(h.P[1] - old[1])
		delta := math.Sqrt(dx*dx + dy*dy + dz*dz)
		// 3 秒内：静止 <3m，龙约 15m；放宽到 4~60m
		if delta < 4 || delta > 60 {
			continue
		}
		x := float64(h.P[0])
		y := float64(h.P[2])
		z := float64(h.P[1]) - elevBias
		ri, idx, rd := nearestDragonRail(rails, x, y, z)
		if rd > 300 {
			continue
		}
		moved = append(moved, movedCand{
			dragonCandidate: dragonCandidate{Addr: h.Addr, Rail: ri, Index: idx, Dist: rd,
				World: [3]float64{x, y, z}, Run: h.Run, Copies: h.Copies},
			delta: delta,
		})
	}
	sort.Slice(moved, func(i, j int) bool { return moved[i].Dist < moved[j].Dist })

	fmt.Printf("\n  === 移动候选（4~60m/%.0fs，且距轨道 <300m）：%d 个 ===\n", gap.Seconds(), len(moved))
	mn := len(moved)
	if mn > 30 {
		mn = 30
	}
	for i := 0; i < mn; i++ {
		c := moved[i]
		fmt.Printf("  #%-2d %-6s addr=0x%-12X speed=%5.2f m/s d=%6.1fm world=(%.1f, %.1f, %.1f)\n",
			i+1, rails[c.Rail].Name, c.Addr, c.delta/gap.Seconds(), c.Dist, c.World[0], c.World[1], c.World[2])
	}
	if len(moved) == 0 {
		fmt.Println("  [!] 没有移动候选。龙可能不在已扫内存块内，或坐标结构不是 0x48/Quat 槽。")
	}
}
