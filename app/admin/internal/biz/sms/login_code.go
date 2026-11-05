package sms

// 登录二因素短信码（sys_users.login_method=2 手机验证码短信）。
//
// 与「手机验证码登录」（config message.phoneLogin.cachePrefix、按 **手机号** 维度）刻意隔离：
// 本流程按 **userID** 维度缓存（mfa:sms:<uid>），且密码已正确才会发码，避免两条流程的
// 验证码互相串用；校验成功即删除（一次性），与 VerifyCode 同口径。
//
// 通道依赖：复用 sys_sms_gateway（feature_id=201）/ config message.providers 兜底；
// 未配置网关或 message.enabled=false 时 SendSmsCodeOnly 直接返回错误 ⇒ 登录侧 fail-closed
// 拒绝 login_method=2 的用户（不会降级为无验证码的单因素登录）。

import (
	"context"
	"fmt"
	"time"
)

// LoginCodeTTL 登录二因素短信码有效期兜底（配置 phoneLogin.expireMinutes 缺省时使用）
const LoginCodeTTL = 5 * time.Minute

// LoginCodeCachePrefix 登录二因素短信码缓存键前缀（userID 维度，与手机号维度的 phone:login: 隔离）
const LoginCodeCachePrefix = "mfa:sms:"

// loginCodeKey 登录短信码缓存键（userID 维度）
func loginCodeKey(userID int64) string {
	return fmt.Sprintf("%s%d", LoginCodeCachePrefix, userID)
}

// SendLoginCode 生成并发送登录短信验证码（login_method=2 两段式登录第 1 步之后调用）。
// 发送失败直接返回错误：调用方必须拒绝签发待验证 token（否则用户拿不到码，流程不可完成）。
func (uc *UseCase) SendLoginCode(ctx context.Context, userID int64, phone string) error {
	length := int32(6)
	expireMinutes := int32(LoginCodeTTL / time.Minute)
	if cfg := uc.GetPhoneLoginConfig(); cfg != nil {
		if cfg.GetCodeLength() > 0 {
			length = cfg.GetCodeLength()
		}
		if cfg.GetExpireMinutes() > 0 {
			expireMinutes = cfg.GetExpireMinutes()
		}
	}
	code := uc.GenerateCode(length, "numeric")
	if err := uc.SendSmsCodeOnly(ctx, phone, code); err != nil {
		return err
	}
	if err := uc.SaveCode(ctx, loginCodeKey(userID), code, expireMinutes); err != nil {
		uc.log.Errorw(ctx, "msg", "登录短信验证码缓存失败", "userID", userID, "error", err)
		return err
	}
	return nil
}

// VerifyLoginCode 校验并消费登录短信验证码（成功即删除，防重放）
func (uc *UseCase) VerifyLoginCode(ctx context.Context, userID int64, code string) (bool, error) {
	if code == "" {
		return false, nil
	}
	return uc.VerifyCode(ctx, loginCodeKey(userID), code)
}
