-- ===========================================================================================
-- 20261105 用户登录方式（sys_users.login_method）：TOTP / 手机验证码短信 二选一
--
-- 语义：登录方式按用户强制，与全局 auth.mfa.required 解耦
--   login_method = 1 → TOTP 动态验证码（默认，存量用户全部按此）：
--       已绑定走 /mfa/verify 两段式；未绑定无条件引导绑定（MustEnroll）；
--       auth.mfa.enabled=false（TOTP 能力总闸关）时降级为单因素登录并打 Error 日志。
--   login_method = 2 → 手机验证码短信：
--       密码通过后向该用户手机号发码，签发短信待验证 token（needSms=true），
--       第二步 POST /system/user/phone/verify 校验短信码换正式 token；
--       无手机号 / 发送失败 / 短信通道未启用一律拒绝（fail-closed）。
--   login_method 取其他值（含 0 脏值）按 TOTP 处理（fail-closed 落到更严的一侧）。
--
-- 幂等口径：MySQL 8.4 的 ALTER TABLE ... ADD COLUMN **不支持** IF NOT EXISTS（MariaDB 才支持），
-- MySQL 直接写会 1064 语法错。故沿用 information_schema 判定 + PREPARE/EXECUTE 动态执行。
-- 列放在 mfa_bound_at 之后，与 gen 模型字段顺序一致。
--
-- 执行后（模板仓：dev 库）请跑 `make gen` 或对应生成器刷新 gen 模型/DAO；
-- 本仓 gen 产物已手工对齐（见 app/admin/internal/data/gen/）。
-- ===========================================================================================
SET @sql := IF(
  (SELECT COUNT(*) FROM information_schema.columns
     WHERE table_schema = DATABASE() AND table_name = 'sys_users'
       AND column_name = 'login_method') = 0,
  'ALTER TABLE sys_users ADD COLUMN login_method tinyint NOT NULL DEFAULT 1 COMMENT ''登录方式 1=TOTP动态验证码(默认) 2=手机验证码短信'' AFTER mfa_bound_at',
  'SELECT ''column already exists'''
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 校验：1 = 列已存在（0 = 失败）；2 = 非 TOTP 用户数（存量应为 0）
SELECT COUNT(*) AS column_present FROM information_schema.columns
 WHERE table_schema = DATABASE() AND table_name = 'sys_users' AND column_name = 'login_method';
SELECT COUNT(*) AS non_totp_users FROM sys_users WHERE login_method <> 1;
