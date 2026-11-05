package admin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
	"github.com/swordkee/kratos-vue-admin/pkg/loginlock"
)

// fakeSecurityConfigRepo 单行表替身（id=1）
type fakeSecurityConfigRepo struct {
	mu   sync.Mutex
	row  *model.SysSecurityConfig
	save int
}

func (r *fakeSecurityConfigRepo) FindByID(_ context.Context, id int64) (*model.SysSecurityConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.row == nil || r.row.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	c := *r.row
	return &c, nil
}

func (r *fakeSecurityConfigRepo) Create(_ context.Context, cfg *model.SysSecurityConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *cfg
	r.row = &c
	return nil
}

func (r *fakeSecurityConfigRepo) Save(_ context.Context, cfg *model.SysSecurityConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *cfg
	r.row = &c
	r.save++
	return nil
}

func newSecurityConfigFixture(t *testing.T) (*SecurityConfigUseCase, *fakeSecurityConfigRepo) {
	t.Helper()
	repo := &fakeSecurityConfigRepo{}
	return NewSecurityConfigUseCase(repo, nopLogger{}), repo
}

// nopLogger 满足 pkg/log.Logger 的空实现（测试不关注日志输出）。
type nopLogger struct{}

func (nopLogger) Log(level log.Level, keyvals ...any) error { return nil }

func TestDefaultSecurityConfig(t *testing.T) {
	cfg := DefaultSecurityConfig()
	if cfg.ID != SecurityConfigRowID {
		t.Fatalf("默认行 id 应为 1, got %d", cfg.ID)
	}
	if cfg.CaptchaOpen != 0 || cfg.CaptchaTimeout != 3600 || cfg.KeyLong != 6 || cfg.ImgWidth != 240 || cfg.ImgHeight != 80 {
		t.Fatalf("验证码默认值不符: %+v", cfg)
	}
	if cfg.PwdMinLength != 8 || cfg.LockThreshold != 5 || cfg.LockDuration != 30 || cfg.PwdExpireDays != 90 {
		t.Fatalf("数值默认值不符: %+v", cfg)
	}
	if cfg.LockEnable || cfg.LimitEnable || cfg.MfaRequired || cfg.ForceNewUserChangePassword || cfg.PwdExpireEnable {
		t.Fatalf("开关默认值应全关: %+v", cfg)
	}
}

func TestSecurityConfigGetCreatesDefaultRow(t *testing.T) {
	uc, repo := newSecurityConfigFixture(t)
	cfg, err := uc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if cfg.PwdMinLength != 8 {
		t.Fatalf("应落默认行, got %+v", cfg)
	}
	repo.mu.Lock()
	created := repo.row
	repo.mu.Unlock()
	if created == nil || created.ID != SecurityConfigRowID {
		t.Fatal("缺行时应按默认值补建 id=1")
	}
}

func TestSecurityConfigCurrentCaches(t *testing.T) {
	uc, repo := newSecurityConfigFixture(t)
	ctx := context.Background()
	// 先落一行拟真配置
	repo.row = &model.SysSecurityConfig{ID: SecurityConfigRowID, PwdMinLength: 12, LockThreshold: 3}

	got := uc.Current(ctx)
	if got.PwdMinLength != 12 || got.LockThreshold != 3 {
		t.Fatalf("首次 Current 应读库: %+v", got)
	}
	// 直接改库不改缓存：Current 仍返回缓存值（热读语义）
	repo.mu.Lock()
	repo.row.PwdMinLength = 20
	repo.mu.Unlock()
	if got := uc.Current(ctx); got.PwdMinLength != 12 {
		t.Fatalf("Current 应命中缓存, got %d", got.PwdMinLength)
	}
}

func TestSecurityConfigUpdateRoundTrip(t *testing.T) {
	uc, repo := newSecurityConfigFixture(t)
	ctx := context.Background()
	repo.row = &model.SysSecurityConfig{ID: SecurityConfigRowID, PwdMinLength: 8}

	updated, err := uc.Update(ctx, model.SysSecurityConfig{
		LockEnable:    true,
		LockThreshold: 2,
		LockDuration:  5,
		PwdMinLength:  10,
		CreatedAt:     time.Unix(0, 0).UTC(), // 应被 prev 覆盖
	})
	if err != nil {
		t.Fatalf("Update 失败: %v", err)
	}
	if updated.ID != SecurityConfigRowID || !updated.LockEnable || updated.LockThreshold != 2 {
		t.Fatalf("Update 返回值异常: %+v", updated)
	}
	// 缓存热更新
	if got := uc.Current(ctx); !got.LockEnable || got.LockThreshold != 2 {
		t.Fatalf("Update 后缓存应热更新: %+v", got)
	}
	// 落库
	repo.mu.Lock()
	stored := repo.row
	saves := repo.save
	repo.mu.Unlock()
	if saves != 1 {
		t.Fatalf("应保存一次, got %d", saves)
	}
	if stored.PwdMinLength != 10 || !stored.LockEnable {
		t.Fatalf("落库值不符: %+v", stored)
	}
	if !stored.CreatedAt.IsZero() {
		t.Fatalf("created_at 应保留原值(零值), got %v", stored.CreatedAt)
	}
}

func TestDurationHelpers(t *testing.T) {
	cfg := model.SysSecurityConfig{CaptchaTimeout: 3600, LockDuration: 30, LimitWindow: 60}
	if CaptchaTimeoutDuration(cfg) != time.Hour {
		t.Fatalf("CaptchaTimeoutDuration 应为 1h")
	}
	if LockDurationTimeout(cfg) != 30*time.Minute {
		t.Fatalf("LockDurationTimeout 应为 30m")
	}
	if LimitWindowDuration(cfg) != time.Minute {
		t.Fatalf("LimitWindowDuration 应为 1m")
	}
}

func TestValidatePasswordComplexity(t *testing.T) {
	base := model.SysSecurityConfig{PwdMinLength: 8}
	if err := ValidatePasswordComplexity("Abcd1234", base); err != nil {
		t.Fatalf("满足最小长度应通过: %v", err)
	}
	if err := ValidatePasswordComplexity("Ab1!", base); err == nil {
		t.Fatal("短于最小长度应拒绝")
	}

	strict := model.SysSecurityConfig{
		PwdMinLength: 8, PwdRequireUpper: true, PwdRequireLower: true,
		PwdRequireDigit: true, PwdRequireSpecial: true,
	}
	if err := ValidatePasswordComplexity("Abcd1234", strict); err == nil {
		t.Fatal("缺特殊字符应拒绝")
	}
	if err := ValidatePasswordComplexity("Abcd123!", strict); err != nil {
		t.Fatalf("四类齐全应通过: %v", err)
	}
	// 中文按 rune 计长（源 utf8.RuneCountInString 口径）
	if err := ValidatePasswordComplexity("密码密码密码密码", model.SysSecurityConfig{PwdMinLength: 8}); err != nil {
		t.Fatalf("8 个中文字符应满足长度 8: %v", err)
	}
}

func TestIsPasswordExpired(t *testing.T) {
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	cfg := model.SysSecurityConfig{PwdExpireEnable: true, PwdExpireDays: 90}
	old := now.AddDate(0, 0, -100)
	recent := now.AddDate(0, 0, -10)

	if !IsPasswordExpired(&old, cfg, now) {
		t.Fatal("100 天前应过期")
	}
	if IsPasswordExpired(&recent, cfg, now) {
		t.Fatal("10 天前不应过期")
	}
	if IsPasswordExpired(nil, cfg, now) {
		t.Fatal("nil 时间不应过期")
	}
	if IsPasswordExpired(&old, model.SysSecurityConfig{PwdExpireEnable: false, PwdExpireDays: 90}, now) {
		t.Fatal("未开启时不应过期")
	}
	if !ShouldForcePasswordChange(true, &recent, cfg, now) {
		t.Fatal("初始密码义务应触发强制改密")
	}
	if ShouldForcePasswordChange(false, &recent, cfg, now) {
		t.Fatal("无义务且未过期不应强制改密")
	}
}

// ------ 失败锁定 ------

func newLockFixture(t *testing.T, cfg model.SysSecurityConfig) (*SecurityLockUseCase, *SecurityConfigUseCase) {
	t.Helper()
	cfgRepo := &fakeSecurityConfigRepo{row: &cfg}
	cfgRepo.row.ID = SecurityConfigRowID
	cfgUC := NewSecurityConfigUseCase(cfgRepo, nopLogger{})
	lock := NewSecurityLockUseCase(cfgUC, loginlock.NewManager(nil), nopLogger{})
	return lock, cfgUC
}

func TestSecurityLockThreshold(t *testing.T) {
	lock, _ := newLockFixture(t, model.SysSecurityConfig{
		ID: SecurityConfigRowID, LockEnable: true, LockThreshold: 3, LockDuration: 30,
	})
	ctx := context.Background()
	user := "lockuser"

	if locked, _ := lock.IsAccountLocked(ctx, user); locked {
		t.Fatal("初始不应锁定")
	}
	lock.RecordLoginFail(ctx, user)
	lock.RecordLoginFail(ctx, user)
	if locked, _ := lock.IsAccountLocked(ctx, user); locked {
		t.Fatal("2 次失败（阈值 3）不应锁定")
	}
	lock.RecordLoginFail(ctx, user)
	if locked, minutes := lock.IsAccountLocked(ctx, user); !locked || minutes != 30 {
		t.Fatalf("3 次失败应锁定且返回 30 分钟, got locked=%v minutes=%d", locked, minutes)
	}

	st := lock.Status(ctx, user)
	if !st.Locked || st.FailCount != 3 || st.RemainSeconds <= 0 || st.Threshold != 3 || !st.LockEnable {
		t.Fatalf("锁状态快照异常: %+v", st)
	}

	// 登录成功路径：清计数与锁
	lock.ClearLoginFail(ctx, user)
	if locked, _ := lock.IsAccountLocked(ctx, user); locked {
		t.Fatal("Clear 后应解锁")
	}
	if st := lock.Status(ctx, user); st.FailCount != 0 || st.Locked {
		t.Fatalf("Clear 后状态应为空: %+v", st)
	}

	// 人工解锁
	lock.RecordLoginFail(ctx, user)
	lock.RecordLoginFail(ctx, user)
	lock.RecordLoginFail(ctx, user)
	if err := lock.Unlock(ctx, user); err != nil {
		t.Fatalf("解锁失败: %v", err)
	}
	if st := lock.Status(ctx, user); st.Locked || st.FailCount != 0 {
		t.Fatalf("人工解锁后应为空: %+v", st)
	}
}

func TestSecurityLockDisabled(t *testing.T) {
	lock, _ := newLockFixture(t, model.SysSecurityConfig{
		ID: SecurityConfigRowID, LockEnable: false, LockThreshold: 1, LockDuration: 30,
	})
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		lock.RecordLoginFail(ctx, "u")
	}
	if locked, _ := lock.IsAccountLocked(ctx, "u"); locked {
		t.Fatal("未开启失败锁定时不应锁定")
	}
	if st := lock.Status(ctx, "u"); st.LockEnable {
		t.Fatal("Status 应反映 lockEnable=false")
	}
}

func TestSecurityLockNilSafe(t *testing.T) {
	var lock *SecurityLockUseCase
	ctx := context.Background()
	if locked, _ := lock.IsAccountLocked(ctx, "u"); locked {
		t.Fatal("nil 用例不应锁定")
	}
	lock.RecordLoginFail(ctx, "u")
	lock.ClearLoginFail(ctx, "u")
	if err := lock.Unlock(ctx, "u"); err != nil {
		t.Fatalf("nil 用例解锁应 no-op: %v", err)
	}
	if st := lock.Status(ctx, "u"); st.Locked || st.LockEnable {
		t.Fatalf("nil 用例状态应为空: %+v", st)
	}
}

func TestSecurityLockEmptyUsername(t *testing.T) {
	lock, _ := newLockFixture(t, model.SysSecurityConfig{
		ID: SecurityConfigRowID, LockEnable: true, LockThreshold: 1, LockDuration: 30,
	})
	ctx := context.Background()
	lock.RecordLoginFail(ctx, "")
	if locked, _ := lock.IsAccountLocked(ctx, ""); locked {
		t.Fatal("空用户名不应进入计数/锁定")
	}
}

// 配置读取失败时 IsAccountLocked 走默认值（fail-open，不挡登录）
func TestSecurityLockConfigErrorFallback(t *testing.T) {
	repo := &failingConfigRepo{}
	cfgUC := NewSecurityConfigUseCase(repo, nopLogger{})
	lock := NewSecurityLockUseCase(cfgUC, loginlock.NewManager(nil), nopLogger{})
	ctx := context.Background()
	lock.RecordLoginFail(ctx, "u")
	if locked, _ := lock.IsAccountLocked(ctx, "u"); locked {
		t.Fatal("配置读不到时应按默认值(关闭锁定)处理")
	}
}

type failingConfigRepo struct{}

func (failingConfigRepo) FindByID(context.Context, int64) (*model.SysSecurityConfig, error) {
	return nil, errors.New("db down")
}
func (failingConfigRepo) Create(context.Context, *model.SysSecurityConfig) error { return nil }
func (failingConfigRepo) Save(context.Context, *model.SysSecurityConfig) error   { return nil }
