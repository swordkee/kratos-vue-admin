package biz

import (
	"github.com/redis/go-redis/v9"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/sms"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// NewSmsUseCase 装配消息/短信验证码用例（nil 配置时也安全：功能默认关闭）
func NewSmsUseCase(messageConf *conf.Message, rdb redis.UniversalClient, gatewayRepo sms.GatewayRepo, logger *logx.Logger) *sms.UseCase {
	return sms.NewUseCase(messageConf, rdb, gatewayRepo, logger)
}
