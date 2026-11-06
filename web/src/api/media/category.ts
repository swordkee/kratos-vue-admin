import request from '@/utils/request';

// 媒体库分类（/attachmentCategory/*；表 exa_attachment_category）
// 源 web/src/api/attachmentCategory.js 同路径迁移。

// 分类树（源 getCategoryList：返回树，children 嵌套）
export function getCategoryList() {
	return request({
		url: '/attachmentCategory/getCategoryList',
		method: 'get',
	});
}

// 添加/编辑分类（id>0 视为更新；同父同名后端拒绝「分类名称已存在」）
export function addCategory(data: { id?: number | string; name: string; pid: number | string }) {
	return request({
		url: '/attachmentCategory/addCategory',
		method: 'post',
		data: {
			...(data.id ? { id: Number(data.id) } : {}),
			name: data.name,
			pid: Number(data.pid || 0),
		},
	});
}

// 删除分类（有子级后端拒绝「请先删除子级」）
export function deleteCategory(id: number | string) {
	return request({
		url: '/attachmentCategory/deleteCategory',
		method: 'post',
		data: { id: Number(id) },
	});
}
