package middleware

// R26 TOTP 双因素认证守卫（对齐 api-admin feature/mfa-totp 的 MfaRequiredGuard）。
//
// 挂载于 Auth 中间件链内（jwtServer 解析 claims 之后）：
//   - MfaPending=true（两段式登录中间态，5 分钟待验证 token）→ 一律拒绝访问
//     业务接口（401 MFA_PENDING）。注意 /mfa/verify 在 Auth 白名单内不经此链，
//     因此这里对 MfaPending 的拦截与 verify 放行不冲突。
//   - MustEnrollMfa=true（强制绑定但用户未绑定）→ 仅放行绑定相关接口
//     （/mfa/status|enroll|enable + 用户信息 + 菜单 + 登出），其余 409 引导
//     前端跳个人中心绑定（对齐参考实现 allowlist 语义）。
//   - 两 flag 均为 false（默认/未启用）→ 直接放行，零影响。

import (
	"context"
	"strings"

	kratosErrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

// mfaEnrollAllowList 强制绑定（MustEnrollMfa）状态下允许访问的接口 operation 后缀。
// 对齐参考实现 mfaEnrollAllowList（/user/mfa/status|enroll|enable、/user/getUserInfo、
// /menu/getMenu、/jwt/jsonInBlacklist）——exAdmin 对应 operation 映射。
var mfaEnrollAllowList = []string{
	"/api.admin.v1.SysMfa/Status",
	"/api.admin.v1.SysMfa/Enroll",
	"/api.admin.v1.SysMfa/Enable",
	"/api.admin.v1.SysUser/Auth",        // 用户信息（个人中心）
	"/api.admin.v1.SysUser/Logout",      // 登出
	"/api.admin.v1.Menus/QueryMenusTree", // 动态路由/菜单加载
}

// MfaRequiredGuard 当 jwt 携带 MfaPending / MustEnrollMfa 时执行守卫。
func MfaRequiredGuard() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			claims, err := authz.FromContext(ctx)
			if err != nil {
				// 理论不会发生（jwtServer 已注入；claims 缺失由上层 401 拦截）
				return handler(ctx, req)
			}
			// MfaPending：两段式登录中间态，拒绝访问业务接口
			if claims.MfaPending {
				return nil, kratosErrors.New(401, "MFA_PENDING", "请先完成二因素验证")
			}
			// MustEnrollMfa：强制绑定但未绑定，仅放行绑定相关接口
			if claims.MustEnrollMfa {
				op := operation(ctx)
				for _, allow := range mfaEnrollAllowList {
					if strings.HasSuffix(op, allow) {
						return handler(ctx, req)
					}
				}
				return nil, kratosErrors.New(409, "MFA_ENROLL_REQUIRED", "请先绑定二因素认证")
			}
			return handler(ctx, req)
		}
	}
}

// operation 从 context 取 gRPC/HTTP operation（/service/method）
func operation(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}