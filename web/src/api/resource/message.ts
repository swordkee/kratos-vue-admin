import request from '@/utils/request';

// ========== 统一站内信（KVA 域 sys_message，对齐 api/admin/v1/message.proto）==========
// 身份口径：List 默认查自己（超管可传 uid 查他人）；详情/已读/删除/未读数一律服务端
// 按 JWT 定位收件人，请求不带 uid（带了也会被服务端丢弃）。
// 响应口径（模板拦截器 utils/request.ts）：外层 CommonReply{code:200} 解包后返回内层
// Rsp（本文件的 *Rsp 类型），字段再由拦截器补 snake_case 别名，页面可直读 is_read 等。

// 站内信行（sys_message 列全集，时间为毫秒时间戳，未发生=0；protojson int64 可能为字符串）
export interface MessageItem {
	id: number | string;
	uid: number;
	type: number; // 1系统 2订单 3结算 4投诉
	title: string;
	content: string;
	scene: string;
	param: string;
	sender_id: number;
	sender_name: string;
	is_read: number; // 0未读 1已读
	read_at: number | string;
	deleted_at: number | string;
	created_at: number | string;
	updated_at: number | string;
}

export interface ListMessagesParams {
	page?: number;
	page_size?: number;
	uid?: number; // 仅超管可查他人；缺省/0=自己
	type?: number; // 缺省=全部
	is_read?: number; // 0未读 1已读 -1=全部
}

export interface ListMessagesReply {
	code: number;
	msg: string;
	total: number;
	list: MessageItem[];
}

export interface MessageRsp {
	code: number;
	msg: string;
}

export interface SendMessageParams {
	uid: number; // 收件人
	type: number;
	title: string;
	content: string;
	scene?: string;
	param?: string;
}

const messageApi = {
	// 分页列表（type / is_read 过滤）
	list: (query: ListMessagesParams) =>
		request({
			url: '/system/message/list',
			method: 'get',
			params: query,
		}) as unknown as Promise<ListMessagesReply>,
	// 未读数（角标）
	unread: () =>
		request({
			url: '/system/message/unread',
			method: 'get',
		}) as unknown as Promise<MessageRsp & { count: number }>,
	// 详情
	info: (id: number | string) =>
		request({
			url: `/system/message/info/${id}`,
			method: 'get',
		}) as unknown as Promise<MessageRsp & { data: MessageItem }>,
	// 单条已读（不带 uid）
	markRead: (id: number | string) =>
		request({
			url: `/system/message/read/${id}`,
			method: 'put',
		}) as unknown as Promise<MessageRsp>,
	// 全部已读
	markAllRead: () =>
		request({
			url: '/system/message/readAll',
			method: 'put',
		}) as unknown as Promise<MessageRsp & { updated: number }>,
	// 删除（软删）
	remove: (id: number | string) =>
		request({
			url: `/system/message/${id}`,
			method: 'delete',
		}) as unknown as Promise<MessageRsp>,
	// 发送（管理面发信，可指定任意收件人）
	send: (data: SendMessageParams) =>
		request({
			url: '/system/message/send',
			method: 'post',
			data: data,
		}) as unknown as Promise<MessageRsp & { id: number }>,
};

export default messageApi;
