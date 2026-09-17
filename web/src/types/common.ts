// 公共 API 响应类型
// 多个 api/system/*.ts 通过 `import type { BaseResponse, PageResponse } from '@/types/common'`
// 引用时使用。补齐全局通用类型。

// BaseResponse 通用操作响应（无业务数据，仅状态/消息）
export interface BaseResponse {
  success?: boolean
  code?: number
  message?: string
  [key: string]: unknown
}

// PageResponse 分页响应通用结构（列表 + 总数）
export interface PageResponse<T = unknown> {
  list?: T[]
  total?: number
  page?: number
  page_size?: number
  data?: T[]
  [key: string]: unknown
}
