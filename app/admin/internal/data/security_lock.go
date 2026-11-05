package data

import "github.com/swordkee/kratos-vue-admin/pkg/loginlock"

// NewLoginLockManager 登录失败计数/锁态存取器。
//
// 注入了 Redis 客户端即多实例共享（gva GVA_CACHE 的 Redis 后端）；
// Data.rdb 为 nil（单测/未装配）时退化为进程内存，语义与源内存后端一致。
func NewLoginLockManager(d *Data) *loginlock.Manager {
	if d == nil {
		return loginlock.NewManager(nil)
	}
	if d.rdb == nil {
		return loginlock.NewManager(nil)
	}
	return loginlock.NewManager(d.rdb)
}
