package admin

// MfaUseCase TOTP 双因素认证用例。
//
//   - Status/Enroll/Enable/Disable 面向本人（uid 从 JWT claims 取）
//   - Reset 面向管理员（重置目标用户）
//   - Verify 两段式登录二次验证：解析 MfaPending token → 校验动态码/恢复码
//     → 签发正式 token
//
// 开关/加密/恢复码逻辑全部在 pkg/mfa（本文件为薄适配层）。

import (
	"context"
	"crypto/ecdsa"
	"time"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
)

// MfaUseCase TOTP 用例
type MfaUseCase struct {
	// ecdsaKey 签发/验签侧 ECDSA P-384 私钥（Verify 解析 MfaPending token 用
	// 其公钥、签发正式 token 用私钥——同一把密钥对，与 authz 同源）。
	ecdsaKey *ecdsa.PrivateKey
	expire   time.Duration
	mfaSvc   *mfa.Service
	userRepo SysUserRepo // Verify 通过后加载用户确认状态/角色（签发正式 token 需要）
	roleRepo SysRoleRepo
	log      *logx.Logger
}

// NewMfaUseCase 创建 MFA 用例（解析签发侧私钥，配置缺失 fail-closed）
func NewMfaUseCase(authConf *conf.Auth, mfaSvc *mfa.Service, userRepo SysUserRepo, roleRepo SysRoleRepo, logger *logx.Logger) (*MfaUseCase, error) {
	priv, err := authz.ParseECDSAPrivateKey(authConf.EcdsaPrivateKey)
	if err != nil {
		return nil, err
	}
	return &MfaUseCase{
		ecdsaKey: priv,
		expire:   authConf.Expires.AsDuration(),
		mfaSvc:   mfaSvc,
		userRepo: userRepo,
		roleRepo: roleRepo,
		log:      logger,
	}, nil
}

// Status 查询当前用户 TOTP 状态
func (uc *MfaUseCase) Status(ctx context.Context, uid int64) (*mfa.Status, error) {
	st, err := uc.mfaSvc.Status(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// Enroll 生成 TOTP 密钥与 otpauth URI（uid 归属校验在 service 层）
func (uc *MfaUseCase) Enroll(ctx context.Context, uid int64, username string) (secret, url string, err error) {
	return uc.mfaSvc.StartEnroll(ctx, uid, username)
}

// Enable 校验动态码并启用，返回一次性恢复码
func (uc *MfaUseCase) Enable(ctx context.Context, uid int64, code string) ([]string, error) {
	return uc.mfaSvc.ConfirmEnable(ctx, uid, code)
}

// Disable 关闭 TOTP（需动态码或恢复码二次确认）
func (uc *MfaUseCase) Disable(ctx context.Context, uid int64, code string) error {
	return uc.mfaSvc.Disable(ctx, uid, code)
}

// Reset 管理员重置指定用户 TOTP
func (uc *MfaUseCase) Reset(ctx context.Context, targetUID int64) error {
	return uc.mfaSvc.ResetMFA(ctx, targetUID)
}

// Verify 两段式登录二次验证：
//  1. 解析 mfaToken（MfaPending claim），失败/非 pending → 拒绝（验证会话失效）
//  2. mfaSvc.VerifyForLogin 校验动态码或恢复码
//  3. 通过后重新加载用户（确认未被停用）→ 签发正式 token
func (uc *MfaUseCase) Verify(ctx context.Context, mfaToken, code string) (*pb.LoginReply, error) {
	claims, err := authz.ParseToken(mfaToken, &uc.ecdsaKey.PublicKey)
	if err != nil {
		return nil, pb.ErrorMfaVerifyFailed("验证会话已过期，请重新登录")
	}
	if !claims.MfaPending {
		return nil, pb.ErrorMfaVerifyFailed("无效的验证会话")
	}

	ok, err := uc.mfaSvc.VerifyForLogin(ctx, claims.UserID, code)
	if err != nil || !ok {
		uc.log.Warnw(ctx, "二因素校验失败", "uid", claims.UserID)
		return nil, pb.ErrorMfaVerifyFailed("动态验证码错误或已过期")
	}

	user, err := uc.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		uc.log.Errorw(ctx, "二因素验证后加载用户失败", "uid", claims.UserID, "error", err)
		return nil, pb.ErrorMfaVerifyFailed("登录失败，请稍后重试")
	}
	if user.Status != 1 {
		return nil, pb.ErrorAccountForbidden("用户被禁止登录")
	}
	role, err := uc.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	expire := time.Now().Add(uc.expire)
	token, err := authz.NewToken(uc.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if err != nil {
		return nil, pb.ErrorMfaVerifyFailed("登录失败，请稍后重试")
	}
	return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
}
