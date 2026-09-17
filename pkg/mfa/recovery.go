package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
	"time"
)

// RecoveryCodeCount 绑定成功时生成的恢复码数量
const RecoveryCodeCount = 10

// SysMfaRecoveryCode 恢复码表模型（新表 sys_mfa_recovery_code，DDL 见 consolidated）
type SysMfaRecoveryCode struct {
	ID       uint       `gorm:"column:id;primaryKey;autoIncrement"`
	UserID   int64      `gorm:"column:user_id;index"`
	CodeHash string     `gorm:"column:code_hash"`
	UsedAt   *time.Time `gorm:"column:used_at"`
}

// TableName 表名
func (*SysMfaRecoveryCode) TableName() string { return "sys_mfa_recovery_code" }

// GenerateRecoveryCodes 生成 n 个形如 XXXX-XXXX 的恢复码，使用 crypto/rand
func GenerateRecoveryCodes(n int) ([]string, error) {
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		s := strings.ToUpper(enc.EncodeToString(b))
		if len(s) < 8 {
			s = strings.Repeat("A", 8-len(s)) + s
		}
		out = append(out, s[:4]+"-"+s[4:8])
	}
	return out, nil
}