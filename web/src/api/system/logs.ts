import request from '@/utils/request'

// 操作日志（sys_logs / LogsService）——与 go-payAdmin `web/admin/src/api/system/logs.ts` 同源。
// 后端：api/admin/v1/logs.proto 的 LogsService，HTTP 面 /system/logs/*。
// ⚠️ KVA 桩 `api/log/login.ts`（/log/logLogin/*）在 exAdmin 后端无对应路由（恒 404），
// 日志管理页应以本文件为准。

// 查询操作日志列表
export function listOperationRecord(query: any) {
	return request({
		url: '/system/logs/list',
		method: 'get',
		params: query
	})
}

// 清空操作日志
export function cleanOperationRecord() {
	return request({
		url: '/system/logs/clean',
		method: 'delete'
	})
}

// 操作单条日志
export function getOperationRecordByIds(ids: string) {
	return request({
		url: '/system/logs/' + ids,
		method: 'get'
	})
}

// 批量删除操作日志
export function delOperationRecordByIds(ids: string) {
	return request({
		url: '/system/logs/deleteByIds',
		method: 'delete',
		params: { ids }
	})
}
