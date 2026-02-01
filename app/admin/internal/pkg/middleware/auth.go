package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	jwtV5 "github.com/golang-jwt/jwt/v5"
	"github.com/swordkee/kratos-casbin/authz/casbin"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

func AuthWhiteListMatcher() selector.MatchFunc {
	whiteList := make(map[string]struct{})
	whiteList["/api.admin.v1.Sysuser/Login"] = struct{}{}
	whiteList["/api.admin.v1.Sysuser/GetCaptcha"] = struct{}{}
	whiteList["/api.admin.v1.TencentCallback/TencentCallback"] = struct{}{}
	return func(ctx context.Context, operation string) bool {
		if _, ok := whiteList[operation]; ok {
			return false
		}
		return true
	}
}

func Auth(s *conf.Auth, repo admin.CasbinRuleRepo, userRepo admin.SysUserRepo) middleware.Middleware {
	return selector.Server(
		jwt.Server(
			func(token *jwtV5.Token) (interface{}, error) { return []byte(s.JwtKey), nil },
			jwt.WithSigningMethod(jwtV5.SigningMethodHS256),
			jwt.WithClaims(func() jwtV5.Claims { return &authz.TokenClaims{} }),
		),
		// JWT 黑名单和 IP 黑名单检查中间件
		func(handler middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req interface{}) (interface{}, error) {
				// 获取客户端 IP
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
				
				// 检查 JWT 是否在黑名单中
				rawToken := ""
				if tr, ok := kratoshttp.RequestFromServerContext(ctx); ok {
					authHeader := tr.Header.Get("Authorization")
					if strings.HasPrefix(authHeader, "Bearer ") {
						rawToken = strings.TrimPrefix(authHeader, "Bearer ")
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
		casbin.Server(
			casbin.WithCasbinModel(repo.GetModel()),
			casbin.WithCasbinPolicy(repo.GetAdapter()),
			casbin.WithSecurityUserCreator(authz.NewSecurityUser),
			casbin.WithAutoLoadPolicy(true, 30*time.Second),
		),
	).Match(AuthWhiteListMatcher()).Build()
}
