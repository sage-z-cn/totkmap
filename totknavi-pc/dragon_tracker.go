// dragon_tracker.go — 龙的实时坐标追踪（内存直读 + 轨道匹配）。
//
// 机制（2026-11-14 实机验证）：
//   - 龙在玩家附近时，PC 重编译版会把坐标以 5.00 m/s 沿轨道更新；
//   - 龙在远处时不模拟，内存里只有静态缓存；
//   - 因此启动时用两次宽扫描（全用户空间）找"2 秒内位移约 10m 且贴近轨道"的
//     坐标锁定为龙地址，之后 200ms 直读；坐标静止时标记 Simulated=false。
//
// 坐标口径与 /pos 一致：gx=X 东、gy=-Y北、gz=真实高度；
// mx,my = (-gy, gx) = (Y北, X东)，即 Leaflet latlng。
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type dragonLive struct {
	Id        string  `json:"id"`
	Name      string  `json:"name"`
	Gx        float64 `json:"gx"`
	Gy        float64 `json:"gy"`
	Gz        float64 `json:"gz"`
	Mx        float64 `json:"mx"`
	My        float64 `json:"my"`
	Layer     int     `json:"layer"`
	Simulated bool    `json:"simulated"`
	Locked    bool    `json:"locked"`
	Updated   int64   `json:"updated"`
}

type dragonTracker struct {
	mu       sync.RWMutex
	rails    []dragonRail
	live     []dragonLive
	addrs    []uintptr
	last     [][3]float64
	lastT    []time.Time
	railT    []float64
	lastGood []time.Time
	scanning bool
}

var dragonTrk *dragonTracker

// dragonBlocks 返回全用户空间已提交可读块（龙坐标不一定在主堆 1GB 范围内）。
func dragonBlocks(p Platform) []MemBlock {
	if plat, ok := p.(*totkpcPlatform); ok {
		return plat.blocksAll(0.0625)
	}
	return p.Blocks(0.0625)
}

func startDragonTracker(p Platform, appRoot string, fixedBase uintptr) {
	rails, path, err := loadDragonRails(appRoot)
	if err != nil {
		fmt.Printf("  [dragon] 轨道数据加载失败：%v\n", err)
		return
	}
	fmt.Printf("  [dragon] 轨道数据：%s\n", path)
	t := &dragonTracker{
		rails:    rails,
		live:     make([]dragonLive, len(rails)),
		addrs:    make([]uintptr, len(rails)),
		last:     make([][3]float64, len(rails)),
		lastT:    make([]time.Time, len(rails)),
		railT:    make([]float64, len(rails)),
		lastGood: make([]time.Time, len(rails)),
	}
	for i, r := range rails {
		t.live[i] = dragonLive{Id: r.Id, Name: r.Name, Layer: r.Layer}
	}
	dragonTrk = t
	if fixedBase == 0 {
		fixedBase = readDragonBase()
	}
	if fixedBase != 0 {
		applyDragonBase(t, fixedBase)
		fmt.Printf("  [dragon] 使用缓存的数组基址 0x%X\n", fixedBase)
	}
	go t.readLoop(p)
	go func() {
		t.scanOnce(p)
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			t.scanOnce(p)
		}
	}()
}

func dragonBasePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "dragon_base.txt"
	}
	return filepath.Join(filepath.Dir(exe), "dragon_base.txt")
}

func readDragonBase() uintptr {
	b, err := os.ReadFile(dragonBasePath())
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(b))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return uintptr(v)
}

func writeDragonBase(base uintptr) {
	_ = os.WriteFile(dragonBasePath(), []byte(fmt.Sprintf("0x%X\n", base)), 0o644)
}

func applyDragonBase(t *dragonTracker, base uintptr) {
	addrByID := map[string]uintptr{
		"farosh":  base,
		"dinraal": base + 0x80,
		"naydra":  base + 0x100,
		"light":   base + 0x180,
	}
	for i := range t.rails {
		if a, ok := addrByID[t.rails[i].Id]; ok {
			t.addrs[i] = a
			t.live[i].Locked = true
		}
	}
}

// dragonCoordsValid 检查基址处四个槽能否读出合法坐标（不检查速度，暂停时也成立）。
func dragonCoordsValid(p Platform, base uintptr) bool {
	if base == 0 {
		return false
	}
	okCount := 0
	for _, off := range []uintptr{0, 0x80, 0x100, 0x180} {
		buf := readMem(p.Handle(), base+off, 12)
		if len(buf) < 12 {
			continue
		}
		x := math.Float32frombits(uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24)
		zs := math.Float32frombits(uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24)
		yn := math.Float32frombits(uint32(buf[8]) | uint32(buf[9])<<8 | uint32(buf[10])<<16 | uint32(buf[11])<<24)
		if ptkCoordOK(x, zs, yn) {
			okCount++
		}
	}
	return okCount >= 3
}

// scanOnce 宽扫描两次，按"2 秒位移约 10m + 距轨道 <100m"锁定每条龙的地址。
func (t *dragonTracker) scanOnce(p Platform) {
	t.mu.Lock()
	if t.scanning {
		t.mu.Unlock()
		return
	}
	t.scanning = true
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.scanning = false
		t.mu.Unlock()
	}()

	blks := dragonBlocks(p)
	totalMB := 0.0
	for _, b := range blks {
		totalMB += float64(b.Size) / 1048576.0
	}
	fmt.Printf("  [dragon] 宽扫描开始：%d 块 / %.0fMB\n", len(blks), totalMB)
	grid := buildRailGrid(t.rails, 100)
	t0 := time.Now()
	first := scanWideCoords(p.Handle(), blks, grid, 600)
	fmt.Printf("  [dragon] 第一次完成 %d 个候选（%.1fs）\n", len(first), time.Since(t0).Seconds())
	time.Sleep(2 * time.Second)
	t1 := time.Now()
	second := scanWideCoords(p.Handle(), blks, grid, 600)
	fmt.Printf("  [dragon] 第二次完成 %d 个候选（%.1fs）\n", len(second), time.Since(t1).Seconds())
	fmt.Printf("  [dragon] 扫描完成 %.1fs：%d / %d 个候选\n",
		time.Since(t0).Seconds(), len(first), len(second))

	best := make([]float64, len(t.rails))
	for i := range best {
		best[i] = math.MaxFloat64
	}
	found := make([]uintptr, len(t.rails))
	moveSet := make(map[uintptr]bool)
	for addr, a := range first {
		b, ok := second[addr]
		if !ok {
			continue
		}
		dx, dy, dz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
		d := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if d < 30 || d > 60 { // 2 秒 × 5.00m/s ≈ 10m
			continue
		}
		moveSet[addr] = true
		// 兜底候选排除局部坐标系（X 接近 0 的缓存/特效坐标）。
		if math.Abs(b.X) < 200 {
			continue
		}
		ri := b.Rail
		rd := b.Dist
		if rd > 300 {
			continue
		}
		if rd < best[ri] {
			best[ri] = rd
			found[ri] = addr
		}
	}

	// 优先：四条龙在同一个 0x80 步长数组里。找连续四元组，按 ID 映射。
	var arrayBase uintptr
	for addr := range moveSet {
		if moveSet[addr+0x80] && moveSet[addr+0x100] && moveSet[addr+0x180] {
			arrayBase = addr
			break
		}
	}
	if arrayBase != 0 {
		if cached := readDragonBase(); cached != 0 && cached != arrayBase && dragonCoordsValid(p, cached) {
			fmt.Printf("  [dragon] 缓存基址 0x%X 仍有效，忽略新数组 0x%X\n", cached, arrayBase)
			return
		}
		t.mu.Lock()
		applyDragonBase(t, arrayBase)
		t.mu.Unlock()
		writeDragonBase(arrayBase)
		fmt.Printf("  [dragon] 数组基址 0x%X（0x80 步长，四条已锁定，已写缓存）\n", arrayBase)
		return
	}

	t.mu.Lock()
	for i := range found {
		if found[i] == 0 {
			continue
		}
		// 锁定前验证该地址能读出合法坐标（宽扫描的候选可能随后失效）。
		buf := readMem(p.Handle(), found[i], 12)
		if len(buf) < 12 {
			continue
		}
		x := math.Float32frombits(uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24)
		zs := math.Float32frombits(uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24)
		yn := math.Float32frombits(uint32(buf[8]) | uint32(buf[9])<<8 | uint32(buf[10])<<16 | uint32(buf[11])<<24)
		if !ptkCoordOK(x, zs, yn) {
			continue
		}
		t.addrs[i] = found[i]
		t.live[i].Locked = true
		fmt.Printf("  [dragon] 锁定 %-6s addr=0x%X 距轨道 %.1fm\n", t.rails[i].Name, found[i], best[i])
	}
	t.mu.Unlock()
}

// readLoop 200ms 直读已锁定的龙地址，更新坐标与模拟状态。
func (t *dragonTracker) readLoop(p Platform) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for now := range ticker.C {
		t.mu.Lock()
		for i, addr := range t.addrs {
			if addr == 0 {
				continue
			}
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
			wx, wy, wz := float64(x), float64(yn), float64(zs)-elevBias
			if !t.lastT[i].IsZero() {
				dt := now.Sub(t.lastT[i]).Seconds()
				if dt > 0.05 {
					dx, dy, dz := wx-t.last[i][0], wy-t.last[i][1], wz-t.last[i][2]
					speed := math.Sqrt(dx*dx+dy*dy+dz*dz) / dt
					if speed > 2.5 && speed < 7.5 {
						t.live[i].Simulated = true
						// 记录轨道弧长与最后有效时刻，地址失效后据此按 5m/s 推算。
						if s, d := nearestRailT(&t.rails[i], wx, wy, wz); d < 500 {
							t.railT[i] = s
							t.lastGood[i] = now
						}
					} else if speed < 0.5 {
						t.live[i].Simulated = false
					}
				}
			}
			t.last[i] = [3]float64{wx, wy, wz}
			t.lastT[i] = now
			t.live[i].Gx = wx
			t.live[i].Gy = -wy
			t.live[i].Gz = wz
			t.live[i].Mx = wy
			t.live[i].My = wx
			t.live[i].Layer = t.layerOf(i, wz)
			t.live[i].Locked = true
			t.live[i].Updated = now.UnixMilli()
		}
		t.mu.Unlock()
	}
}

func (t *dragonTracker) layerOf(i int, gz float64) int {
	if t.rails[i].Layer == 20 {
		return 20
	}
	if gz < -80 {
		return 19
	}
	return 18
}

// handleDragonsPC GET /dragons — 四条龙的当前坐标。
func handleDragonsPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	t := dragonTrk
	if t == nil {
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "dragon tracker not started"})
		return
	}
	t.mu.RLock()
	items := make([]dragonLive, len(t.live))
	copy(items, t.live)
	now := time.Now()
	for i := range items {
		if items[i].Locked && !items[i].Simulated && !t.lastGood[i].IsZero() {
			dt := now.Sub(t.lastGood[i]).Seconds()
			if dt > 1 && dt < 600 {
				x, y, z := railAt(&t.rails[i], t.railT[i]+5*dt)
				items[i].Gx = x
				items[i].Gy = -y
				items[i].Gz = z
				items[i].Mx = y
				items[i].My = x
				items[i].Layer = t.layerOf(i, z)
			}
		}
	}
	t.mu.RUnlock()
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": time.Now().UnixMilli(), "items": items})
}

// handleDragonsRescanPC POST/GET /dragons/rescan — 手动重扫（传送后地址变化时用）。
func handleDragonsRescanPC(w http.ResponseWriter, r *http.Request) {
	if dragonTrk != nil {
		if p := currentPlatform(); p != nil {
			go dragonTrk.scanOnce(p)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, `{"ok":true}`)
}

// railAt 按弧长 s 在轨道上插值（超出长度自动取模，闭合轨道）。
func railAt(r *dragonRail, s float64) (float64, float64, float64) {
	if len(r.Points) == 0 || r.TotalLen <= 0 {
		return 0, 0, 0
	}
	s = math.Mod(s, r.TotalLen)
	if s < 0 {
		s += r.TotalLen
	}
	i := sort.SearchFloat64s(r.Cum, s)
	if i <= 0 {
		i = 1
	}
	if i >= len(r.Cum) {
		i = len(r.Cum) - 1
	}
	a, b := r.Points[i-1], r.Points[i]
	seg := r.Cum[i] - r.Cum[i-1]
	f := 0.0
	if seg > 0 {
		f = (s - r.Cum[i-1]) / seg
	}
	return a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f, a.Z + (b.Z-a.Z)*f
}

// nearestRailT 返回坐标在轨道上的最近点弧长与距离。
func nearestRailT(r *dragonRail, x, y, z float64) (float64, float64) {
	best := math.MaxFloat64
	bi := 0
	for i := range r.Points {
		dx, dy, dz := x-r.Points[i].X, y-r.Points[i].Y, z-r.Points[i].Z
		d := dx*dx + dy*dy + dz*dz
		if d < best {
			best = d
			bi = i
		}
	}
	return r.Cum[bi], math.Sqrt(best)
}
