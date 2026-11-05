package admin

import (
	"context"
	"crypto/ecdsa"
	"strings"
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

// LoginMethodTOTP / LoginMethodSMS sys_users.login_method 取值（用户级二因素强制方式）。
// 1=TOTP 动态验证码（默认，存量用户全部按此）；2=手机验证码短信。
const (
	LoginMethodTOTP int32 = 1
	LoginMethodSMS  int32 = 2
)

// SmsLoginCodeRepo 手机验证码登录（login_method=2）的短信码发送/校验端口。
//
// 实现位于 biz/sms（Redis 缓存 mfa:sms:<uid> + sys_sms_gateway 网关），biz 侧只依赖该窄接口
// 便于单测；未装配短信网关时注入 nil 实现即可——此时 login_method=2 的用户一律拒绝登录
// （fail-closed，绝不降级为单因素）。接入短信网关后（config message.enabled + sys_sms_gateway
// 或 config message.providers）即自动可用，无需改本层代码。
type SmsLoginCodeRepo interface {
	// SendLoginCode 生成并发送一条登录短信验证码（userID 维度缓存，短 TTL，一次性）
	SendLoginCode(ctx context.Context, userID int64, phone string) error
	// VerifyLoginCode 校验并消费验证码（成功即删除，防重放）
	VerifyLoginCode(ctx context.Context, userID int64, code string) (bool, error)
}

type AuthUseCase struct {
	// ecdsaKey 签发侧 ECDSA P-384 私钥（ES384），由 conf.EcdsaPrivateKey 解析。
	ecdsaKey *ecdsa.PrivateKey
	expire   time.Duration
	userRepo SysUserRepo
	roleRepo SysRoleRepo
	// mfaSvc TOTP 双因素服务。nil 安全：未装配/未启用（mfa.enabled=false）
	// 时 Login 走原逻辑零影响；构造不查库，仅调用时按配置判定。
	mfaSvc *mfa.Service
	// smsSvc 短信登录码端口（login_method=2）。nil 安全：未装配时该登录方式拒绝。
	smsSvc SmsLoginCodeRepo
	// lockSvc 登录失败锁定（gva security_lock 迁入）。nil 安全：未装配时
	// 全部锁定判定退化为 no-op，登录行为与本批改动前一致。
	lockSvc *SecurityLockUseCase
	log     *logx.Logger
}

func NewAuthUseCase(conf *conf.Auth, userRepo SysUserRepo, roleRepo SysRoleRepo, mfaSvc *mfa.Service,
	smsSvc SmsLoginCodeRepo, lockSvc *SecurityLockUseCase, logger *logx.Logger) (*AuthUseCase, error) {
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
		smsSvc:   smsSvc,
		lockSvc:  lockSvc,
		log:      logger,
	}, nil
}

// Login 密码登录（按用户 login_method 分流 + 图形验证码在 service 入口校验，
// 失败计数/锁定在 biz 层按 gva 语义判定）。
func (receiver *AuthUseCase) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	// 1. 账号锁定检查（gva 登录第 1 步：锁定期间连密码都不校验）
	if locked, minutes := receiver.accountLocked(ctx, req.Username); locked {
		return nil, pb.ErrorLoginFail("账号已锁定，请 %d 分钟后再试", minutes)
	}
	user, err := receiver.userRepo.FindByUsername(ctx, req.Username)
	if err != nil {
		// gva 口径：用户名不存在与密码错误同样计入失败计数
		receiver.recordLoginFail(ctx, req.Username)
		return nil, pb.ErrorUserNotFound("用户名或密码错误")
	}
	if user.Status == constant.StatusUserForbidden {
		// gva 口径：停用账号不计失败（不是凭证失败）
		return nil, pb.ErrorAccountForbidden("账号被停用")
	}

	if !util.BcryptCheck(req.Password, user.Password) {
		receiver.recordLoginFail(ctx, req.Username)
		return nil, pb.ErrorLoginFail(pkg.ErrPassword)
	}

	role, err := receiver.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	reply, err := receiver.issueLoginReply(ctx, user, role)
	if err != nil {
		return nil, err
	}
	// gva 口径：只有完整登录成功才清失败计数（TOTP / 短信待验证阶段不清，
	// 由第二步校验成功后再清）。本仓两种二因素并存，故两个标志都要判。
	if !reply.GetNeedMfa() && !reply.GetNeedSms() {
		receiver.clearLoginFail(ctx, req.Username)
	}
	return reply, nil
}

// accountLocked 账号锁定判定（nil 安全）。
func (receiver *AuthUseCase) accountLocked(ctx context.Context, username string) (bool, int32) {
	if receiver.lockSvc == nil {
		return false, 0
	}
	return receiver.lockSvc.IsAccountLocked(ctx, username)
}

// recordLoginFail 记一次登录失败（配置关闭失败锁定时内部直接返回）。
func (receiver *AuthUseCase) recordLoginFail(ctx context.Context, username string) {
	if receiver.lockSvc == nil {
		return
	}
	receiver.lockSvc.RecordLoginFail(ctx, username)
}

// clearLoginFail 清失败计数与锁态。
func (receiver *AuthUseCase) clearLoginFail(ctx context.Context, username string) {
	if receiver.lockSvc == nil {
		return
	}
	receiver.lockSvc.ClearLoginFail(ctx, username)
}

// PhoneLogin 手机验证码登录（短信码在 service 入口已校验）。
//
// 登录方式收口（login_method 用户级强制二因素方式）：
//   - login_method=2（手机验证码）：短信码校验通过即满足手机因素，直接签发正式 token
//     （不再二次发码——第二次发码与第一次同因素，无附加安全性）。
//   - login_method=1（TOTP）：二因素是动态码，短信登录入口一律拒绝（不允许旁路 TOTP）。
func (receiver *AuthUseCase) PhoneLogin(ctx context.Context, phone string) (*pb.LoginReply, error) {
	user, err := receiver.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, pb.ErrorUserNotFound("用户不存在")
	}
	if user.Status == constant.StatusUserForbidden {
		return nil, pb.ErrorAccountForbidden("账号被停用")
	}
	if normalizeLoginMethod(user.LoginMethod) != LoginMethodSMS {
		return nil, pb.ErrorLoginFail("该用户登录方式为动态验证码（TOTP），不支持短信验证码登录，请使用密码登录")
	}
	role, err := receiver.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	return receiver.issueFullToken(ctx, user, role)
}

// normalizeLoginMethod 归一登录方式：只有显式 2 视为短信登录，其余（0 脏值/未知值）
// 一律按 TOTP 处理（fail-closed：零值落到更严的一侧）。
func normalizeLoginMethod(v int32) int32 {
	if v == LoginMethodSMS {
		return LoginMethodSMS
	}
	return LoginMethodTOTP
}

// issueLoginReply 按用户级 login_method 分流签发登录响应：
//   - login_method=2 → 短信待验证 token（NeedSms=true），发码在 biz 内完成
//   - login_method=1 → issueTOTPReply（TOTP 强制）
func (receiver *AuthUseCase) issueLoginReply(ctx context.Context, user *model.SysUsers, role *model.SysRoles) (*pb.LoginReply, error) {
	if normalizeLoginMethod(user.LoginMethod) == LoginMethodSMS {
		return receiver.issueSmsPending(ctx, user, role)
	}
	return receiver.issueTOTPReply(ctx, user, role)
}

// issueTOTPReply login_method=1（TOTP）的登录响应：
//   - 已绑定 → 5 分钟 MfaPending token，NeedMfa=true，第二步 /mfa/verify
//   - 未绑定 → 无条件 MustEnrollMfa token（**不再受 mfa.required 影响**：login_method=1
//     本身即「该用户强制 TOTP」，未绑定不允许绕过，只能先去个人中心绑定）
//
// 总闸口径：auth.mfa.enabled 是 TOTP **能力**总闸（关闭时加密密钥不校验、绑定/校验接口
// 全部禁用），此时对 login_method=1 的用户强制 TOTP 只会造成「既不能绑定也不能校验」的
// 死锁 ⇒ 降级为单因素登录并打 Error 日志留痕（不静默）。总闸打开后自动恢复强制。
func (receiver *AuthUseCase) issueTOTPReply(ctx context.Context, user *model.SysUsers, role *model.SysRoles) (*pb.LoginReply, error) {
	if receiver.mfaSvc == nil || !receiver.mfaSvc.Config().FeatureEnabled() {
		receiver.log.Errorw(ctx, "login_method=TOTP 但 auth.mfa.enabled=false：二因素能力未启用，本次降级为单因素登录",
			"uid", user.ID, "username", user.Username)
		return receiver.issueFullToken(ctx, user, role)
	}
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
	expire := time.Now().Add(receiver.expire)
	token, terr := authz.NewMustEnrollToken(
		receiver.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if terr != nil {
		return nil, pb.ErrorLoginFail("generate token failed: %s", terr.Error())
	}
	return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
}

// issueSmsPending login_method=2（手机验证码）的登录响应：先校验手机号与短信通道，
// 发送登录码后签发 5 分钟短信待验证 token（NeedSms=true），第二步 /system/user/phone/verify。
// 未绑定手机号 / 短信通道不可用 / 发送失败一律拒绝（fail-closed，不降级为单因素）。
func (receiver *AuthUseCase) issueSmsPending(ctx context.Context, user *model.SysUsers, role *model.SysRoles) (*pb.LoginReply, error) {
	if strings.TrimSpace(user.Phone) == "" {
		return nil, pb.ErrorLoginFail("该用户登录方式为手机验证码但未绑定手机号")
	}
	if receiver.smsSvc == nil {
		return nil, pb.ErrorLoginFail("短信验证码登录未启用，请联系管理员")
	}
	if err := receiver.smsSvc.SendLoginCode(ctx, user.ID, user.Phone); err != nil {
		receiver.log.Errorw(ctx, "发送登录短信验证码失败", "uid", user.ID, "error", err)
		return nil, pb.ErrorLoginFail("短信验证码发送失败，请稍后重试")
	}
	smsToken, terr := authz.NewSMSPendingToken(
		receiver.ecdsaKey, mfa.PendingTTL, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if terr != nil {
		return nil, pb.ErrorLoginFail("generate sms token failed: %s", terr.Error())
	}
	return &pb.LoginReply{
		NeedSms:  true,
		MfaToken: smsToken,
		Expire:   time.Now().Add(mfa.PendingTTL).Unix(),
	}, nil
}

// VerifySmsLogin 两段式登录第二步（login_method=2）：短信待验证 token + 短信码 → 正式 token。
// 防串道：只接受 SmsPending 中间态；登录方式若在两步之间被改成 TOTP，本次短信会话作废。
func (receiver *AuthUseCase) VerifySmsLogin(ctx context.Context, smsToken, code string) (*pb.LoginReply, error) {
	claims, err := authz.ParseToken(smsToken, &receiver.ecdsaKey.PublicKey)
	if err != nil {
		return nil, pb.ErrorLoginFail("验证会话已过期，请重新登录")
	}
	if !claims.MfaPending || !claims.SmsPending {
		return nil, pb.ErrorLoginFail("无效的验证会话")
	}
	user, err := receiver.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, pb.ErrorUserNotFound("用户不存在")
	}
	if user.Status == constant.StatusUserForbidden {
		return nil, pb.ErrorAccountForbidden("账号被停用")
	}
	if normalizeLoginMethod(user.LoginMethod) != LoginMethodSMS {
		return nil, pb.ErrorLoginFail("登录方式已变更，请重新登录")
	}
	if receiver.smsSvc == nil {
		return nil, pb.ErrorLoginFail("短信验证码登录未启用，请联系管理员")
	}
	ok, verr := receiver.smsSvc.VerifyLoginCode(ctx, user.ID, strings.TrimSpace(code))
	if verr != nil || !ok {
		receiver.log.Warnw(ctx, "短信登录码校验失败", "uid", user.ID)
		return nil, pb.ErrorLoginFail("短信验证码错误或已过期")
	}
	role, err := receiver.roleRepo.FindByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}
	return receiver.issueFullToken(ctx, user, role)
}

// issueFullToken 签发正式登录 token（单因素 / 二因素已完成两种场景共用）。
func (receiver *AuthUseCase) issueFullToken(ctx context.Context, user *model.SysUsers, role *model.SysRoles) (*pb.LoginReply, error) {
	expire := time.Now().Add(receiver.expire)
	token, err := authz.NewToken(receiver.ecdsaKey, expire, user.ID, user.RoleID, role.RoleKey, user.NickName)
	if err != nil {
		return nil, pb.ErrorLoginFail("generate token failed: %s", err.Error())
	}
	return &pb.LoginReply{Token: token, Expire: expire.Unix()}, nil
}
