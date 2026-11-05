-- ===========================================================================================
-- 20261105 登录安全策略表 sys_security_config（gva loginlock / SecurityLock 体系回灌）
--
-- 语义：单行表（固定 id=1），进程内原子缓存热读，保存即热更新。
--   已接入行为：失败锁定段（lock_enable / lock_threshold / lock_duration）
--     → POST /system/user/login：锁定期间直接拒绝；用户名不存在与密码错误同样计数；
--       停用账号不计数；TOTP / 短信待验证阶段不清计数，完整登录成功才清。
--     计数与锁态存 Redis（login_fail:<username> / login_lock:<username>）；
--     未配置 Redis 时退化为进程内存（单实例）；Redis 故障按源语义 fail-open
--     （读不到锁 = 未锁定、计数失败只记日志），不因缓存故障打死全站登录。
--   仅提供读写面、行为尚未接入：验证码段 / 密码复杂度段 / 限流段 / 密码过期段 /
--     mfa_required（KVA 的 sys_users 无 password_updated_at 列，密码过期不生效）。
--
-- 幂等口径：本文件只做「缺表补建 + 缺行补插 + 缺 API 记录补插」，重复执行不删数据。
--   管理面接口（4 个，超管 role_key=admin 由中间件旁路；其余角色需在「API 管理」→
--   「角色管理」勾选后写 casbin_rule）：
--     GET  /system/security-config         查询配置
--     PUT  /system/security-config         保存配置
--     GET  /system/security-config/lock    查询账号锁状态
--     POST /system/security-config/unlock  人工解锁
--
-- 执行后请跑 `make gen`（或 app/admin/cmd/tools/generator.go）刷新 gen 模型/DAO；
-- 模板仓 gen 产物已同步提交（app/admin/internal/data/gen/）。
-- 回滚：DROP TABLE sys_security_config; DELETE FROM sys_apis WHERE api_group = '登录安全';
-- ===========================================================================================

CREATE TABLE IF NOT EXISTS `sys_security_config` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `captcha_open` int NOT NULL DEFAULT 0 COMMENT '错误N次后出验证码 0=每次都要',
  `captcha_timeout` int NOT NULL DEFAULT 3600 COMMENT '防爆破计数缓存超时(秒)',
  `key_long` int NOT NULL DEFAULT 6 COMMENT '验证码长度',
  `img_width` int NOT NULL DEFAULT 240 COMMENT '验证码宽度',
  `img_height` int NOT NULL DEFAULT 80 COMMENT '验证码高度',
  `pwd_min_length` int NOT NULL DEFAULT 8 COMMENT '密码最小长度',
  `pwd_require_upper` tinyint(1) NOT NULL DEFAULT 0 COMMENT '需大写字母',
  `pwd_require_lower` tinyint(1) NOT NULL DEFAULT 0 COMMENT '需小写字母',
  `pwd_require_digit` tinyint(1) NOT NULL DEFAULT 0 COMMENT '需数字',
  `pwd_require_special` tinyint(1) NOT NULL DEFAULT 0 COMMENT '需特殊字符',
  `limit_enable` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否开启限流',
  `limit_window` int NOT NULL DEFAULT 60 COMMENT '限流窗口(秒)',
  `limit_count` int NOT NULL DEFAULT 30 COMMENT '窗口内最大次数',
  `lock_enable` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否开启失败锁定',
  `lock_threshold` int NOT NULL DEFAULT 5 COMMENT '失败次数阈值',
  `lock_duration` int NOT NULL DEFAULT 30 COMMENT '锁定时长(分钟)',
  `pwd_expire_enable` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否开启密码过期',
  `pwd_expire_days` int NOT NULL DEFAULT 90 COMMENT '密码有效天数',
  `force_new_user_change_password` tinyint(1) NOT NULL DEFAULT 0 COMMENT '新用户首次登录是否强制改密',
  `mfa_required` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否强制全员开启TOTP',
  `created_at` datetime NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = DYNAMIC COMMENT = '登录安全策略（单行表）';

-- 默认行（id=1）；服务端缺行时也会按代码默认值惰性补建，这里预置便于直接改配置。
INSERT INTO `sys_security_config`
  (`id`, `captcha_open`, `captcha_timeout`, `key_long`, `img_width`, `img_height`,
   `pwd_min_length`, `pwd_require_upper`, `pwd_require_lower`, `pwd_require_digit`, `pwd_require_special`,
   `limit_enable`, `limit_window`, `limit_count`,
   `lock_enable`, `lock_threshold`, `lock_duration`,
   `pwd_expire_enable`, `pwd_expire_days`, `force_new_user_change_password`, `mfa_required`,
   `created_at`, `updated_at`)
SELECT 1, 0, 3600, 6, 240, 80,
       8, 0, 0, 0, 0,
       0, 60, 30,
       0, 5, 30,
       0, 90, 0, 0,
       NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_security_config` WHERE `id` = 1);

-- API 记录（供「API 管理」展示与「角色管理」勾选；超管旁路无需分配）
INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`, `created_at`, `updated_at`)
SELECT '/system/security-config', '查询登录安全策略', '登录安全', 'GET', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_apis` WHERE `path` = '/system/security-config' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`, `created_at`, `updated_at`)
SELECT '/system/security-config', '保存登录安全策略', '登录安全', 'PUT', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_apis` WHERE `path` = '/system/security-config' AND `method` = 'PUT' AND `deleted_at` IS NULL);

INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`, `created_at`, `updated_at`)
SELECT '/system/security-config/lock', '查询账号登录锁状态', '登录安全', 'GET', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_apis` WHERE `path` = '/system/security-config/lock' AND `method` = 'GET' AND `deleted_at` IS NULL);

INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`, `created_at`, `updated_at`)
SELECT '/system/security-config/unlock', '解锁账号（清失败计数与锁态）', '登录安全', 'POST', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_apis` WHERE `path` = '/system/security-config/unlock' AND `method` = 'POST' AND `deleted_at` IS NULL);

-- 校验：1 = 表存在；1 = 默认行存在；4 = API 记录条数
SELECT COUNT(*) AS table_present FROM information_schema.tables
 WHERE table_schema = DATABASE() AND table_name = 'sys_security_config';
SELECT COUNT(*) AS default_row FROM `sys_security_config` WHERE `id` = 1;
SELECT COUNT(*) AS api_rows FROM `sys_apis` WHERE `api_group` = '登录安全' AND `deleted_at` IS NULL;