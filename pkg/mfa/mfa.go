package mfa

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
	"gorm.io/gorm"
)

// 常量（对齐参考实现 sys_mfa.go）
const (
	// enrollPrefix 待确认绑定的 TOTP 密钥缓存键前缀
	enrollPrefix = "Mfa:Enroll:"
	// EnrollTTL 待确认绑定有效期
	EnrollTTL = 5 * time.Minute
	// PendingTTL 二因素待验证 token 有效期（由 authz 层签发时使用）
	PendingTTL = 5 * time.Minute
	// Skew TOTP 允许的时间漂移窗口（前后各 skew 个周期）
	Skew = 1
)

// 错误定义（对齐参考实现）
var (
	ErrAlreadyEnabled = errors.New("该用户已开启二因素认证")
	ErrNotEnabled     = errors.New("该用户未开启二因素认证")
	ErrCodeInvalid    = errors.New("动态验证码错误或已过期")
	ErrEnrollExpired  = errors.New("绑定会话已过期，请重新生成密钥")
	ErrRecoveryUsed   = errors.New("恢复码错误或已使用")
	ErrNoUser         = errors.New("用户不存在")
	ErrDisabled       = errors.New("二因素认证未开启")
)

// FeatureEnabled 二因素认证总开关（配置 mfa.enabled，默认关闭）。
// 关闭后登录不要求二因素，所有管理接口不可用。
func (c Config) FeatureEnabled() bool { return c.Enabled }

// Required 二因素强制全员开启判定（总开关开启且配置 required=true）
func (c Config) ForceRequired() bool { return c.Enabled && c.Required }

// UserMFA sys_users MFA 三列投影（端口读写共用；BoundAt=NULL 表示未绑定/已禁用）
type UserMFA struct {
	Enabled bool
	Secret  string
	BoundAt *time.Time
}

// MFAUserStore sys_users MFA 列读写端口。实现位于应用侧
// （app/admin/internal/data/mfastore，gen 字段对象白名单——UNIFIED §5.4
// 「指定列（含写零值）」形态，先例 sys_role_repo.go Update）；pkg 层不直触
// 数据库、不手写列名/字段名串（旧 mfaRow+字符串白名单方案废弃）。
type MFAUserStore interface {
	// Under 返回绑定 tx 会话的 store（Enable/Disable 与恢复码读写同事务）
	Under(tx *gorm.DB) MFAUserStore
	// GetMFA 读取用户 MFA 三列；用户不存在（含软删）found=false（调用方归 ErrNoUser）
	GetMFA(ctx context.Context, userID int64) (UserMFA, bool, error)
	// SetMFA 三列全量覆写（白名单强制写零值/NULL：禁用=Enabled false/Secret ""/BoundAt NULL，
	// 与旧 map 语义等价）；返回影响行数
	SetMFA(ctx context.Context, userID int64, u UserMFA) (rowsAffected int64, err error)
}

// Service TOTP 双因素认证服务（依赖注入模式，对齐 kratos 架构；
// 参考实现用 global 包，两仓改为构造注入 store/db/rdb/cfg/log）。
type Service struct {
	// store sys_users MFA 三列读写（gen 字段对象，实现=data/mfastore）
	store MFAUserStore
	// db 仅用于 sys_mfa_recovery_code 恢复码读写（model 式 CRUD，无列名白名单串）
	db  *gorm.DB
	rdb redis.UniversalClient
	cfg Config
	log *logx.Logger
}

// NewService 创建 MFA 服务
func NewService(store MFAUserStore, db *gorm.DB, rdb redis.UniversalClient, cfg Config, logger *logx.Logger) *Service {
	return &Service{store: store, db: db, rdb: rdb, cfg: cfg, log: logger}
}

// Config 返回当前配置（供 authz/中间件查询开关）
func (s *Service) Config() Config { return s.cfg }

// issuer 返回 TOTP 发行者名，缺省 BMALL
func (s *Service) issuer() string {
	if v := strings.TrimSpace(s.cfg.Issuer); v != "" {
		return v
	}
	return "BMALL"
}

// Status 查询用户 TOTP 开启状态
type Status struct {
	// Available 功能总开关（配置 mfa.enabled），关闭时前端隐藏二因素入口
	Available bool       `json:"available"`
	Enabled   bool       `json:"enabled"`
	BoundAt   *time.Time `json:"boundAt"`
}

// Status 查询用户 TOTP 开启状态
func (s *Service) Status(ctx context.Context, userID int64) (Status, error) {
	if !s.cfg.FeatureEnabled() {
		return Status{Available: false}, nil
	}
	u, found, err := s.store.GetMFA(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	if !found {
		return Status{}, ErrNoUser
	}
	return Status{Available: true, Enabled: u.Enabled, BoundAt: u.BoundAt}, nil
}

// StartEnroll 生成 TOTP 密钥与 otpauth URI，临时缓存待确认，不落库。
// secret 仅返回给当前用户用于扫码绑定，ConfirmEnable 校验通过后才加密落库。
func (s *Service) StartEnroll(ctx context.Context, userID int64, username string) (secret, url string, err error) {
	if !s.cfg.FeatureEnabled() {
		return "", "", ErrDisabled
	}
	u, found, err := s.store.GetMFA(ctx, userID)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", ErrNoUser
	}
	if u.Enabled {
		return "", "", ErrAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer(),
		AccountName: username,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
		SecretSize:  20,
	})
	if err != nil {
		return "", "", err
	}
	// 待确认密钥缓存（Redis，TTL 5min）
	err = s.rdb.Set(ctx, enrollPrefix+fmt.Sprint(userID), key.Secret(), EnrollTTL).Err()
	if err != nil {
		return "", "", fmt.Errorf("缓存绑定会话失败: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ConfirmEnable 校验用户提交的动态码，通过后将待确认密钥加密落库，并生成一次性恢复码。
// 返回的明文恢复码仅本次返回，库中仅存 bcrypt 哈希。
func (s *Service) ConfirmEnable(ctx context.Context, userID int64, code string) (recoveryCodes []string, err error) {
	if !s.cfg.FeatureEnabled() {
		return nil, ErrDisabled
	}
	secret, err := s.rdb.Get(ctx, enrollPrefix+fmt.Sprint(userID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrEnrollExpired
		}
		return nil, err
	}
	if secret == "" {
		s.rdb.Del(ctx, enrollPrefix+fmt.Sprint(userID))
		return nil, ErrEnrollExpired
	}
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      Skew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil || !valid {
		return nil, ErrCodeInvalid
	}

	enc, err := EncryptSecret(secret, []byte(s.cfg.EncryptionKey))
	if err != nil {
		s.log.Errorw(ctx, "加密 TOTP 密钥失败", "userID", userID, "error", err)
		return nil, errors.New("开启二因素认证失败，请稍后重试")
	}
	recoveryCodes, err = GenerateRecoveryCodes(RecoveryCodeCount)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新 sys_users 的 mfa 三列（gen 字段白名单 store，绑定本事务会话，
		// 与恢复码清理/写入原子提交）
		affected, err := s.store.Under(tx).SetMFA(ctx, userID, UserMFA{Enabled: true, Secret: enc, BoundAt: &now})
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNoUser
		}
		// 清理旧恢复码后写入新恢复码
		if err := tx.Where("user_id = ?", userID).
			Delete(&SysMfaRecoveryCode{}).Error; err != nil {
			return err
		}
		rows := make([]SysMfaRecoveryCode, 0, len(recoveryCodes))
		for _, rc := range recoveryCodes {
			rows = append(rows, SysMfaRecoveryCode{
				UserID:   userID,
				CodeHash: util.BcryptHash(rc),
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	s.rdb.Del(ctx, enrollPrefix+fmt.Sprint(userID))
	return recoveryCodes, nil
}

// Disable 关闭 TOTP，需提供当前动态码或有效恢复码进行二次确认
func (s *Service) Disable(ctx context.Context, userID int64, code string) error {
	if !s.cfg.FeatureEnabled() {
		return ErrDisabled
	}
	u, found, err := s.store.GetMFA(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNoUser
	}
	if !u.Enabled {
		return ErrNotEnabled
	}
	ok, err := s.verify(ctx, userID, u.Secret, code)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCodeInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.store.Under(tx).SetMFA(ctx, userID, UserMFA{}); err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&SysMfaRecoveryCode{}).Error
	})
}

// ResetMFA 管理员重置指定用户的 TOTP，清空密钥、关闭开关并删除恢复码
func (s *Service) ResetMFA(ctx context.Context, targetUserID int64) error {
	if !s.cfg.FeatureEnabled() {
		return ErrDisabled
	}
	affected, err := s.store.SetMFA(ctx, targetUserID, UserMFA{})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNoUser
	}
	return s.db.WithContext(ctx).Where("user_id = ?", targetUserID).
		Delete(&SysMfaRecoveryCode{}).Error
}

// VerifyForLogin 登录二因素校验，校验动态码或恢复码，通过返回 true
func (s *Service) VerifyForLogin(ctx context.Context, userID int64, code string) (bool, error) {
	if !s.cfg.FeatureEnabled() {
		return false, ErrDisabled
	}
	u, found, err := s.store.GetMFA(ctx, userID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, ErrNoUser
	}
	if !u.Enabled {
		return false, ErrNotEnabled
	}
	return s.verify(ctx, userID, u.Secret, code)
}

// verify 校验动态码或恢复码；恢复码命中后标记已用
func (s *Service) verify(ctx context.Context, userID int64, encSecret, code string) (bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return false, nil
	}
	secret, err := DecryptSecret(encSecret, []byte(s.cfg.EncryptionKey))
	if err != nil {
		s.log.Errorw(ctx, "解密 TOTP 密钥失败", "userID", userID, "error", err)
		return false, errors.New("二因素校验失败，请联系管理员")
	}
	ok, vErr := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      Skew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if vErr == nil && ok {
		return true, nil
	}
	// 回退到恢复码
	return s.consumeRecoveryCode(ctx, userID, code)
}

// consumeRecoveryCode 查找并标记一个可用恢复码
func (s *Service) consumeRecoveryCode(ctx context.Context, userID int64, code string) (bool, error) {
	var codes []SysMfaRecoveryCode
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL", userID).
		Find(&codes).Error
	if err != nil {
		return false, err
	}
	now := time.Now()
	for i := range codes {
		if util.BcryptCheck(code, codes[i].CodeHash) {
			// R28-AUTH-02：条件更新 CAS（主键 + 用户归属 + used_at IS NULL），
			// 仅 RowsAffected=1 判定成功——并发请求读到同一行时只有一个赢家，
			// 一次性恢复码只被消费一次（败者拒绝，不可重放）。
			res := s.db.WithContext(ctx).Model(&SysMfaRecoveryCode{}).
				Where("id = ? AND user_id = ? AND used_at IS NULL", codes[i].ID, userID).
				Update("used_at", now)
			if res.Error != nil {
				return false, res.Error
			}
			if res.RowsAffected != 1 {
				return false, nil
			}
			return true, nil
		}
	}
	return false, nil
}
