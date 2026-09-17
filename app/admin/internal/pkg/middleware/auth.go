package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	jwtV5 "github.com/golang-jwt/jwt/v5"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

func AuthWhiteListMatcher() selector.MatchFunc {
	whiteList := make(map[string]struct{})
	whiteList["/api.admin.v1.SysUser/Login"] = struct{}{}
	whiteList["/api.admin.v1.SysUser/FindCaptcha"] = struct{}{}
	whiteList["/api.admin.v1.SysUser/FindPostInit"] = struct{}{}
	whiteList["/api.admin.v1.SysMfa/Verify"] = struct{}{}
	return func(ctx context.Context, operation string) bool {
		if _, ok := whiteList[operation]; ok {
			return false
		}
		return true
	}
}

// jwtServer 自定义 JWT 解析中间件（kratos v3 已移除 middleware/auth/jwt）。
// 从 transport 请求头取 Bearer token，按 HS256（阶段 3 将切换 ES384）校验后
// 注入自有 context key（authz.NewContext），下游 casbin/MFA 守卫统一从该 key 取 claims。
func jwtServer(keyFunc func(*jwtV5.Token) (interface{}, error)) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errors.Unauthorized("TOKEN_INVALID", "transport missing")
			}
			authHeader := tr.RequestHeader().Get("Authorization")
			tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || tokenStr == "" {
				return nil, errors.Unauthorized("TOKEN_INVALID", "token invalid")
			}
			token, err := jwtV5.ParseWithClaims(tokenStr, &authz.TokenClaims{}, keyFunc)
			if err != nil {
				return nil, errors.Unauthorized("TOKEN_INVALID", "token invalid")
			}
			claims, ok := token.Claims.(*authz.TokenClaims)
			if !ok || !token.Valid {
				return nil, errors.Unauthorized("TOKEN_INVALID", "token invalid")
			}
			ctx = authz.NewContext(ctx, claims)
			return handler(ctx, req)
		}
	}
}

func Auth(s *conf.Auth, repo admin.CasbinRuleRepo, userRepo admin.SysUserRepo) middleware.Middleware {
	// 解析验证侧 ECDSA P-384 公钥（ES384）；解析失败 pub 为 nil，
	// ES384Keyfunc 对 nil 公钥 fail-closed（验签全部失败），避免缺配置误放行。
	pub, pubErr := authz.ParseECDSAPublicKey(s.EcdsaPublicKey)
	if pubErr != nil {
		log.Errorf("parse ecdsa public key: %v", pubErr)
	}
	keyFunc := authz.ES384Keyfunc(pub)
	return selector.Server(
		jwtServer(keyFunc),
		// R26 TOTP：MfaPending 拒绝访问业务接口 + MustEnrollMfa 强制绑定守卫
		MfaRequiredGuard(),
		// JWT 黑名单和 IP 黑名单检查中间件
		func(handler middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req interface{}) (interface{}, error) {
				// 获取客户端 IP（仅 HTTP；gRPC 无 IP 黑名单语义）
				clientIP := ""
				if httpReq, ok := kratoshttp.RequestFromServerContext(ctx); ok {
					clientIP = getClientIP(httpReq)
				}

				// 检查 IP 是否在黑名单中
				if clientIP != "" {
					inBlacklist, err := userRepo.IsIpInBlacklist(ctx, clientIP)
					if err != nil {
						log.Errorf("Failed to check IP blacklist: %v", err)
					} else if inBlacklist {
						return nil, errors.Forbidden("IP_BLACKLISTED", "您的IP已被封禁")
					}
				}

				// 检查 JWT 是否在黑名单中（HTTP 头 / gRPC 元数据统一走 transport）
				rawToken := ""
				if tr, ok := transport.FromServerContext(ctx); ok {
					if a := tr.RequestHeader().Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
						rawToken = strings.TrimPrefix(a, "Bearer ")
					}
				}
				if rawToken != "" {
					inBlacklist, err := userRepo.IsJwtInBlacklist(ctx, rawToken)
					if err != nil {
						log.Errorf("Failed to check JWT blacklist: %v", err)
					} else if inBlacklist {
						return nil, errors.Unauthorized("JWT_BLACKLISTED", "Token已被撤销")
					}
				}

				return handler(ctx, req)
			}
		},
		// Casbin 权限校验中间件（自定义，替代 kratos-casbin 的 casbin.Server——
		// 后者类型绑定 kratos v2 中间件，v3 无法直接装配）。
		// 策略缺失时超级管理员（role_key=admin）旁路放行，避免空策略锁死系统。
		func(handler middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req interface{}) (interface{}, error) {
				claims, err := authz.FromContext(ctx)
				if err != nil {
					return nil, errors.Unauthorized("CLAIMS_MISS", "claims miss")
				}
				tr, ok := transport.FromServerContext(ctx)
				if !ok {
					return nil, errors.Forbidden("FORBIDDEN", "forbidden")
				}
				obj := tr.Operation()
				act := ""
				if ht, ok := tr.(kratoshttp.Transporter); ok {
					act = ht.Request().Method
				}
				allowed, err := repo.Enforce(claims.RoleKey, obj, act)
				if err != nil {
					log.Errorf("casbin enforce error: %v", err)
					return nil, errors.Forbidden("FORBIDDEN", "forbidden")
				}
				if !allowed {
					return nil, errors.Forbidden("FORBIDDEN", "无权限访问")
				}
				return handler(ctx, req)
			}
		},
	).Match(AuthWhiteListMatcher()).Build()
}
