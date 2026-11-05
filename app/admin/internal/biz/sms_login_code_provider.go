package biz

// 登录二因素短信码端口的装配（sys_users.login_method=2）。
//
// admin.SmsLoginCodeRepo 是 biz/admin 定义的窄接口（SendLoginCode/VerifyLoginCode），
// biz/sms.UseCase 已实现同名同签名方法 ⇒ 这里只做接口绑定，便于 AuthUseCase 单测注入假实现。
//
// 模板仓口径：短信网关设施（sys_sms_gateway / config message.providers）默认关闭，
// 此时 SendLoginCode 会因「未找到短信提供商配置」失败 ⇒ login_method=2 的用户登录被
// fail-closed 拒绝。接入真实短信网关后（config message.enabled=true + 网关配置）即自动可用，
// 无需改动本文件或 biz 层。

import (
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/sms"
)

// SmsLoginCodeRepoProvider 把短信用例绑定为登录短信码端口
func SmsLoginCodeRepoProvider(uc *sms.UseCase) admin.SmsLoginCodeRepo { return uc }
