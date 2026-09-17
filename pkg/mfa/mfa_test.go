package mfa

// MFA Service 层测试（对齐 api-admin sys_mfa_test.go）：
//   - TOTP 全链：Status → StartEnroll → ConfirmEnable（动态码校验+恢复码）→
//     VerifyForLogin（动态码+恢复码）→ Disable（二次确认）→ 管理员 Reset
//   - 恢复码一次性（bcrypt 哈希 + used_at 标记；明文仅 ConfirmEnable 返回一次）
//   - 动态码用真实 pquerna/totp.GenerateCode 计算（非固定值，防空洞）
// 反证：错误码/失效会话/重放恢复码必须被拒。

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"gorm.io/gorm"
)

var mfaTestKey = "0123456789abcdef0123456789abcdef" // 32 字节测试主密钥

// rawStore sys_users MFA 列读写测试桩（实现 MFAUserStore）。pkg 测试不能 import
// app/admin/internal/data/mfastore（internal 边界），此桩以旧 map 语义复现同一行为；
// gen 字段对象的生产实现由 biz 层 mfa_login_test.go 经 mfastore 端到端覆盖。
type rawStore struct{ db *gorm.DB }

func (r *rawStore) Under(tx *gorm.DB) MFAUserStore { return &rawStore{db: tx} }

func (r *rawStore) GetMFA(ctx context.Context, userID int64) (UserMFA, bool, error) {
	var u struct {
		MfaEnabled bool       `gorm:"column:mfa_enabled"`
		MfaSecret  string     `gorm:"column:mfa_secret"`
		MfaBoundAt *time.Time `gorm:"column:mfa_bound_at"`
	}
	res := r.db.WithContext(ctx).Table("sys_users").
		Select("mfa_enabled", "mfa_secret", "mfa_bound_at").
		Where("id = ?", userID).Scan(&u)
	if res.Error != nil {
		return UserMFA{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return UserMFA{}, false, nil
	}
	return UserMFA{Enabled: u.MfaEnabled, Secret: u.MfaSecret, BoundAt: u.MfaBoundAt}, true, nil
}

func (r *rawStore) SetMFA(ctx context.Context, userID int64, u UserMFA) (int64, error) {
	res := r.db.WithContext(ctx).Table("sys_users").Where("id = ?", userID).Updates(map[string]any{
		"mfa_enabled":  u.Enabled,
		"mfa_secret":   u.Secret,
		"mfa_bound_at": u.BoundAt,
	})
	return res.RowsAffected, res.Error
}

// newTestService 内存 sqlite（sys_users + 恢复码表）+ miniredis + 启用配置
func newTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE sys_users (
		id INTEGER PRIMARY KEY,
		username TEXT,
		mfa_enabled INTEGER DEFAULT 0,
		mfa_secret TEXT,
		mfa_bound_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sys_mfa_recovery_code (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		code_hash TEXT,
		used_at DATETIME
	)`).Error)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewService(&rawStore{db: db}, db, rdb, Config{
		Enabled:       true,
		EncryptionKey: mfaTestKey,
		Issuer:        "BMALL-Test",
	}, logx.NewLogger(nil))
	return svc, mr
}

// seedUser 插入测试用户，返回 id
func seedUser(t *testing.T, db *gorm.DB, id int64, username string, mfaEnabled bool) {
	t.Helper()
	enabled := 0
	if mfaEnabled {
		enabled = 1
	}
	require.NoError(t, db.Exec(
		"INSERT INTO sys_users (id, username, mfa_enabled) VALUES (?, ?, ?)",
		id, username, enabled,
	).Error)
}

// currentCode 用真实 TOTP 计算当前动态码
func currentCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      0,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	require.NoError(t, err)
	return code
}

// currentCodeForUser 读取用户库内加密 secret 并计算当前动态码（对齐 verify 解密口径）
func currentCodeForUser(t *testing.T, svc *Service, uid int64) string {
	t.Helper()
	var enc string
	require.NoError(t, svc.db.Table("sys_users").Select("mfa_secret").Where("id = ?", uid).Scan(&enc).Error)
	require.NotEmpty(t, enc)
	secret, err := DecryptSecret(enc, []byte(mfaTestKey))
	require.NoError(t, err)
	return currentCode(t, secret)
}

// boundUser 绑定完成的用户（含 ConfirmEnable 返回的明文恢复码——仅此一份）
type boundUser struct {
	secret        string
	recoveryCodes []string
}

// bind 完成一次完整绑定（enroll → 动态码确认），捕获明文恢复码
func bind(t *testing.T, svc *Service, uid int64, username string) *boundUser {
	t.Helper()
	seedUser(t, svc.db, uid, username, false)
	secret, _, err := svc.StartEnroll(context.Background(), uid, username)
	require.NoError(t, err)
	code := currentCode(t, secret)
	codes, err := svc.ConfirmEnable(context.Background(), uid, code)
	require.NoError(t, err)
	require.Len(t, codes, RecoveryCodeCount)
	return &boundUser{secret: secret, recoveryCodes: codes}
}

func TestStatus_Disabled_AvailableFalse(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	// 配置关掉 → Available=false（前端隐藏入口）
	svc.cfg.Enabled = false
	st, err := svc.Status(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, st.Available)
	assert.False(t, st.Enabled)
}

func TestStatus_UserNotFound(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	_, err := svc.Status(context.Background(), 999)
	require.ErrorIs(t, err, ErrNoUser)
}

func TestStatus_Enabled(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	seedUser(t, svc.db, 1, "admin", true)
	st, err := svc.Status(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, st.Available)
	assert.True(t, st.Enabled)
}

func TestStartEnroll_And_ConfirmEnable(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	seedUser(t, svc.db, 1, "admin", false)

	// StartEnroll：返回 secret + URL，Redis 有缓存
	secret, url, err := svc.StartEnroll(context.Background(), 1, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Contains(t, url, "otpauth://totp/")
	require.Contains(t, url, "BMALL-Test")
	require.True(t, mr.Exists("Mfa:Enroll:1"), "绑定会话必须缓存到 Redis")

	// 未确认前再次 enroll → 允许重新生成（参考实现：mfa_enabled 未变允许覆盖）
	secret2, _, err := svc.StartEnroll(context.Background(), 1, "admin")
	require.NoError(t, err)
	require.NotEqual(t, secret, secret2, "每次生成不同密钥")

	// 错误动态码 → 拒绝
	_, err = svc.ConfirmEnable(context.Background(), 1, "000000")
	require.ErrorIs(t, err, ErrCodeInvalid)

	// 正确动态码 → 启用 + 恢复码落库
	secret = secret2 // 用第二次的 secret
	code := currentCode(t, secret)
	codes, err := svc.ConfirmEnable(context.Background(), 1, code)
	require.NoError(t, err)
	require.Len(t, codes, RecoveryCodeCount)

	// 数据库已更新
	var enabled bool
	require.NoError(t, svc.db.Table("sys_users").Select("mfa_enabled").Where("id = 1").Scan(&enabled).Error)
	assert.True(t, enabled)
	// 恢复码 bcrypt 落库（10 行，未使用）
	var cnt int64
	require.NoError(t, svc.db.Table("sys_mfa_recovery_code").Where("user_id = 1 AND used_at IS NULL").Count(&cnt).Error)
	assert.Equal(t, int64(10), cnt)
	// 缓存已删
	assert.False(t, mr.Exists("Mfa:Enroll:1"))
}

func TestConfirmEnable_ExpiredSession(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	seedUser(t, svc.db, 1, "admin", false)

	// 无缓存会话（未 enroll）→ ErrEnrollExpired
	_, err := svc.ConfirmEnable(context.Background(), 1, "123456")
	require.ErrorIs(t, err, ErrEnrollExpired)

	// enroll 后 TTL 过期（FastForward）→ ErrEnrollExpired
	_, _, err = svc.StartEnroll(context.Background(), 1, "admin")
	require.NoError(t, err)
	mr.FastForward(6 * 60 * 1e9) // 6 分钟 > TTL 5 分钟
	_, err = svc.ConfirmEnable(context.Background(), 1, "123456")
	require.ErrorIs(t, err, ErrEnrollExpired)
}

func TestDisable_RequiresCode(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	seedUser(t, svc.db, 1, "admin", false)

	// 未绑定 → ErrNotEnabled
	err := svc.Disable(context.Background(), 1, "123456")
	require.ErrorIs(t, err, ErrNotEnabled)
}

func TestDisable_WithRecoveryCode(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	bu := bind(t, svc, 1, "admin")

	// 错误码 → ErrCodeInvalid
	err := svc.Disable(context.Background(), 1, "000000")
	require.ErrorIs(t, err, ErrCodeInvalid)

	// 用恢复码关闭
	err = svc.Disable(context.Background(), 1, bu.recoveryCodes[0])
	require.NoError(t, err)

	// 已关闭 + 恢复码清空
	var enabled bool
	require.NoError(t, svc.db.Table("sys_users").Select("mfa_enabled").Where("id = 1").Scan(&enabled).Error)
	assert.False(t, enabled)
	var cnt int64
	require.NoError(t, svc.db.Table("sys_mfa_recovery_code").Where("user_id = 1").Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt)
}

func TestResetMFA_Admin(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	bind(t, svc, 1, "admin")
	// 不存在的用户 → ErrNoUser
	err := svc.ResetMFA(context.Background(), 999)
	require.ErrorIs(t, err, ErrNoUser)

	err = svc.ResetMFA(context.Background(), 1)
	require.NoError(t, err)
	var enabled bool
	require.NoError(t, svc.db.Table("sys_users").Select("mfa_enabled").Where("id = 1").Scan(&enabled).Error)
	assert.False(t, enabled, "管理员重置必须关闭 mfa")
	var cnt int64
	require.NoError(t, svc.db.Table("sys_mfa_recovery_code").Where("user_id = 1").Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt, "管理员重置必须清空恢复码")
}

func TestVerifyForLogin_DynamicCode(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	bind(t, svc, 1, "admin")

	// 错误动态码 → false
	ok, err := svc.VerifyForLogin(context.Background(), 1, "000000")
	require.NoError(t, err)
	assert.False(t, ok)

	// 正确动态码 → true
	code := currentCodeForUser(t, svc, 1)
	ok, err = svc.VerifyForLogin(context.Background(), 1, code)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyForLogin_RecoveryCode_OneTime(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	bu := bind(t, svc, 1, "admin")

	// 第一次使用 → true
	ok, err := svc.VerifyForLogin(context.Background(), 1, bu.recoveryCodes[0])
	require.NoError(t, err)
	assert.True(t, ok)
	// 同一恢复码重放 → false（一次性）
	ok, err = svc.VerifyForLogin(context.Background(), 1, bu.recoveryCodes[0])
	require.NoError(t, err)
	assert.False(t, ok, "恢复码必须一次性，重放要拒绝")
	// 另一个恢复码仍可用
	ok, err = svc.VerifyForLogin(context.Background(), 1, bu.recoveryCodes[1])
	require.NoError(t, err)
	assert.True(t, ok, "未使用的恢复码必须可用")
}

func TestVerifyForLogin_FeatureDisabled(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	svc.cfg.Enabled = false
	_, err := svc.VerifyForLogin(context.Background(), 1, "123456")
	require.ErrorIs(t, err, ErrDisabled, "功能关闭时二因素校验必须显式拒绝")
}

func TestStartEnroll_FeatureDisabled(t *testing.T) {
	svc, mr := newTestService(t)
	defer mr.Close()
	svc.cfg.Enabled = false
	_, _, err := svc.StartEnroll(context.Background(), 1, "admin")
	require.ErrorIs(t, err, ErrDisabled)
}
