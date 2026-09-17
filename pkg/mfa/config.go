// Package mfa TOTP 双因素认证核心（移植自 api-admin feature/mfa-totp，c9558df10）。
//
// 设计见 docs/design/2026-09-12-mfa-totp-integration.md：
//   - 两段式登录：密码 → MfaPending 5min token → /mfa/verify → 正式 token
//   - TOTP secret AES-256-GCM 加密入库（32 字节主密钥，启动校验拒绝占位密钥）
//   - 10 个一次性恢复码（bcrypt 哈希入库，绑定成功仅返回一次）
//   - 绑定会话缓存 Redis Mfa:Enroll:<uid>（TTL 5min）
//   - 默认不启用（Config.Enabled=false），现网零影响
package mfa

import "errors"

// Config MFA 配置（由 conf.Auth.Mfa 注入）
type Config struct {
	// Enabled 总开关，默认 false。关闭后登录不要求二因素，绑定/解绑/重置接口不可用。
	Enabled bool
	// Required 强制全员绑定（在总开关开启前提下）。开启后未绑定用户登录后需先完成绑定才能使用其它接口。
	Required bool
	// EncryptionKey 用于 AES-256-GCM 加密存储用户 TOTP 密钥，必须为 32 字节。
	EncryptionKey string
	// Issuer TOTP 发行者名称，显示在验证器 App 中（缺省 "BMALL"）。
	Issuer string
}

// mfaPlaceholderKey 是默认配置中的示例占位密钥，禁止在 enabled=true 时使用。
const mfaPlaceholderKey = "change-me-to-32-bytes-secret-key"

// Validate 在 MFA 开启时校验加密密钥已被正确设置。
// 未开启时不校验，避免影响默认部署（默认不启用零影响）。
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if len(c.EncryptionKey) != MFAKeyLen {
		return errors.New("mfa.encryption-key 必须为 32 字节，当前长度不满足")
	}
	if c.EncryptionKey == mfaPlaceholderKey {
		return errors.New("mfa.enabled=true 时必须将 mfa.encryption-key 替换为随机 32 字节密钥，不能使用示例占位值")
	}
	return nil
}