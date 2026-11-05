package admin

// 敏感信息响应脱敏辅助（2026-11-04，对齐 go-exAdmin「API 层逐字段显式掩码」层 B）。
//
// 判定来源：SensitiveInfoGuard 中间件经 sensitive.CanReveal(ctx) 注入；
// reveal=true（超管/获授虚拟权限点 /sensitive/view）原样返回，false 就地掩码。
// 配套约束：更新入口须做掩码回环保护（值含 '*' 视为未修改，保持库中原值）。
//
// 范围（本仓 = KVA 模板仓，无 example/exa_customer 业务）：示范挂载在本仓自带的
// 系统用户模型上 —— sys_users.phone（用户管理读面 FindSysUser/ListSysUser），
// 更新入口 UpdateSysUser 做掩码回环保护。业务仓接入方式：
//   1. 响应构建点对 PII 字段调用 maskXxx(sensitive.CanReveal(ctx), v)
//      （或直接用 pkg/sensitive 的 MaskIDCard / MaskBankCard / MaskPhoneLike）；
//   2. 写入口对「响应面被掩码过的字段」用 sensitive.HasMask + sensitive.ResolveMasked
//      做回环保护，避免掩码串回写覆盖明文；
//   3. 中间件与 casbin 虚拟权限点无需改，只需在「API 管理 → 角色管理」勾选
//      /sensitive/view (GET) 授权（超管旁路）。
// 不掩：AuthReply.User.Phone —— 那是「当前登录操作员自己的资料」，本人可见。

import "github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/sensitive"

// maskPhone 手机号按权限掩码。
func maskPhone(reveal bool, v string) string {
	if reveal {
		return v
	}
	return sensitive.MaskMobile(v)
}
