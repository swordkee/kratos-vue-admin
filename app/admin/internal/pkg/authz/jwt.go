package authz

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	jwtV5 "github.com/golang-jwt/jwt/v5"
)

// ECDSACurve 是 JWT ES384 使用的椭圆曲线：P-384。
// 跨服务统一使用同一把 ECDSA P-384 密钥对，曲线必须严格收敛，防密钥误配。
var ECDSACurve = elliptic.P384()

// ParseECDSAPrivateKey 解析 ECDSA P-384 私钥 PEM。
// 兼容 SEC1（BEGIN EC PRIVATE KEY）与 PKCS8（BEGIN PRIVATE KEY）两种封装，
// 并校验曲线必须是 P-384（ES384）。
func ParseECDSAPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	if pemStr == "" {
		return nil, errors.New("ecdsa private key is empty")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode ecdsa private key PEM")
	}
	var (
		key any
		err error
	)
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unexpected private key PEM type: %s", block.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("parse ecdsa private key: %w", err)
	}
	priv, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an ECDSA private key")
	}
	if priv.Curve != ECDSACurve {
		return nil, errors.New("ecdsa private key curve is not P-384")
	}
	return priv, nil
}

// ParseECDSAPublicKey 解析 ECDSA P-384 公钥 PEM（PKIX，BEGIN PUBLIC KEY），
// 并校验曲线必须是 P-384（ES384）。
func ParseECDSAPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	if pemStr == "" {
		return nil, errors.New("ecdsa public key is empty")
	}
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode ecdsa public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse ecdsa public key: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}
	if ecPub.Curve != ECDSACurve {
		return nil, errors.New("ecdsa public key curve is not P-384")
	}
	return ecPub, nil
}

// ES384Keyfunc 构造验证侧的 keyfunc：拒绝非 ES384 算法（防 alg 混淆），
// 并返回 ECDSA P-384 公钥供 jwt/v5 验签。
// pub 为 nil（配置缺失/解析失败/调用方误传）时返回错误（fail-closed）：
// 严禁把 typed-nil 公钥交给 jwt/v5，否则 ecdsa.Verify 访问 pub.Curve 会空指针 panic。
func ES384Keyfunc(pub *ecdsa.PublicKey) jwtV5.Keyfunc {
	return func(token *jwtV5.Token) (any, error) {
		// 算法收敛：仅接受 ES384，拒绝 RS256/HS256 等任意其他算法。
		if token.Method.Alg() != jwtV5.SigningMethodES384.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// 公钥 nil 防护：fail-closed 返回错误，而非 nil/typed-nil 公钥。
		if pub == nil {
			return nil, errors.New("ecdsa public key is nil")
		}
		return pub, nil
	}
}

// ParseToken 解析并验证 JWT（ES384），返回自定义 claims。
//
// 供独立验签场景使用（如 R26 TOTP /mfa/verify 解析 MfaPending token）：
// 与中间件 jwtServer 同口径（ParseWithClaims + ES384Keyfunc + 算法收敛）。
// 校验通过返回 claims；失败返回具体错误（过期/签名/算法/格式）。
func ParseToken(tokenStr string, pub *ecdsa.PublicKey) (*TokenClaims, error) {
	token, err := jwtV5.ParseWithClaims(tokenStr, &TokenClaims{}, ES384Keyfunc(pub))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
