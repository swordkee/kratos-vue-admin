-- ============================================================
-- 敏感信息按权限可见（2026-11-04，对齐 bmall/go-exAdmin 同名功能）
--   /sensitive/view (GET)：虚拟权限点 = 明文查看身份证/手机号/银行卡号
--   （无真实路由；casbin obj 直传，角色管理 → API 分配 勾选后写 casbin_rule）
--
-- 目标表：sys_apis（本仓系统库；幂等：存在即跳过）
-- 分配方式：管理后台「API 管理」可见该条 → 「角色管理」按需勾选；
--          超管角色（role_key=admin）无需分配（中间件旁路）。
-- 回滚：DELETE FROM sys_apis WHERE path = '/sensitive/view' AND method = 'GET';
--
-- 生效链路：SensitiveInfoGuard 中间件（Auth 之后）→ casbin Enforce(roleKey,
-- '/sensitive/view', 'GET') → ctx 注入 reveal → service 层决定明文/掩码。
--
-- 本仓（KVA 模板仓）示范挂载：sys_users.phone（用户管理 FindSysUser/ListSysUser），
-- 写入口 UpdateSysUser 做掩码回环保护；业务仓接入法见 service/admin/sensitive_mask.go 头注。
-- ============================================================

INSERT INTO sys_apis (path, description, api_group, method, created_at, updated_at)
SELECT '/sensitive/view', '敏感信息明文查看（按角色授权；控制身份证/手机号/银行卡号是否掩码）', '敏感数据', 'GET', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (
  SELECT 1 FROM sys_apis
  WHERE path = '/sensitive/view' AND method = 'GET' AND deleted_at IS NULL
);

-- 验证（应返回 1 行）
-- SELECT path, method, api_group FROM sys_apis WHERE api_group = '敏感数据';
