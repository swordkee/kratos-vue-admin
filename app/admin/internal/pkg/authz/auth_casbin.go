package authz

import (
	"context"
	"crypto/ecdsa"
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
	// MfaPending 两段式登录中间态（TOTP）：密码已过但待二因素验证。
	// 携带此 flag 的 token 仅短有效期，中间件拒绝其访问业务接口（仅 /mfa/verify 可用）。
	// 旧 token 无此字段解析为 false，向后兼容。
	MfaPending bool `json:"mfa_pending"`
	// MustEnrollMfa 强制绑定 TOTP 但用户尚未绑定（mfa.required=true 时）。
	// 携带此 flag 的 token 仅放行绑定相关接口，其余 409 引导。
	MustEnrollMfa bool `json:"must_enroll_mfa"`
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

// NewToken 用 ECDSA P-384 私钥签发 ES384 JWT（普通登录 token）。
func NewToken(priv *ecdsa.PrivateKey, expireAt time.Time, userID, roleID int64, roleKey, nickname string) (string, error) {
	return newToken(priv, expireAt, userID, roleID, roleKey, nickname, false, false)
}

// NewMFAPendingToken 签发两段式登录的待二因素验证短期 token（MfaPending=true，TTL 通常 5 分钟）。
// 密码校验通过但用户已绑定 TOTP 时使用：前端 /mfa/verify 验证通过后才签发正式 token。
// 中间件对 MfaPending token 拒绝访问业务接口（仅 /mfa/verify 白名单）。
func NewMFAPendingToken(priv *ecdsa.PrivateKey, ttl time.Duration, userID, roleID int64, roleKey, nickname string) (string, error) {
	return newToken(priv, time.Now().Add(ttl), userID, roleID, roleKey, nickname, true, false)
}

// NewMustEnrollToken 签发强制绑定 token（MustEnrollMfa=true，mfa.required 且用户未绑定时使用）。
// MfaRequiredGuard 仅放行绑定相关接口，其余 409 引导前端跳个人中心绑定。
func NewMustEnrollToken(priv *ecdsa.PrivateKey, expireAt time.Time, userID, roleID int64, roleKey, nickname string) (string, error) {
	return newToken(priv, expireAt, userID, roleID, roleKey, nickname, false, true)
}

// newToken 统一签发（mfaPending/mustEnrollMfa 由调用方按场景指定）。
func newToken(priv *ecdsa.PrivateKey, expireAt time.Time, userID, roleID int64, roleKey, nickname string, mfaPending, mustEnrollMfa bool) (string, error) {
	claims := jwtV5.NewWithClaims(jwtV5.SigningMethodES384, &TokenClaims{
		UserID:        userID,
		RoleID:        roleID,
		Nickname:      nickname,
		RoleKey:       roleKey,
		MfaPending:    mfaPending,
		MustEnrollMfa: mustEnrollMfa,
		RegisteredClaims: jwtV5.RegisteredClaims{
			Issuer:    "admin",
			ExpiresAt: jwtV5.NewNumericDate(expireAt),
		},
	})
	return claims.SignedString(priv)
}
