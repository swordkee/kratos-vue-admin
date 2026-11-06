-- ============================================================================
-- 模板版媒体库菜单与操作日志 API（feat/media，2026-11-06 从 wallpaper/go-kva-admin 回灌）
--
-- 口径（与模板既有菜单 SQL 一致，见 deploy/sql/20261105_sensitive_view_api.sql）：
--   * 表结构 = deploy/sql/kva.sql 的 sys_menus / sys_role_menus / sys_apis；
--   * hidden 口径：后端 service/admin/menus.go 的 Build() 以 `hidden == 2` 判隐藏
--     （前端 meta.isHide）；0/1 均视为显示。本文件沿用既有菜单行的 1（显示）。
--   * menu_type: M 目录 / C 菜单 / F 按钮；
--   * component = web/src/views 下相对路径（router 的 import.meta.glob 前缀匹配，需唯一）。
--
-- 幂等：全部 INSERT ... SELECT ... WHERE NOT EXISTS（按 path+menu_type 判重），可重复执行。
-- 菜单 ID 交给 AUTO_INCREMENT；角色授权行按 path 反查 menu_id。
--
-- 说明：
--   1. 扫码上传页 views/media/scanUpload 是**静态路由** /scanUpload（手机扫码直达，
--      不进侧边菜单），故此处不为它建菜单行。
--   2. sys_apis 里的 14 条 = 11 条 proto 路由 + 3 条 multipart/流式裸路由的 casbin 对象名
--      （= proto operation）。角色管理里勾选后即可分配；超管（role_key=admin）由中间件旁路。
-- ============================================================================
SET NAMES utf8mb4;

-- ---------------------------------------------------------------------------
-- 一级目录：媒体库
-- ---------------------------------------------------------------------------
INSERT INTO `sys_menus`
(`menu_name`, `title`, `parent_id`, `sort`, `icon`, `path`, `component`, `is_iframe`, `link`,
 `menu_type`, `hidden`, `keep_alive`, `is_affix`, `permission`, `status`, `create_by`, `update_by`,
 `remark`, `created_at`, `updated_at`)
SELECT '媒体库', '', 0, 12, 'elementFiles', '/media', 'Layout', 2, '',
       'M', 1, 2, 2, '', 1, 'admin', '', '媒体库（feat/media：上传页 + 文件管理）', NOW(), NOW()
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `sys_menus` m WHERE m.`path` = '/media' AND m.`menu_type` = 'M');

SET @media_root := (SELECT `id` FROM `sys_menus` WHERE `path` = '/media' AND `menu_type` = 'M' AND `deleted_at` IS NULL ORDER BY `id` LIMIT 1);

-- 媒体库上传 → views/media/upload/index.vue
--   源 view/example/upload/upload.vue（分类树 + 多文件上传 + 分片上传 + 扫码上传 + 导入URL + 列表）
INSERT INTO `sys_menus`
(`menu_name`, `title`, `parent_id`, `sort`, `icon`, `path`, `component`, `is_iframe`, `link`,
 `menu_type`, `hidden`, `keep_alive`, `is_affix`, `permission`, `status`, `create_by`, `update_by`,
 `remark`, `created_at`, `updated_at`)
SELECT '媒体库上传', '', @media_root, 1, 'elementUpload', '/media/upload', 'media/upload/index', 2, '',
       'C', 1, 2, 2, 'media:upload', 1, 'admin', '',
       '/attachmentCategory/* + /fileUploadAndDownload/*（api/media/v1/media.proto）', NOW(), NOW()
FROM DUAL
WHERE @media_root IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `sys_menus` m WHERE m.`path` = '/media/upload' AND m.`menu_type` = 'C');

-- 文件管理 → views/media/file/index.vue
--   源 view/example/upload/upload.vue 的列表/改名/删除/导入URL（批量删为增补）
INSERT INTO `sys_menus`
(`menu_name`, `title`, `parent_id`, `sort`, `icon`, `path`, `component`, `is_iframe`, `link`,
 `menu_type`, `hidden`, `keep_alive`, `is_affix`, `permission`, `status`, `create_by`, `update_by`,
 `remark`, `created_at`, `updated_at`)
SELECT '文件管理', '', @media_root, 2, 'elementFiles', '/media/file', 'media/file/index', 2, '',
       'C', 1, 2, 2, 'media:file', 1, 'admin', '',
       '/fileUploadAndDownload/getFileList|editFileName|deleteFile|deleteFiles|importURL', NOW(), NOW()
FROM DUAL
WHERE @media_root IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM `sys_menus` m WHERE m.`path` = '/media/file' AND m.`menu_type` = 'C');

-- ---------------------------------------------------------------------------
-- 角色授权：超管理员(role_id=1) / 管理员(role_id=2)
-- ---------------------------------------------------------------------------
INSERT INTO `sys_role_menus` (`role_id`, `menu_id`, `role_name`)
SELECT r.id, m.id, r.role_name
FROM `sys_menus` m
JOIN `sys_roles` r ON r.id IN (1, 2)
WHERE m.`path` IN ('/media', '/media/upload', '/media/file')
  AND m.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM `sys_role_menus` rm WHERE rm.`role_id` = r.id AND rm.`menu_id` = m.id
  );

-- ---------------------------------------------------------------------------
-- 媒体库 API 权限点（casbin 对象名 = proto operation；供「角色管理」按需勾选）
--   multipart/流式 3 条裸路由的 operation 也在内（upload / breakpointContinue / download）。
-- ---------------------------------------------------------------------------
INSERT INTO `sys_apis` (`path`, `description`, `api_group`, `method`, `created_at`, `updated_at`)
SELECT t.path, t.descr, '媒体库', t.method, NOW(), NOW()
FROM (
  SELECT '/api.media.v1.AttachmentCategoryService/GetCategoryList' AS path, '媒体库-分类树' AS descr, 'GET' AS method
  UNION ALL SELECT '/api.media.v1.AttachmentCategoryService/AddCategory', '媒体库-新增/编辑分类', 'POST'
  UNION ALL SELECT '/api.media.v1.AttachmentCategoryService/DeleteCategory', '媒体库-删除分类', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/GetFileList', '媒体库-文件列表', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/EditFileName', '媒体库-文件改名', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/DeleteFile', '媒体库-删除文件', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/DeleteFiles', '媒体库-批量删除文件', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/ImportURL', '媒体库-导入外链', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/UploadFile', '媒体库-普通上传（multipart 裸路由 /fileUploadAndDownload/upload）', 'POST'
  UNION ALL SELECT '/api.media.v1.FileUploadAndDownloadService/DownloadFile', '媒体库-下载（流式裸路由 /fileUploadAndDownload/download）', 'GET'
  UNION ALL SELECT '/api.media.v1.BreakpointContinueService/FindFile', '媒体库-断点续传会话', 'GET'
  UNION ALL SELECT '/api.media.v1.BreakpointContinueService/BreakpointContinue', '媒体库-分片上传（multipart 裸路由 /fileUploadAndDownload/breakpointContinue）', 'POST'
  UNION ALL SELECT '/api.media.v1.BreakpointContinueService/BreakpointContinueFinish', '媒体库-分片合并', 'POST'
  UNION ALL SELECT '/api.media.v1.BreakpointContinueService/RemoveChunk', '媒体库-清理分片', 'POST'
) t
WHERE NOT EXISTS (
  SELECT 1 FROM `sys_apis` a WHERE a.`path` = t.path AND a.`method` = t.method AND a.`deleted_at` IS NULL
);

-- ---------------------------------------------------------------------------
-- 自检（执行后人工核对，期望 3 行菜单 / 每个角色 3 行授权 / 14 行 API）
-- ---------------------------------------------------------------------------
-- SELECT id, menu_name, parent_id, path, component, menu_type, hidden, permission
--   FROM sys_menus
--  WHERE path IN ('/media','/media/upload','/media/file') AND deleted_at IS NULL
--  ORDER BY parent_id, sort;
-- SELECT role_id, COUNT(*) FROM sys_role_menus
--  WHERE menu_id IN (SELECT id FROM sys_menus WHERE path IN ('/media','/media/upload','/media/file'))
--  GROUP BY role_id;
-- SELECT COUNT(*) FROM sys_apis WHERE api_group = '媒体库' AND deleted_at IS NULL;
