// Package sensitive 敏感信息按权限可见（2026-11-04，对齐 go-exAdmin app/admin/internal/pkg/sensitive）。
//
// 设计要点（与 exAdmin 逐条同构）：
//   - 权限点 = 一条虚拟 casbin 策略（ViewPermissionPath，无真实路由），
//     经「API 管理 + 角色分配」勾选授权，超管角色（role_key=admin）旁路；
//   - 授权者明文、未授权者掩码：**无 reveal 二次接口**（明文可见性=授权本身，
//     避免二次授权接口攻击面）；
//   - 判定由 SensitiveInfoGuard 中间件在 Auth 之后解析一次并注入 ctx，
//     响应构建点（service 层）经 CanReveal(ctx) 读取，决定明文或掩码；
//   - 判定故障 fail-closed（按无权限处理，不阻断业务读接口）。
//
// 掩码回环保护：编辑表单可能回填掩码值并随更新提交——各更新入口须按
// 「值含 '*' 即保持原值」处理（用 HasMask 判定，见 service 层 UpdateXxx）。
package sensitive

import (
	"context"
	"strings"
)

// ViewPermissionPath 敏感信息明文查看的虚拟权限点（casbin obj；无真实路由）。
// 种子见 deploy/sql/20261105_sensitive_view_api.sql（sys_apis 行 + 角色按需勾选）。
const ViewPermissionPath = "/sensitive/view"

// ViewPermissionMethod 虚拟权限点的 casbin act（与 API 管理页登记一致）。
const ViewPermissionMethod = "GET"

// ctxKey 私有 context key（防外部伪造注入）。
type ctxKey struct{}

// WithReveal 在 ctx 中注入「本次请求是否允许查看敏感明文」的判定结果。
// 仅由 SensitiveInfoGuard 中间件调用。
func WithReveal(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, ctxKey{}, allowed)
}

// CanReveal 当前请求是否允许查看敏感明文（未注入=未授权，fail-closed）。
func CanReveal(ctx context.Context) bool {
	v, ok := ctx.Value(ctxKey{}).(bool)
	return ok && v
}

// HasMask 值中是否已含掩码字符 '*'（更新入口回环保护判定）。
// 前端把掩码值原样回填到编辑表单再提交时，据此「视为未修改」。
func HasMask(v string) bool {
	return strings.Contains(v, "*")
}

// ResolveMasked 更新入口掩码回环保护：提交值含 '*' 视为未修改，返回库中原值；
// 否则返回提交值。各 Update 入口对「响应面被掩码过的字段」统一走此函数。
func ResolveMasked(submitted, original string) string {
	if HasMask(submitted) {
		return original
	}
	return submitted
}

// MaskIDCard 身份证号掩码：保留前 3 后 4（110**********1234）；长度不足 8 全掩码。
func MaskIDCard(v string) string {
	return maskKeepEnds(v, 3, 4)
}

// MaskMobile 手机号掩码：保留前 3 后 4（138****0000）；长度不足 8 全掩码。
func MaskMobile(v string) string {
	return maskKeepEnds(v, 3, 4)
}

// MaskBankCard 银行卡号掩码：保留前 4 后 4（6222********1234）；长度不足 9 全掩码。
func MaskBankCard(v string) string {
	return maskKeepEnds(v, 4, 4)
}

// MaskPhoneLike 对「看起来是手机号」的值按手机号掩码，其他形态原样返回。
//
// 用于账户名列（如壁纸线 w_user_base.account）：该列手机号注册时为 11 位手机号，
// 微信/Apple 注册时为 openid/昵称等非手机号串；无条件掩码会把非 PII 的标识串
// 也打碎（运营无法区分账号）。故按值判定：11 位数字且以 1 开头才掩码。
func MaskPhoneLike(v string) string {
	if len(v) != 11 || v[0] != '1' {
		return v
	}
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return v
		}
	}
	return MaskMobile(v)
}

// maskKeepEnds 通用两端保留掩码；空串原样；过短（head+tail 放不下）整串替换为等长 '*'。
func maskKeepEnds(v string, head, tail int) string {
	if v == "" {
		return v
	}
	n := len([]rune(v))
	if n <= head+tail {
		return strings.Repeat("*", n)
	}
	r := []rune(v)
	return string(r[:head]) + strings.Repeat("*", n-head-tail) + string(r[n-tail:])
}
