package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/sensitive"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

// SensitiveRevealChecker 敏感信息明文查看权限判定（casbin 虚拟权限点 enforce）。
// 注入实现来自 server/http.go 装配闭包（casbinRepo.Enforce(roleKey, sensitive.ViewPermissionPath, "GET")）。
type SensitiveRevealChecker func(ctx context.Context, roleKey string) (bool, error)

// SensitiveInfoGuard 敏感信息可见性中间件（Kratos middleware.Middleware，全局挂载于 Auth 之后）。
//
// 对齐 go-exAdmin app/admin/internal/pkg/middleware/sensitive_guard.go 的判定语义：
//   - 超管角色（role_key=admin）旁路（authz.IsSuperAdmin；casbin repo 内部亦有同款旁路）；
//   - 非 admin 角色按虚拟权限点 sensitive.ViewPermissionPath 经 casbin enforce 判定
//     （角色管理 UI 从「API 管理」勾选授权，种子见 deploy/sql/20261105_sensitive_view_api.sql）；
//   - 判定结果注入 ctx（sensitive.WithReveal），响应构建点经 sensitive.CanReveal(ctx) 读取；
//   - 判定故障 fail-closed（按未授权处理，不阻断业务读接口），并告警留痕。
//
// 与 exAdmin 的差异（Kratos 适配）：exAdmin 另有「全局缓冲响应体正则脱敏」兜底层，
// Kratos 中间件拿不到 ResponseWriter、无法等价实现；本仓以响应构建点结构化脱敏 +
// 掩码回环保护（更新入口含 '*' 保持原值）达到同等防护目标。
func SensitiveInfoGuard(checker SensitiveRevealChecker) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			allowed := false
			if claims, err := authz.FromContext(ctx); err == nil && claims != nil {
				if authz.IsSuperAdmin(claims.RoleKey) {
					allowed = true
				} else if checker != nil {
					ok, cErr := checker(ctx, claims.RoleKey)
					if cErr != nil {
						// fail-closed：按未授权处理（与 exAdmin 引擎不可用返回 false 同口径）
						log.Errorf("SensitiveInfoGuard: checker failed (fail-closed), roleKey=%s, op=%s, error=%v",
							claims.RoleKey, transportOperation(ctx), cErr)
					} else {
						allowed = ok
					}
				}
			}
			return handler(sensitive.WithReveal(ctx, allowed), req)
		}
	}
}

// transportOperation 当前请求的 gRPC operation 全限定名（日志定位用；可空）。
func transportOperation(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}
