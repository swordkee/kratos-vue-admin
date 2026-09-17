package mfa

// AES-256-GCM 密钥加解密测试（对齐 api-admin mfa_crypto_test.go）。
// 反证：篡改密文/错误密钥必须解密失败；密钥非法必须显式报错。

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testKey      = []byte("0123456789abcdef0123456789abcdef") // 32 字节
	testTooShort = []byte("short-key")
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	secrets := []string{
		"JBSWY3DPEHPK3PXP",
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ234567",
		"短密钥也可加密测试中文",
		"",
	}
	for _, s := range secrets {
		enc, err := EncryptSecret(s, testKey)
		require.NoError(t, err)
		require.NotEmpty(t, enc)
		dec, err := DecryptSecret(enc, testKey)
		require.NoError(t, err)
		assert.Equal(t, s, dec, "加密解密必须往返一致")
	}
}

func TestEncryptDecrypt_TamperedFails(t *testing.T) {
	enc, err := EncryptSecret("JBSWY3DPEHPK3PXP", testKey)
	require.NoError(t, err)
	// 篡改密文最后一个字符（不同 nonce/ciphertext）
	tampered := enc[:len(enc)-1] + strings.ReplaceAll(string(enc[len(enc)-1]), enc[len(enc)-1:], "A")
	if tampered == enc {
		tampered = enc[:len(enc)-2] + "B"
	}
	_, err = DecryptSecret(tampered, testKey)
	require.Error(t, err, "篡改密文必须解密失败（GCM 认证）")
}

func TestEncryptDecrypt_WrongKeyFails(t *testing.T) {
	enc, err := EncryptSecret("JBSWY3DPEHPK3PXP", testKey)
	require.NoError(t, err)
	wrongKey := []byte("ffffffffffffffffffffffffffffffff")
	_, err = DecryptSecret(enc, wrongKey)
	require.Error(t, err, "错误密钥必须解密失败")
}

func TestEncryptSecret_BadKeyLen(t *testing.T) {
	_, err := EncryptSecret("secret", testTooShort)
	require.ErrorIs(t, err, ErrMFAKeyNotConfigured, "密钥长度非 32 字节必须显式报错")
	_, err = EncryptSecret("secret", nil)
	require.ErrorIs(t, err, ErrMFAKeyNotConfigured)
}

func TestDecryptSecret_BadInput(t *testing.T) {
	// 非 base64
	_, err := DecryptSecret("!!!not-base64!!!", testKey)
	require.Error(t, err)
	// 密文太短（不含 nonce）
	_, err = DecryptSecret("aGk=", testKey)
	require.ErrorIs(t, err, ErrMFACiphertextShort, "短密文必须报 ErrMFACiphertextShort")
	// 空密文
	_, err = DecryptSecret("", testKey)
	require.Error(t, err)
}

func TestConfig_Validate(t *testing.T) {
	// 默认不启用：不校验，直接通过（现网零影响）
	require.NoError(t, Config{Enabled: false}.Validate())

	// 启用但空密钥 → 拒绝
	require.Error(t, Config{Enabled: true, EncryptionKey: ""}.Validate())
	// 启用但占位密钥 → 拒绝
	require.Error(t, Config{Enabled: true, EncryptionKey: "change-me-to-32-bytes-secret-key"}.Validate())
	// 启用但长度不足 → 拒绝
	require.Error(t, Config{Enabled: true, EncryptionKey: "short"}.Validate())
	// 启用 + 32 字节随机密钥 → 通过
	require.NoError(t, Config{Enabled: true, EncryptionKey: string(testKey)}.Validate())
}

func TestConfig_FeatureFlags(t *testing.T) {
	// 总开关默认关
	require.False(t, Config{}.FeatureEnabled())
	require.False(t, Config{}.ForceRequired())
	// enabled=true 但 required=false → 不强制
	c := Config{Enabled: true, Required: false, EncryptionKey: string(testKey)}
	require.True(t, c.FeatureEnabled())
	require.False(t, c.ForceRequired())
	// 两者都开 → 强制
	c2 := Config{Enabled: true, Required: true, EncryptionKey: string(testKey)}
	require.True(t, c2.FeatureEnabled())
	require.True(t, c2.ForceRequired())
	// required 但 enabled=false → 不强制（总开关前提）
	c3 := Config{Enabled: false, Required: true}
	require.False(t, c3.ForceRequired())
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(10)
	require.NoError(t, err)
	require.Len(t, codes, 10)
	seen := map[string]bool{}
	for _, c := range codes {
		// 格式 XXXX-XXXX（8 大写字母数字）
		require.Len(t, c, 9, "恢复码格式 XXXX-XXXX")
		require.Equal(t, "-", c[4:5])
		require.False(t, seen[c], "恢复码必须唯一")
		seen[c] = true
	}
}