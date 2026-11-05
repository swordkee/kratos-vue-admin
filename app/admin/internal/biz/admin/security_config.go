package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

// 登录安全策略配置（gva model/system/sys_security_config.go +
// service/system/sys_security_config.go + utils/password_complexity.go 迁入）。
//
// 与源一致：单行表（固定 id=1）、进程内原子缓存热读、未初始化时回退代码默认值且
// 不写缓存（库就绪后再惰性加载）、密码过期由关变开时回填存量 NULL 用户。
//
// 默认值常量刻意与 gva DefaultSecurityConfig 逐字段一致，避免迁移后行为漂移。
const (
	defaultCaptchaOpen    = 0
	defaultCaptchaTimeout = 3600
	defaultCaptchaKeyLong = 6
	defaultCaptchaWidth   = 240
	defaultCaptchaHeight  = 80

	// SecurityConfigRowID 单行表固定主键
	SecurityConfigRowID int64 = 1
)

// SecurityConfigRepo 安全配置仓储（表 sys_security_config）
type SecurityConfigRepo interface {
	FindByID(ctx context.Context, id int64) (*model.SysSecurityConfig, error)
	Create(ctx context.Context, cfg *model.SysSecurityConfig) error
	Save(ctx context.Context, cfg *model.SysSecurityConfig) error
}

// SecurityConfigUseCase 安全配置用例
type SecurityConfigUseCase struct {
	repo  SecurityConfigRepo
	log   *log.Helper
	cache atomic.Value // model.SysSecurityConfig
}

// NewSecurityConfigUseCase 构造用例（不主动查库，首次 Current/Get 惰性加载）。
func NewSecurityConfigUseCase(repo SecurityConfigRepo, logger log.Logger) *SecurityConfigUseCase {
	return &SecurityConfigUseCase{
		repo: repo,
		log:  log.NewHelper(log.With(logger, "module", "biz/security_config")),
	}
}

// DefaultSecurityConfig 首次初始化与缺行回退使用的默认值（对齐 gva。）
func DefaultSecurityConfig() model.SysSecurityConfig {
	return model.SysSecurityConfig{
		ID:                         SecurityConfigRowID,
		CaptchaOpen:                defaultCaptchaOpen,
		CaptchaTimeout:             defaultCaptchaTimeout,
		KeyLong:                    defaultCaptchaKeyLong,
		ImgWidth:                   defaultCaptchaWidth,
		ImgHeight:                  defaultCaptchaHeight,
		PwdMinLength:               8,
		PwdRequireUpper:            false,
		PwdRequireLower:            false,
		PwdRequireDigit:            false,
		PwdRequireSpecial:          false,
		LimitEnable:                false,
		LimitWindow:                60,
		LimitCount:                 30,
		LockEnable:                 false,
		LockThreshold:              5,
		LockDuration:               30,
		PwdExpireEnable:            false,
		PwdExpireDays:              90,
		ForceNewUserChangePassword: false,
		MfaRequired:                false,
	}
}

// CaptchaTimeoutDuration 防爆破计数缓存超时
func CaptchaTimeoutDuration(c model.SysSecurityConfig) time.Duration {
	return time.Duration(c.CaptchaTimeout) * time.Second
}

// LockDurationTimeout 锁定时长
func LockDurationTimeout(c model.SysSecurityConfig) time.Duration {
	return time.Duration(c.LockDuration) * time.Minute
}

// LimitWindowDuration 限流窗口
func LimitWindowDuration(c model.SysSecurityConfig) time.Duration {
	return time.Duration(c.LimitWindow) * time.Second
}

// Get 读取单行配置，不存在则按代码默认值创建并返回。
// 库不可用时返回默认值 + error：调用方 Current 据此不写缓存（源同款语义）。
func (u *SecurityConfigUseCase) Get(ctx context.Context) (model.SysSecurityConfig, error) {
	cfg, err := u.repo.FindByID(ctx, SecurityConfigRowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fresh := DefaultSecurityConfig()
			if err := u.repo.Create(ctx, &fresh); err != nil {
				return fresh, err
			}
			return fresh, nil
		}
		return DefaultSecurityConfig(), err
	}
	return *cfg, nil
}

// Current 返回内存缓存当前配置；未加载则惰性 Get（失败时返回代码默认值）。
func (u *SecurityConfigUseCase) Current(ctx context.Context) model.SysSecurityConfig {
	if v := u.cache.Load(); v != nil {
		return v.(model.SysSecurityConfig)
	}
	cfg, err := u.Get(ctx)
	if err == nil {
		u.cache.Store(cfg)
	}
	return cfg
}

// Update 持久化配置并刷新内存缓存；密码过期由关变开时回填存量 NULL 用户。
func (u *SecurityConfigUseCase) Update(ctx context.Context, cfg model.SysSecurityConfig) (model.SysSecurityConfig, error) {
	prev, err := u.Get(ctx)
	if err != nil {
		return cfg, err
	}
	cfg.ID = prev.ID
	if cfg.ID == 0 {
		cfg.ID = SecurityConfigRowID
	}
	cfg.CreatedAt = prev.CreatedAt
	if err := u.repo.Save(ctx, &cfg); err != nil {
		return cfg, err
	}
	u.cache.Store(cfg)
	// 源在「密码过期由关变开」时回填 sys_users.password_updated_at。
	// KVA 的 sys_users 没有该列（密码过期策略未接入登录链），故此处不做回填，
	// 见 deploy/sql/20261105_sys_security_config.sql 头注；接入前不得把 pwdExpireEnable 当生效开关。
	return cfg, nil
}

// LoadAll 启动时加载配置入内存缓存（可选；Current 也会惰性加载）。
func (u *SecurityConfigUseCase) LoadAll(ctx context.Context) {
	cfg, err := u.Get(ctx)
	if err != nil {
		u.log.Warnf("加载安全配置失败: %v", err)
		return
	}
	u.cache.Store(cfg)
}

// CurrentLimit 供限流中间件读取（enable/window 秒/count）。
func (u *SecurityConfigUseCase) CurrentLimit(ctx context.Context) (enable bool, window int, count int) {
	cfg := u.Current(ctx)
	return cfg.LimitEnable, int(cfg.LimitWindow), int(cfg.LimitCount)
}

// ValidatePasswordComplexity 按安全配置校验密码复杂度，不满足返回可读错误
// （gva utils/password_complexity.go 逐条同义）。
func ValidatePasswordComplexity(pwd string, cfg model.SysSecurityConfig) error {
	if cfg.PwdMinLength > 0 && utf8.RuneCountInString(pwd) < int(cfg.PwdMinLength) {
		return fmt.Errorf("密码长度不能少于 %d 位", cfg.PwdMinLength)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range pwd {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	var missing []string
	if cfg.PwdRequireUpper && !hasUpper {
		missing = append(missing, "大写字母")
	}
	if cfg.PwdRequireLower && !hasLower {
		missing = append(missing, "小写字母")
	}
	if cfg.PwdRequireDigit && !hasDigit {
		missing = append(missing, "数字")
	}
	if cfg.PwdRequireSpecial && !hasSpecial {
		missing = append(missing, "特殊字符")
	}
	if len(missing) > 0 {
		return fmt.Errorf("密码必须包含%s", strings.Join(missing, "、"))
	}
	return nil
}

// IsPasswordExpired 纯函数：判定密码是否过期（源 security_lock.go 同义）。
func IsPasswordExpired(passwordUpdatedAt *time.Time, cfg model.SysSecurityConfig, now time.Time) bool {
	if !cfg.PwdExpireEnable || cfg.PwdExpireDays <= 0 {
		return false
	}
	if passwordUpdatedAt == nil {
		return false
	}
	deadline := passwordUpdatedAt.AddDate(0, 0, int(cfg.PwdExpireDays))
	return now.After(deadline)
}

// ShouldForcePasswordChange 合并「初始密码必须改」与过期策略（源同义）。
// KVA 登录响应暂无 mustChangePassword 字段，本函数供后续接入与单测复用。
func ShouldForcePasswordChange(mustChange bool, passwordUpdatedAt *time.Time, cfg model.SysSecurityConfig, now time.Time) bool {
	return mustChange || IsPasswordExpired(passwordUpdatedAt, cfg, now)
}
