package admin

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/pkg/log"
	"github.com/swordkee/kratos-vue-admin/pkg/loginlock"
)

// 登录失败计数 / 账号锁定（gva service/system/security_lock.go 迁入）。
//
// 源是包级函数 + 全局 GVA_CACHE；KVA 改为用例 + 注入的 loginlock.Manager，
// 便于单测（内存后端）与多实例（Redis 后端）。
//
// fail-open 口径（与源一致）：Redis/缓存故障时「读不到锁 = 未锁定」、「计数失败只
// 记日志」，不因为缓存故障把全站登录拒之门外；锁定本身由配置 lockEnable 控制。

// LockStatus 账号锁状态快照（管理面查询用）
type LockStatus struct {
	Username      string
	LockEnable    bool
	Locked        bool
	FailCount     int64
	RemainSeconds int64
	Threshold     int32
	DurationMin   int32
}

// SecurityLockUseCase 登录锁定用例
type SecurityLockUseCase struct {
	cfg *SecurityConfigUseCase
	mgr *loginlock.Manager
	log *log.Helper
}

// NewSecurityLockUseCase 构造用例。cfg/mgr 任一为 nil 时全部方法退化为 no-op
// （等价于「关闭失败锁定」），保证未装配也不影响登录主链。
func NewSecurityLockUseCase(cfg *SecurityConfigUseCase, mgr *loginlock.Manager, logger log.Logger) *SecurityLockUseCase {
	return &SecurityLockUseCase{
		cfg: cfg,
		mgr: mgr,
		log: log.NewHelper(log.With(logger, "module", "biz/security_lock")),
	}
}

func (u *SecurityLockUseCase) config(ctx context.Context) (bool, int32, int32) {
	if u == nil || u.cfg == nil {
		return false, 0, 0
	}
	c := u.cfg.Current(ctx)
	return c.LockEnable, c.LockThreshold, c.LockDuration
}

// IsAccountLocked 查询账号是否处于锁定状态；同时返回剩余锁定时长。
func (u *SecurityLockUseCase) IsAccountLocked(ctx context.Context, username string) (bool, int32) {
	enabled, _, duration := u.config(ctx)
	if !enabled || u == nil || u.mgr == nil || username == "" {
		return false, 0
	}
	if !u.mgr.IsLocked(ctx, username) {
		return false, 0
	}
	return true, duration
}

// RecordLoginFail 记录一次登录失败：计数滚动窗口 TTL=LockDuration，达阈值即置锁。
func (u *SecurityLockUseCase) RecordLoginFail(ctx context.Context, username string) {
	enabled, threshold, duration := u.config(ctx)
	if !enabled || u == nil || u.mgr == nil || username == "" {
		return
	}
	if threshold <= 0 {
		threshold = 1
	}
	cfg := u.cfg.Current(ctx)
	n, err := u.mgr.IncrFail(ctx, username, LockDurationTimeout(cfg))
	if err != nil {
		// 与源一致：计数失败只告警，不阻断登录
		u.log.Warnf("登录失败计数失败: %v", err)
		return
	}
	if n >= int64(threshold) {
		if err := u.mgr.Lock(ctx, username, LockDurationTimeout(cfg)); err != nil {
			u.log.Warnf("登录锁定写入失败: %v", err)
			return
		}
		u.log.Infof("账号因连续登录失败被锁定: %s (阈值 %d, %d 分钟)", username, threshold, duration)
	}
}

// ClearLoginFail 清除失败计数与锁态（登录成功调用）。
func (u *SecurityLockUseCase) ClearLoginFail(ctx context.Context, username string) {
	if u == nil || u.mgr == nil || username == "" {
		return
	}
	u.mgr.Clear(ctx, username)
}

// Status 账号锁状态快照（管理面）。
func (u *SecurityLockUseCase) Status(ctx context.Context, username string) LockStatus {
	enabled, threshold, duration := u.config(ctx)
	st := LockStatus{
		Username:    username,
		LockEnable:  enabled,
		Threshold:   threshold,
		DurationMin: duration,
	}
	if u == nil || u.mgr == nil || username == "" {
		return st
	}
	st.Locked = u.mgr.IsLocked(ctx, username)
	st.FailCount = u.mgr.FailCount(ctx, username)
	st.RemainSeconds = u.mgr.RemainSeconds(ctx, username)
	return st
}

// Unlock 人工解锁（清计数 + 清锁态）。
func (u *SecurityLockUseCase) Unlock(ctx context.Context, username string) error {
	if u == nil || u.mgr == nil || username == "" {
		return nil
	}
	u.mgr.Clear(ctx, username)
	u.log.Infof("账号已人工解锁: %s", username)
	return nil
}
