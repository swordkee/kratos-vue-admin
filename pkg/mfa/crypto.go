package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// MFAKeyLen AES-256 密钥长度（32 字节）
const MFAKeyLen = 32

var (
	// ErrMFAKeyNotConfigured 加密主密钥未配置或长度非 32 字节
	ErrMFAKeyNotConfigured = errors.New("mfa.encryption-key 未配置或长度不是 32 字节")
	// ErrMFACiphertextShort 密文太短（不含 nonce）
	ErrMFACiphertextShort = errors.New("mfa 密文太短")
)

// EncryptSecret 使用 AES-256-GCM 加密 TOTP secret，返回 base64(nonce||ciphertext)
func EncryptSecret(plaintext string, key []byte) (string, error) {
	if len(key) != MFAKeyLen {
		return "", ErrMFAKeyNotConfigured
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret 解密 EncryptSecret 产生的密文
func DecryptSecret(encoded string, key []byte) (string, error) {
	if len(key) != MFAKeyLen {
		return "", ErrMFAKeyNotConfigured
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrMFACiphertextShort
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("mfa secret 解密失败: %w", err)
	}
	return string(plaintext), nil
}