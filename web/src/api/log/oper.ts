import request from '@/utils/request';

// 操作日志（sys_logs / LogsService）。
//
// 2026-10-05：原桩指向 /log/logOper/*，而模板后端只实现了 LogsService 的
// /system/logs/*（sys_logs 表）⇒ 原桩恒 404。参照 go-payAdmin
// （web/admin/src/api/log/oper.ts 已同样改指 /system/logs/list）改为读 sys_logs。
// 页面见 web/src/views/system/logs/index.vue（api/system/logs.ts 为对应封装）。

// 查询操作日志列表
export function listOperInfo(query:any) {
    return request({
        url: '/system/logs/list',
        method: 'get',
        params: query
    })
}

// 删除（批量，ids 逗号分隔）
export function delOperInfo(ids:any) {
    const s = Array.isArray(ids) ? ids.join(',') : String(ids)
    return request({
        url: '/system/logs/deleteByIds',
        method: 'delete',
        params: { ids: s }
    })
}

// 清空操作日志
export function cleanOpernfo() {
    return request({
        url: '/system/logs/clean',
        method: 'delete',
    })
}
