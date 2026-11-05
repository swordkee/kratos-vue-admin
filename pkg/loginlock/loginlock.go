// Package loginlock 登录失败计数与账号锁定存储（gva service/system/security_lock.go
// 的存取层语义迁入）。
//
// 与源口径一致的三点：
//  1. 双后端：注入了 Redis 客户端就多实例共享；未注入（nil）时回退进程内存，
//     对齐 gva GVA_CACHE 的 Redis/内存双实现；
//  2. 计数是「固定窗口」：首次失败时设置 TTL（窗口起点），后续自增不续期；
//  3. Redis 故障时读侧返回「未锁定」（fail-open，与源 gva_cache.Exists 返回 false
//     一致），写侧由调用方记日志跳过——锁定是运营策略，不能因缓存故障把全站登录打死。
package loginlock

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// FailKeyPrefix 登录失败计数 key 前缀（gva login_fail:）
	FailKeyPrefix = "login_fail:"
	// LockKeyPrefix 账号锁定标记 key 前缀（gva login_lock:）
	LockKeyPrefix = "login_lock:"
)

// Manager 失败计数/锁态存取器。rdb 为 nil 时走进程内存。
type Manager struct {
	rdb redis.UniversalClient

	mu       sync.Mutex
	counters map[string]memCounter
	locks    map[string]time.Time
	now      func() time.Time
}

type memCounter struct {
	val int64
	exp time.Time
}

// NewManager 构造存取器；rdb 为 nil 表示无 Redis（进程内存，单实例/测试）。
func NewManager(rdb redis.UniversalClient) *Manager {
	return &Manager{
		rdb:      rdb,
		counters: make(map[string]memCounter),
		locks:    make(map[string]time.Time),
		now:      time.Now,
	}
}

// FailKey 失败计数 key。
func FailKey(username string) string { return FailKeyPrefix + username }

// LockKey 锁态 key。
func LockKey(username string) string { return LockKeyPrefix + username }

// IsLocked 查询账号是否处于锁定状态。读失败返回 false（对齐源 gva_cache.Exists）。
func (m *Manager) IsLocked(ctx context.Context, username string) bool {
	if m == nil {
		return false
	}
	if m.rdb != nil {
		n, err := m.rdb.Exists(ctx, LockKey(username)).Result()
		return err == nil && n > 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.locks[LockKey(username)]
	if !ok {
		return false
	}
	if !m.now().Before(exp) {
		delete(m.locks, LockKey(username))
		return false
	}
	return true
}

// RemainSeconds 剩余锁定秒数（未锁定=0）。
func (m *Manager) RemainSeconds(ctx context.Context, username string) int64 {
	if m == nil {
		return 0
	}
	if m.rdb != nil {
		d, err := m.rdb.PTTL(ctx, LockKey(username)).Result()
		if err != nil || d <= 0 {
			return 0
		}
		return int64(d / time.Second)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.locks[LockKey(username)]
	if !ok {
		return 0
	}
	remain := exp.Sub(m.now())
	if remain <= 0 {
		delete(m.locks, LockKey(username))
		return 0
	}
	return int64(remain / time.Second)
}

// FailCount 当前窗口内失败次数（读失败返回 0）。
func (m *Manager) FailCount(ctx context.Context, username string) int64 {
	if m == nil {
		return 0
	}
	if m.rdb != nil {
		v, err := m.rdb.Get(ctx, FailKey(username)).Result()
		if err != nil {
			return 0
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.counters[FailKey(username)]
	if !ok || !m.now().Before(c.exp) {
		delete(m.counters, FailKey(username))
		return 0
	}
	return c.val
}

// IncrFail 失败计数 +1，返回新计数；窗口首次计数时设置 TTL（固定窗口，不续期）。
func (m *Manager) IncrFail(ctx context.Context, username string, ttl time.Duration) (int64, error) {
	if m == nil {
		return 0, nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	if m.rdb != nil {
		v, err := m.rdb.IncrBy(ctx, FailKey(username), 1).Result()
		if err != nil {
			return 0, err
		}
		// 首次计数时设置过期（窗口起点），与 gva redisCache.IncrementWithExpire 一致
		if v == 1 {
			if err := m.rdb.Expire(ctx, FailKey(username), ttl).Err(); err != nil {
				return v, err
			}
		}
		return v, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := FailKey(username)
	now := m.now()
	c, ok := m.counters[key]
	if !ok || !now.Before(c.exp) {
		c = memCounter{val: 1, exp: now.Add(ttl)}
	} else {
		c.val++
	}
	m.counters[key] = c
	return c.val, nil
}

// Lock 置锁（TTL=锁定时长）。
func (m *Manager) Lock(ctx context.Context, username string, ttl time.Duration) error {
	if m == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	if m.rdb != nil {
		return m.rdb.Set(ctx, LockKey(username), 1, ttl).Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locks[LockKey(username)] = m.now().Add(ttl)
	return nil
}

// Clear 清除失败计数与锁态（登录成功/人工解锁）。
func (m *Manager) Clear(ctx context.Context, username string) {
	if m == nil {
		return
	}
	if m.rdb != nil {
		m.rdb.Del(ctx, FailKey(username), LockKey(username))
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counters, FailKey(username))
	delete(m.locks, LockKey(username))
}
