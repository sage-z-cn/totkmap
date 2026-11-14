// window_layout_test.go — 纯几何/选择逻辑的单测（不碰任何 Win32 调用）。
// 移动与还原依赖真实窗口，只能在真机上验；这里守住"选哪块屏"的规则。

package main

import "testing"

func mkMonitor(name string, primary bool, l, t, r, b int32) wlMonitor {
	m := wlMonitor{
		Name:    name,
		Primary: primary,
		Info: wlMonitorInfo{
			Monitor: wlRect{l, t, r, b},
			Work:    wlRect{l, t, r, b},
		},
	}
	if primary {
		m.Info.Flags = monitorInfoPrimary
	}
	return m
}

func TestRectContainsAndIntersects(t *testing.T) {
	r := wlRect{0, 0, 100, 100}
	if !r.contains(0, 0) || !r.contains(99, 99) {
		t.Fatal("左闭右开区间内应判定为包含")
	}
	if r.contains(100, 0) || r.contains(0, 100) || r.contains(-1, 0) {
		t.Fatal("边界/外侧不应判定为包含")
	}
	if !r.intersects(wlRect{50, 50, 150, 150}) {
		t.Fatal("部分重叠应判定相交")
	}
	if r.intersects(wlRect{100, 0, 200, 100}) || r.intersects(wlRect{-100, 0, 0, 100}) {
		t.Fatal("只贴边不重叠不应判定相交")
	}
}

func TestPickTargetNeedsTwoMonitors(t *testing.T) {
	one := []wlMonitor{mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440)}
	if _, ok := wlPickTarget(one, ""); ok {
		t.Fatal("单屏时不应选出副屏")
	}
	if _, ok := wlPickTarget(nil, ""); ok {
		t.Fatal("无显示器时不应选出副屏")
	}
}

// 三屏：主屏居中，右边一块、下边一块 —— 右侧相邻优先（优先级 0 < 2）。
func TestPickTargetPrefersRightOverBelow(t *testing.T) {
	primary := mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440)
	right := mkMonitor(`\\.\DISPLAY2`, false, 2560, 0, 4480, 1440)
	below := mkMonitor(`\\.\DISPLAY3`, false, 0, 1440, 2560, 2520)
	got, ok := wlPickTarget([]wlMonitor{below, primary, right}, "")
	if !ok || got.Name != right.Name {
		t.Fatalf("应选右侧屏 DISPLAY2，实际 %q", got.Name)
	}
}

// 首元素不共边、第二个才相邻时，不能让首元素凭"先入选"胜出（历史 bug）。
func TestPickTargetDoesNotKeepFirstCandidate(t *testing.T) {
	primary := mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440)
	far := mkMonitor(`\\.\DISPLAY4`, false, 9000, 9000, 10240, 10240)
	right := mkMonitor(`\\.\DISPLAY2`, false, 2560, 0, 4480, 1440)
	got, ok := wlPickTarget([]wlMonitor{far, right, primary}, "")
	if !ok || got.Name != right.Name {
		t.Fatalf("应选相邻的 DISPLAY2，实际 %q", got.Name)
	}
}

// 用户指定了目标设备名：即使不是相邻屏也用指定的那块。
func TestPickTargetHonoursPreferredDevice(t *testing.T) {
	primary := mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440)
	right := mkMonitor(`\\.\DISPLAY2`, false, 2560, 0, 4480, 1440)
	above := mkMonitor(`\\.\DISPLAY3`, false, 216, -1080, 2136, 0)
	got, ok := wlPickTarget([]wlMonitor{primary, right, above}, above.Name)
	if !ok || got.Name != above.Name {
		t.Fatalf("应选指定的 DISPLAY3，实际 %q", got.Name)
	}
}

// 指定设备名已不存在 / 为 auto：回退自动策略。
func TestPickTargetFallsBackWhenPreferredMissing(t *testing.T) {
	primary := mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440)
	above := mkMonitor(`\\.\DISPLAY3`, false, 216, -1080, 2136, 0)
	for _, pref := range []string{"auto", `\\.\DISPLAY9`} {
		got, ok := wlPickTarget([]wlMonitor{primary, above}, pref)
		if !ok || got.Name != above.Name {
			t.Fatalf("pref=%q 应回退到唯一副屏，实际 %q", pref, got.Name)
		}
	}
}

func TestNeighbourPriority(t *testing.T) {
	base := wlRect{0, 0, 2560, 1440}
	cases := []struct {
		name string
		m    wlRect
		prio int
	}{
		{"右侧", wlRect{2560, 0, 4480, 1440}, 0},
		{"左侧", wlRect{-1920, 0, 0, 1440}, 1},
		{"下方", wlRect{0, 1440, 2560, 2520}, 2},
		{"上方", wlRect{216, -1080, 2136, 0}, 3},
		{"垂直重叠不足", wlRect{2560, 1400, 4480, 2500}, 99},
	}
	for _, c := range cases {
		if prio, _ := wlNeighbourPriority(base, c.m); prio != c.prio {
			t.Errorf("%s：优先级应为 %d，实际 %d", c.name, c.prio, prio)
		}
	}
}

func TestPlacementOnAnyMonitor(t *testing.T) {
	mons := []wlMonitor{
		mkMonitor(`\\.\DISPLAY1`, true, 0, 0, 2560, 1440),
		mkMonitor(`\\.\DISPLAY2`, false, 216, -1080, 2136, 0),
	}
	onSecondary := wlWindowPlacement{NormalPosition: wlRect{216, -1080, 2136, 0}}
	if !wlPlacementOnAnyMonitor(onSecondary, mons) {
		t.Fatal("落在副屏上的位置应判定有效")
	}
	offScreen := wlWindowPlacement{NormalPosition: wlRect{-9000, -9000, -8000, -8100}}
	if wlPlacementOnAnyMonitor(offScreen, mons) {
		t.Fatal("显示器之外的位置应判定无效")
	}
}
