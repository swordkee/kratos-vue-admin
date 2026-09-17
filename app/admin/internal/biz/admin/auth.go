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
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
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

// Login 密码登录（TOTP 两段式分流 + 图形验证码在 service 入口校验）。
func (receiver *AuthUseCase) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
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
	return receiver.issueLoginReply(ctx, user, role)
}

// PhoneLogin 手机验证码登录（短信码在 service 入口已校验），MFA 分流与密码登录一致。
func (receiver *AuthUseCase) PhoneLogin(ctx context.Context, phone string) (*pb.LoginReply, error) {
	user, err := receiver.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, pb.ErrorUserNotFound("用户不存在")
	}
	if user.Status == constant.StatusUserForbidden {
		return nil, pb.ErrorAccountForbidden("账号被停用")
	}
	role, err := receiver.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	return receiver.issueLoginReply(ctx, user, role)
}

// issueLoginReply 按 MFA 绑定态签发登录响应：
//   - mfa 未启用 / nil / 用户未绑定 → 正式 token（默认零影响）
//   - 启用且已绑定 → 5 分钟 MfaPending token，NeedMfa=true
//   - 启用且 required 未绑定 → MustEnrollMfa token（守卫仅放行绑定接口，409 引导）
//
// 绑定态读取失败 fail-closed，拒绝降级签发无限制 token（R28-AUTH-01）。
func (receiver *AuthUseCase) issueLoginReply(ctx context.Context, user *model.SysUsers, role *model.SysRoles) (*pb.LoginReply, error) {
	if receiver.mfaSvc != nil && receiver.mfaSvc.Config().FeatureEnabled() {
		st, serr := receiver.mfaSvc.Status(ctx, user.ID)
		if serr != nil {
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
	}

	expire := time.Now().Add(receiver.expire)
	token, err := authz.NewToken(receiver.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if err != nil {
		return nil, pb.ErrorLoginFail("generate token failed: %s", err.Error())
	}
	return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
}
