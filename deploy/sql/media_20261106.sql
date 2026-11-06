-- ============================================================================
-- 媒体库（附件分类 / 文件上传下载 / 断点续传）建表 + 默认分类
-- 模板版：2026-11-06 从 wallpaper/go-kva-admin（feat/wp-media）回灌
--
-- 结构来源：api-admin（gin-vue-admin）server/model/example/*
--   exa_attachment_category        exa_attachment_category.go  （分类树，pid 自关联，无唯一索引）
--   exa_file_upload_and_downloads  exa_file_upload_download.go （文件记录；key 为对象键）
--   exa_files                      exa_breakpoint_continue.go  （断点续传主表，按 file_md5 去重）
--   exa_file_chunks                exa_breakpoint_continue.go  （断点续传分片表，exa_file_id 关联）
--
-- 与源 gorm 逐列对齐（global.GVA_MODEL = id/created_at/updated_at/deleted_at）：
--   * 源模型所有字符串列可空（Go 侧非指针 ⇒ NULL 读成 ""），此处保持 NULL 默认；
--   * `key` 为 MySQL 保留字，DDL 与查询一律反引号；
--   * 源模型 OSSUrl/Summary/Category/CategoryId/Tags/TagId/WebTitle/Short/Long/NameZh/
--     SummaryZh 均为 gorm:"-"（非落库列），本 DDL 不建；
--   * 源端删除走 Unscoped（物理删），deleted_at 仅保留以兼容 gorm 软删默认条件。
--
-- ⚠️ 幂等口径：本文件**不 DROP 表、不清数据**（媒体库是承载业务数据的表，
--   重复执行只补建缺表、补默认行）。需清库重建请手工 DROP 后重跑。
-- 字符集 utf8mb4。
-- ============================================================================
SET NAMES utf8mb4;

-- ----------------------------
-- 媒体库分类（/attachmentCategory/*）
-- ----------------------------
CREATE TABLE IF NOT EXISTS `exa_attachment_category` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `created_at` datetime(3) NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime(3) NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3) NULL DEFAULT NULL COMMENT '删除时间',
  `name` varchar(255) NULL DEFAULT NULL COMMENT '分类名称',
  `pid` int(11) NULL DEFAULT 0 COMMENT '父节点ID（0=根）',
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_pid` (`pid`),
  KEY `idx_name_pid` (`name`, `pid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='媒体库分类';

-- ----------------------------
-- 文件记录（/fileUploadAndDownload/*）
-- ----------------------------
CREATE TABLE IF NOT EXISTS `exa_file_upload_and_downloads` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `created_at` datetime(3) NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime(3) NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3) NULL DEFAULT NULL COMMENT '删除时间',
  `name` varchar(255) NULL DEFAULT NULL COMMENT '文件名/备注',
  `url` varchar(255) NULL DEFAULT NULL COMMENT '访问地址',
  `tag` varchar(255) NULL DEFAULT NULL COMMENT '文件标签（扩展名）',
  `key` varchar(255) NULL DEFAULT NULL COMMENT '对象键',
  `resolution` varchar(255) NULL DEFAULT NULL COMMENT '分辨率',
  `size` bigint(20) NULL DEFAULT NULL COMMENT '大小（字节）',
  `oss_type` varchar(255) NULL DEFAULT NULL COMMENT '存储类型 local/aliyun',
  `created_by` bigint(20) unsigned NULL DEFAULT NULL COMMENT '上传用户 UID',
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_key` (`key`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='文件上传下载记录';

-- ----------------------------
-- 断点续传主表（/fileUploadAndDownload/{findFile,breakpointContinue*}）
-- ----------------------------
CREATE TABLE IF NOT EXISTS `exa_files` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `created_at` datetime(3) NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime(3) NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3) NULL DEFAULT NULL COMMENT '删除时间',
  `file_name` varchar(255) NULL DEFAULT NULL COMMENT '文件名',
  `file_md5` varchar(255) NULL DEFAULT NULL COMMENT '文件指纹（源端为整体 MD5）',
  `file_path` varchar(255) NULL DEFAULT NULL COMMENT '合并后对象键',
  `chunk_total` int(11) NULL DEFAULT NULL COMMENT '分片总数',
  `is_finish` tinyint(1) NULL DEFAULT 0 COMMENT '是否已完成（1=秒传/已合并）',
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_file_md5` (`file_md5`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='断点续传主表';

-- ----------------------------
-- 断点续传分片表
-- ----------------------------
CREATE TABLE IF NOT EXISTS `exa_file_chunks` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `created_at` datetime(3) NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` datetime(3) NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3) NULL DEFAULT NULL COMMENT '删除时间',
  `exa_file_id` bigint(20) unsigned NULL DEFAULT NULL COMMENT 'exa_files.id',
  `file_chunk_number` int(11) NULL DEFAULT NULL COMMENT '分片序号（0 起）',
  `file_chunk_path` varchar(255) NULL DEFAULT NULL COMMENT '分片落盘路径',
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_exa_file_id` (`exa_file_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='断点续传分片';

-- ----------------------------
-- 默认分类（源端 initdb 的「默认分类」；按 name+pid 幂等，可重复执行）
-- ----------------------------
INSERT INTO `exa_attachment_category` (`created_at`, `updated_at`, `name`, `pid`)
SELECT NOW(3), NOW(3), '默认分类', 0
FROM DUAL
WHERE NOT EXISTS (
  SELECT 1 FROM `exa_attachment_category`
  WHERE `name` = '默认分类' AND `pid` = 0 AND `deleted_at` IS NULL
);

-- ----------------------------
-- 自检（执行后人工核对，期望 4 张表 / 1 行默认分类）
-- ----------------------------
-- SHOW TABLES LIKE 'exa_%';
-- SELECT id, name, pid FROM exa_attachment_category WHERE deleted_at IS NULL;
