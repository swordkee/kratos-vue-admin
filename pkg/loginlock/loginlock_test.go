package loginlock

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCounterWindowAndLock(t *testing.T) {
	m := NewManager(nil)
	ctx := context.Background()
	user := "alice"

	if m.IsLocked(ctx, user) {
		t.Fatal("初始不应锁定")
	}
	if got := m.FailCount(ctx, user); got != 0 {
		t.Fatalf("初始计数应为 0, got %d", got)
	}

	n, err := m.IncrFail(ctx, user, time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("首次计数应为 1, got n=%d err=%v", n, err)
	}
	n, _ = m.IncrFail(ctx, user, time.Minute)
	if n != 2 {
		t.Fatalf("第二次计数应为 2, got %d", n)
	}
	if got := m.FailCount(ctx, user); got != 2 {
		t.Fatalf("FailCount 应为 2, got %d", got)
	}

	// 达阈值置锁
	if err := m.Lock(ctx, user, 30*time.Minute); err != nil {
		t.Fatalf("置锁失败: %v", err)
	}
	if !m.IsLocked(ctx, user) {
		t.Fatal("置锁后应锁定")
	}
	if remain := m.RemainSeconds(ctx, user); remain < 29*60 || remain > 30*60 {
		t.Fatalf("剩余锁定秒数异常: %d", remain)
	}

	// 解锁：计数与锁态同时清
	m.Clear(ctx, user)
	if m.IsLocked(ctx, user) {
		t.Fatal("Clear 后应解锁")
	}
	if got := m.FailCount(ctx, user); got != 0 {
		t.Fatalf("Clear 后计数应为 0, got %d", got)
	}
	if remain := m.RemainSeconds(ctx, user); remain != 0 {
		t.Fatalf("Clear 后剩余秒数应为 0, got %d", remain)
	}
}

// TestMemoryFixedWindow 窗口过期后计数归零（源 IncrementWithExpire 固定窗口语义）
func TestMemoryFixedWindow(t *testing.T) {
	m := NewManager(nil)
	ctx := context.Background()
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	if n, _ := m.IncrFail(ctx, "bob", time.Minute); n != 1 {
		t.Fatalf("首次计数应为 1, got %d", n)
	}
	now = now.Add(30 * time.Second)
	if n, _ := m.IncrFail(ctx, "bob", time.Minute); n != 2 {
		t.Fatalf("窗口内第二次应为 2, got %d", n)
	}
	// 越过窗口：重新从 1 开始
	now = now.Add(2 * time.Minute)
	if n, _ := m.IncrFail(ctx, "bob", time.Minute); n != 1 {
		t.Fatalf("新窗口应从 1 开始, got %d", n)
	}
	if got := m.FailCount(ctx, "bob"); got != 1 {
		t.Fatalf("FailCount 应为 1, got %d", got)
	}
}

func TestNilManagerSafe(t *testing.T) {
	var m *Manager
	ctx := context.Background()
	if m.IsLocked(ctx, "x") || m.FailCount(ctx, "x") != 0 || m.RemainSeconds(ctx, "x") != 0 {
		t.Fatal("nil Manager 读操作应零值")
	}
	if n, err := m.IncrFail(ctx, "x", time.Minute); n != 0 || err != nil {
		t.Fatal("nil Manager 写操作应 no-op")
	}
	if err := m.Lock(ctx, "x", time.Minute); err != nil {
		t.Fatal("nil Manager Lock 应 no-op")
	}
	m.Clear(ctx, "x")
}
