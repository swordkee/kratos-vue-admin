package authz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-kratos/kratos/v3/transport/http"
	jwtV5 "github.com/golang-jwt/jwt/v5"
	"github.com/swordkee/kratos-casbin/authz"
)

type contextKey string

const claimsKey contextKey = "jwt_claims"

var (
	ErrTokenMiss  = errors.New(500, "token miss", "token miss")
	ErrClaimsMiss = errors.New(500, "claims miss", "claims miss")
)

// NewContext 将解析后的 claims 注入 ctx（自定义 JWT 中间件使用，
// kratos v3 已移除 middleware/auth/jwt，不再依赖其 context key）。
func NewContext(ctx context.Context, claims *TokenClaims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// SuperAdminRoleKey 超级管理员角色 key：casbin 策略缺失时旁路放行，
// 避免 casbin_rule 表为空/策略缺失锁死系统（对齐下游 E-P0-01）。
const SuperAdminRoleKey = "admin"

// IsSuperAdmin 判断角色 key 是否超级管理员。
func IsSuperAdmin(roleKey string) bool {
	return roleKey == SuperAdminRoleKey
}

type TokenClaims struct {
	UserID   int64  `json:"user_id"`
	RoleID   int64  `json:"role_id"`
	RoleKey  string `json:"role_key"`
	Nickname string `json:"nickname"`
	jwtV5.RegisteredClaims
}

type securityUser struct {
	Path        string
	Method      string
	AuthorityId string
	Domain      string
}

func NewSecurityUser() authz.SecurityUser {
	return &securityUser{}
}

func (su *securityUser) ParseFromContext(ctx context.Context) error {
	claims, err := FromContext(ctx)
	if err != nil {
		return err
	}
	su.AuthorityId = claims.RoleKey
	ts, ok := transport.FromServerContext(ctx)
	if !ok {
		return ErrClaimsMiss
	}
	su.Path = ts.Operation()
	ht, ok := ts.(http.Transporter)
	if !ok {
		return ErrClaimsMiss
	}
	su.Method = ht.Request().Method
	return nil
}

func (su *securityUser) GetSubject() string {
	return su.AuthorityId
}

func (su *securityUser) GetObject() string {
	return su.Path
}

func (su *securityUser) GetAction() string {
	return su.Method
}

func (su *securityUser) GetDomain() string {
	return su.Domain
}

func FromContext(ctx context.Context) (*TokenClaims, error) {
	claims, ok := ctx.Value(claimsKey).(*TokenClaims)
	if !ok || claims == nil {
		return nil, ErrTokenMiss
	}
	return claims, nil
}

func MustFromContext(ctx context.Context) *TokenClaims {
	claims, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return claims
}

func NewToken(key string, expireAt time.Time, userID, roleID int64, roleKey, nickname string) (string, error) {
	claims := jwtV5.NewWithClaims(jwtV5.SigningMethodHS256, &TokenClaims{
		UserID:   userID,
		RoleID:   roleID,
		Nickname: nickname,
		RoleKey:  roleKey,
		RegisteredClaims: jwtV5.RegisteredClaims{
			Issuer:    "admin",
			ExpiresAt: jwtV5.NewNumericDate(expireAt),
		},
	})
	return claims.SignedString([]byte(key))
}
