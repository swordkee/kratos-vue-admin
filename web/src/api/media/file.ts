import request from '@/utils/request';
import { Session } from '@/utils/storage';

// 媒体库文件（/fileUploadAndDownload/*；表 exa_file_upload_and_downloads / exa_files / exa_file_chunks）
// 源 web/src/api/{fileUploadAndDownload,breakpoint}.js 同路径迁移。
//
// 注意：普通上传（/upload）、分片（/breakpointContinue）、下载（/download）是 multipart/流式
// 裸路由，不走 JSON 信封；上传用 el-upload 的 action 直连，下载走 blob。

// 分页列表（keyword 模糊 name；源端 ORDER BY updated_at DESC）
export function getFileList(params: { page?: number; pageSize?: number; keyword?: string }) {
	return request({
		url: '/fileUploadAndDownload/getFileList',
		method: 'post',
		data: {
			page: Number(params.page || 1),
			pageSize: Number(params.pageSize || 10),
			...(params.keyword ? { keyword: params.keyword } : {}),
		},
	});
}

// 编辑文件名/备注
export function editFileName(data: { id: number | string; name: string }) {
	return request({
		url: '/fileUploadAndDownload/editFileName',
		method: 'post',
		data: { id: Number(data.id), name: data.name },
	});
}

// 删除单条（记录 + 存储对象）
export function deleteFile(id: number | string) {
	return request({
		url: '/fileUploadAndDownload/deleteFile',
		method: 'post',
		data: { id: Number(id) },
	});
}

// 批量删除（失败项进 failedIds，不中断）
export function deleteFiles(ids: Array<number | string>) {
	return request({
		url: '/fileUploadAndDownload/deleteFiles',
		method: 'post',
		data: { ids: ids.map((i) => Number(i)) },
	});
}

// 导入外部 URL（批量建档，不下载对象）
export function importURL(list: Array<{ name?: string; url: string; tag?: string; key?: string }>) {
	return request({
		url: '/fileUploadAndDownload/importURL',
		method: 'post',
		data: { list },
	});
}

// 分片会话查询/创建（源 findFile：同指纹已完成 ⇒ 秒传；否则回带已上传分片）
export function findFile(params: { fileMd5: string; fileName: string; chunkTotal: number }) {
	return request({
		url: '/fileUploadAndDownload/findFile',
		method: 'get',
		params,
	});
}

// 分片上传（multipart；chunkMd5 支持 "sha256:<hex>"，浏览器无 MD5）
export function breakpointContinue(form: FormData) {
	return request({
		url: '/fileUploadAndDownload/breakpointContinue',
		method: 'post',
		data: form,
		timeout: 0,
	});
}

// 分片合并（返回对象键/访问地址/大小/建档 id）
export function breakpointContinueFinish(params: { fileMd5: string; fileName: string }) {
	return request({
		url: '/fileUploadAndDownload/breakpointContinueFinish',
		method: 'post',
		params,
		timeout: 0,
	});
}

// 合并后清缓存分片
export function removeChunk(data: { fileMd5: string; filePath?: string }) {
	return request({
		url: '/fileUploadAndDownload/removeChunk',
		method: 'post',
		data,
	});
}

// 访问地址归一：本地存储的 url 形如 /static/<key>，浏览器要经 API 前缀访问
//   dev  → VITE_API_URL=/api，由 vite proxy 的 rewrite 去掉 /api 后到达后端 /static/；
//   生产 → VITE_API_URL 为完整域名，直接拼成 https://host/static/<key>。
// 云端（aliyun）url 本身就是 CDN 直链，原样返回。
export function mediaUrl(url: string) {
	if (!url) return '';
	if (url.startsWith('/static/')) return `${import.meta.env.VITE_API_URL || ''}${url}`;
	return url;
}

// 复制链接用的绝对地址（含 origin）
export function mediaAbsoluteUrl(url: string) {
	const u = mediaUrl(url);
	if (/^https?:\/\//.test(u)) return u;
	return `${window.location.origin}${u}`;
}

// 普通上传直连地址（el-upload action 用；noSave=0 落库）
export function uploadAction() {
	return `${import.meta.env.VITE_API_URL}/fileUploadAndDownload/upload`;
}

// 上传请求头（el-upload headers）
export function uploadHeaders() {
	const token = Session.get('token');
	return token ? { Authorization: `Bearer ${token}` } : {};
}

// 下载：直接取 blob（本地存储流式回包；aliyun 会 302 到直链，浏览器自动跟随）
export function downloadFile(id: number | string) {
	return request({
		url: '/fileUploadAndDownload/download',
		method: 'get',
		params: { id: Number(id) },
		responseType: 'blob',
	});
}

// 保存 blob 到本地（文件名取记录 name）
export function saveBlob(blob: Blob, filename: string) {
	const url = window.URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename || 'download';
	document.body.appendChild(a);
	a.click();
	document.body.removeChild(a);
	window.URL.revokeObjectURL(url);
}
