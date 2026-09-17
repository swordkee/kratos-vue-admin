-- ######################## KVA 模板 R26 TOTP 双因素认证 DDL（连接 KVA 库执行） ########################
-- 设计：docs/design/2026-09-12-mfa-totp-integration.md §4.6；代码：pkg/mfa + app/admin SysMfa
-- 幂等：information_schema 列存在守卫 + CREATE TABLE IF NOT EXISTS；可重复执行。
-- 前置：config.yaml auth.mfa.enabled=true 前必须先执行本脚本（否则登录读取 mfa_* 列报 1054）。
-- 回滚：ALTER TABLE sys_users DROP COLUMN mfa_bound_at, DROP COLUMN mfa_secret, DROP COLUMN mfa_enabled;
--       DROP TABLE sys_mfa_recovery_code;

-- 1.  sys_users MFA 三列（TOTP 双因素）
SET @c := (SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='sys_users' AND column_name='mfa_enabled');
SET @s := IF(@c=0,
  'ALTER TABLE sys_users
     ADD COLUMN mfa_enabled TINYINT NOT NULL DEFAULT 0 COMMENT ''TOTP 双因素是否开启(AES-256-GCM 加密密钥存 mfa_secret)'' AFTER secret,
     ADD COLUMN mfa_secret TEXT NULL COMMENT ''AES-256-GCM 加密的 TOTP secret(32 字节主密钥，auth.mfa.encryptionKey)'' AFTER mfa_enabled,
     ADD COLUMN mfa_bound_at DATETIME NULL DEFAULT NULL COMMENT ''TOTP 绑定时间'' AFTER mfa_secret',
  'SELECT ''sys_users.mfa_* exists, skip'' AS msg');
PREPARE st FROM @s; EXECUTE st; DEALLOCATE PREPARE st;

-- 2.  sys_mfa_recovery_code 恢复码表（一次性，bcrypt 哈希；绑定成功仅返回一次明文）
CREATE TABLE IF NOT EXISTS `sys_mfa_recovery_code` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint NOT NULL COMMENT 'sys_users.id',
  `code_hash` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '恢复码 bcrypt 哈希',
  `used_at` datetime NULL DEFAULT NULL COMMENT '使用时间（一次性标记）',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_user_id`(`user_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = DYNAMIC COMMENT = 'TOTP 恢复码（一次性）';
-- ######################## 结束 ########################
