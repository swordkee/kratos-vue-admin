package admin

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/common/constant"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
	"github.com/swordkee/kratos-vue-admin/pkg/util"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
)

type AuthUseCase struct {
	// ecdsaKey 签发侧 ECDSA P-384 私钥（ES384），由 conf.EcdsaPrivateKey 解析。
	ecdsaKey *ecdsa.PrivateKey
	expire   time.Duration
	userRepo SysUserRepo
	roleRepo SysRoleRepo
	// mfaSvc TOTP 双因素服务。nil 安全：未装配/未启用（mfa.enabled=false）
	// 时 Login 走原逻辑零影响；构造不查库，仅调用时按配置判定。
	mfaSvc *mfa.Service
	log    *logx.Logger
}

func NewAuthUseCase(conf *conf.Auth, userRepo SysUserRepo, roleRepo SysRoleRepo, mfaSvc *mfa.Service, logger *logx.Logger) (*AuthUseCase, error) {
	// 解析签发侧 ECDSA P-384 私钥；配置缺失或非法直接报错拒绝启动（fail-closed）。
	priv, err := authz.ParseECDSAPrivateKey(conf.EcdsaPrivateKey)
	if err != nil {
		return nil, err
	}
	return &AuthUseCase{
		ecdsaKey: priv,
		expire:   conf.Expires.AsDuration(),
		userRepo: userRepo,
		roleRepo: roleRepo,
		mfaSvc:   mfaSvc,
		log:      logger,
	}, nil
}

// Login 登录（TOTP 两段式）。
//   - mfa 未启用 / mfaSvc nil / 用户未绑定 → 直接签发正式 token（默认零影响）
//   - mfa 启用且用户已绑定 → 签发 5 分钟 MfaPending token，NeedMfa=true
//   - mfa 启用且 required=true 用户未绑定 → 签发 MustEnrollMfa token（守卫仅放行绑定接口，409 引导）
func (receiver *AuthUseCase) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	// 图形验证码校验（FindCaptcha 下发 captchaId+图片，登录随 code 上送；一次性，校验即失效）
	if !util.Verify(req.CaptchaId, req.Code) {
		return nil, pb.ErrorCaptchaInvalid("验证码错误或已过期")
	}
	user, err := receiver.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, pb.ErrorUserNotFound("用户名或密码错误")
	}
	if user.Status == constant.StatusUserForbidden {
		return nil, pb.ErrorAccountForbidden("账号被停用")
	}

	if !util.BcryptCheck(req.Password, user.Password) {
		return nil, pb.ErrorLoginFail(pkg.ErrPassword)
	}

	role, err := receiver.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	// TOTP：mfa 启用时按用户绑定态分流（nil mfaSvc / 未启用走原逻辑）
	if receiver.mfaSvc != nil && receiver.mfaSvc.Config().FeatureEnabled() {
		st, serr := receiver.mfaSvc.Status(ctx, user.ID)
		if serr != nil {
			// fail-closed：MFA 启用时绑定态读取失败必须拒绝登录，
			// 不得降级签发无 MFA 限制标记的普通令牌（否则下游守卫被旁路）。
			return nil, pb.ErrorLoginFail("查询 MFA 状态失败，请稍后重试")
		}
		if st.Enabled {
			mfaToken, terr := authz.NewMFAPendingToken(
				receiver.ecdsaKey, mfa.PendingTTL, user.ID, user.RoleID, role.RoleKey, user.NickName)
			if terr != nil {
				return nil, pb.ErrorLoginFail("generate mfa token failed: %s", terr.Error())
			}
			return &pb.LoginReply{
				NeedMfa:  true,
				MfaToken: mfaToken,
				Expire:   time.Now().Add(mfa.PendingTTL).Unix(),
			}, nil
		}
		if receiver.mfaSvc.Config().ForceRequired() {
			expire := time.Now().Add(receiver.expire)
			token, terr := authz.NewMustEnrollToken(
				receiver.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
			if terr != nil {
				return nil, pb.ErrorLoginFail("generate token failed: %s", terr.Error())
			}
			return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
		}
		// 未绑定且非强制：fall through 签发正式 token
	}

	expire := time.Now().Add(receiver.expire)
	token, err := authz.NewToken(receiver.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if err != nil {
		return nil, pb.ErrorLoginFail("generate token failed: %s", err.Error())
	}
	return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
}
