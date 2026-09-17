package admin

// ProvideMFAService 从 conf.Auth.Mfa 构造 mfa.Service（wire provider）。
//
//   - 默认不启用（mfa.enabled=false）：Config.Validate 直接通过，Service 内部
//     按配置短路（Status 返回 Available=false、各方法返回 ErrDisabled）——零行为影响。
//   - enabled=true 时：必须配置 32 字节 encryptionKey（Validate fail-closed
//     拒绝占位密钥/短密钥，启动失败倒逼正确配置）；且须先行执行 DDL。

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/mfastore"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
)

// ProvideMFAService 构造 MFA 服务
func ProvideMFAService(authConf *conf.Auth, db *gorm.DB, rdb redis.UniversalClient, logger *logx.Logger) (*mfa.Service, error) {
	cfg := mfa.Config{
		Enabled:       authConf.GetMfa().GetEnabled(),
		Required:      authConf.GetMfa().GetRequired(),
		EncryptionKey: authConf.GetMfa().GetEncryptionKey(),
		Issuer:        authConf.GetMfa().GetIssuer(),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	// sys_users MFA 三列读写经 gen 字段对象 store（data/mfastore 实现 pkg/mfa 端口）；
	// db 仍传入仅供恢复码表 model 式读写。
	return mfa.NewService(mfastore.New(db), db, rdb, cfg, logger), nil
}
