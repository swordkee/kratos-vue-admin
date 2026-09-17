import axios from 'axios';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Session } from '@/utils/storage';

// 响应侧为 camelCase 追加了 snake_case 别名；页面若 spread 整行提交，请求体会同时
// 含 camel 与 snake 双键（protojson 可能判重复字段）。这里在发送前丢弃「已有 snake
// 对应键」的 camelCase 副本（仅当两者并存时），避免重复字段错误。
function dedupeCamelAliases(value: any): any {
	if (Array.isArray(value)) return value.map(dedupeCamelAliases);
	if (value && typeof value === 'object') {
		const out: Record<string, any> = {};
		for (const k of Object.keys(value)) {
			out[k] = dedupeCamelAliases((value as any)[k]);
		}
		for (const k of Object.keys(out)) {
			const snake = camelToSnake(k);
			if (snake !== k && Object.prototype.hasOwnProperty.call(out, snake)) {
				delete out[k];
			}
		}
		return out;
	}
	return value;
}

// 根因收口：protojson 将 int64 序列化为「字符串」（如 commodityId:"1"），
// 列表/详情回填表单后原样回传，后端按 Go struct int64 反序列化即报
// "json: cannot unmarshal string into Go struct field ... of type int64"（HTTP 500）。
// 覆盖：标量、ID 数组、嵌套对象。仅在 JS 安全整数范围内转 number（超范围保持字符串
// 避免精度丢失——服务端 protojson 同时接受 int64 的字符串/数字两种形态）；
// FormData/Blob 等二进制体不处理。
function coerceInt64String(v: string): string | number {
	if (!/^-?\d+$/.test(v)) return v;
	const n = Number(v);
	return Number.isSafeInteger(n) && String(n) === v ? n : v;
}

function coerceProtoInt64(value: any): any {
	if (Array.isArray(value)) return value.map(coerceProtoInt64);
	if (value && typeof value === 'object') {
		const out: Record<string, any> = {};
		for (const k of Object.keys(value)) {
			const v = (value as any)[k];
			if (/(_id|Id)$/.test(k)) {
				// 标量 int64（position_id:"1"）；数组值按同名单值数组逐项处理
				if (typeof v === 'string') out[k] = coerceInt64String(v);
				else if (Array.isArray(v)) out[k] = v.map((item) => typeof item === 'string' ? coerceInt64String(item) : coerceProtoInt64(item));
				else out[k] = coerceProtoInt64(v);
			} else if (/ids$/i.test(k) && Array.isArray(v)) {
				// repeated int64（order_ids/position_ids/user_ids/settle_member_ids/menuIds/ids…）
				out[k] = v.map((item) => typeof item === 'string' ? coerceInt64String(item) : coerceProtoInt64(item));
			} else {
				out[k] = coerceProtoInt64(v);
			}
		}
		return out;
	}
	return value;
}

// 根因收口（响应侧）：kratos/protojson 输出 camelCase（如 customerId、createdTime），
// 而本前端页面普遍按 snake_case 读取（row.customer_id、row.created_at）→ 列表/详情/编辑
// 回填全空。这里为每个 camelCase 键**追加** snake_case 别名（不改原键、已存在则不覆盖），
// 使两种读取约定都可用；并补 createdTime→created_at / updatedTime→updated_at 特例。
function camelToSnake(s: string): string {
	return s.replace(/([A-Z])/g, (m) => '_' + m.toLowerCase());
}
const SNAKE_ALIAS_EXTRA: Record<string, string[]> = {
	createdTime: ['created_at'],
	updatedTime: ['updated_at'],
};
function addSnakeAliases(value: any): any {
	if (Array.isArray(value)) return value.map(addSnakeAliases);
	if (value && typeof value === 'object') {
		const out: Record<string, any> = {};
		for (const k of Object.keys(value)) {
			out[k] = addSnakeAliases((value as any)[k]);
		}
		for (const k of Object.keys(out)) {
			const snake = camelToSnake(k);
			if (snake !== k && out[snake] === undefined) out[snake] = out[k];
			const extras = SNAKE_ALIAS_EXTRA[k];
			if (extras) {
				for (const e of extras) {
					if (out[e] === undefined) out[e] = out[k];
				}
			}
		}
		return out;
	}
	return value;
}

// 配置新建一个 axios 实例
const service = axios.create({
	baseURL: import.meta.env.VITE_API_URL as any,
	timeout: 50000,
	headers: { 'Content-Type': 'application/json' },
});

// 添加请求拦截器
service.interceptors.request.use(
	(config: any) => {
		// 在发送请求之前做些什么 token
		if (Session.get('token')) {
			config.headers!['Authorization'] = `Bearer ${Session.get('token')}`;
		}
		// protojson int64 字符串 → number（仅 JSON 请求体，跳过 FormData/Blob 等）；
		// 同时去除 camel/snake 同名字段双写。
		if (
			config.data &&
			typeof config.data === 'object' &&
			!(config.data instanceof FormData) &&
			!(config.data instanceof Blob)
		) {
			config.data = coerceProtoInt64(dedupeCamelAliases(config.data));
		}
		// 分页参数兼容：部分 GET 列表接口 proto 字段为 page_num，而页面统一传 page。
		// 对 GET query 同时补发 page_num（未知 query 键会被 Kratos 标准 encoding/json
		// 解码器忽略，对以 page 为字段的端点无副作用）。
		if (config.params && typeof config.params === 'object'
			&& config.params.page !== undefined
			&& config.params.page_num === undefined
			&& config.params.pageNum === undefined) {
			config.params.page_num = config.params.page;
		}
		return config;
	},
	(error) => {
		// 对请求错误做些什么
		return Promise.reject(error);
	}
);

// 添加响应拦截器
service.interceptors.response.use(
	(response) => {
		// 对响应数据做点什么
		const res = response.data;

		// 如果没有 code，直接返回数据（可能是文件下载等）
		// 注意：KVA 存量页面（登录/用户信息等）依赖完整 axios response 的 .data 读取，
		// 此处刻意保留模板原行为（返回 response），不改成 exAdmin 的 response.data。
		if (!res.code) {
			return response;
		}

		// 成功响应
		if (res.code === 200 || res.code === 0) {
			// 如果有警告消息，显示警告
			if (res.message && res.message !== 'success') {
				ElMessage.warning(res.message);
			}
			// 返回 data 字段的内容，解包 Kratos 响应格式；
			// 兜底 {} 防止调用方直接取属性时 "null has no properties"
			// 附带 camelCase→snake_case 别名，兼容页面既有 snake_case 读取。
			return addSnakeAliases(res.data ?? response.data.data ?? {});
		}

		// 业务错误处理
		// 认证相关错误 (100-104, 401)
		if (res.code === 401 || res.code === 4001 || (res.code >= 100 && res.code <= 104)) {
			Session.clear();
			ElMessageBox.alert('登录已过期，请重新登录', '提示', {
				confirmButtonText: '确定',
				type: 'warning',
			}).then(() => {
				window.location.href = '/';
			}).catch(() => {
				window.location.href = '/';
			});
			return Promise.reject(new Error('登录已过期'));
		}

		// 权限错误 (403, 105)
		if (res.code === 403 || res.code === 105) {
			ElMessage.error(res.msg || res.message || '无权限访问该资源');
			return Promise.reject(new Error('无权限访问'));
		}

		// 参数错误 (400, 6, 7)
		if (res.code === 400 || res.code === 6 || res.code === 7) {
			ElMessage.error(res.msg || res.message || '请求参数错误');
			return Promise.reject(new Error('参数错误'));
		}

		// 其他业务错误，显示错误消息
		const errorMessage = res.msg || res.message || '操作失败';
		ElMessage.error(errorMessage);
		return Promise.reject(new Error(errorMessage));
	},
	(error) => {
		// HTTP 错误处理
		const { response } = error;

		if (!response) {
			// 网络错误
			if (error.message?.includes('timeout')) {
				ElMessage.error('网络请求超时，请稍后重试');
			} else if (error.message === 'Network Error') {
				ElMessage.error('网络连接错误，请检查网络');
			} else {
				ElMessage.error('网络异常，请稍后重试');
			}
			return Promise.reject(error);
		}

		const { status, data } = response;

		switch (status) {
			case 400:
				ElMessage.error(data?.message || '请求参数错误');
				break;
			case 401:
				Session.clear();
				ElMessageBox.alert('登录已过期，请重新登录', '提示', {
					confirmButtonText: '确定',
					type: 'warning',
				}).then(() => {
					window.location.href = '/';
				});
				break;
			case 403:
				ElMessage.error('无权限访问该资源');
				break;
			case 404:
				ElMessage.error('请求的资源不存在');
				break;
			case 405:
				ElMessage.error('请求方法不允许');
				break;
			case 408:
				ElMessage.error('请求超时');
				break;
			case 409:
				// TOTP 强制绑定：reason=MFA_ENROLL_REQUIRED 且用户未绑定 → 引导跳个人中心绑定
				if (data?.reason === 'MFA_ENROLL_REQUIRED' || data?.message?.includes('二因素')) {
					ElMessageBox.alert('请先绑定二因素认证（TOTP）', '安全提示', {
						confirmButtonText: '去绑定',
						type: 'warning',
					}).then(() => {
						window.location.href = '/personal';
					}).catch(() => {
						window.location.href = '/personal';
					});
					break;
				}
				ElMessage.error(data?.message || '请求冲突');
				break;
			case 500:
				ElMessage.error('服务器内部错误');
				break;
			case 502:
				ElMessage.error('网关错误');
				break;
			case 503:
				ElMessage.error('服务不可用');
				break;
			case 504:
				ElMessage.error('网关超时');
				break;
			default:
				ElMessage.error(data?.message || `请求失败: ${status}`);
		}

		return Promise.reject(error);
	}
);

// 导出 axios 实例
export default service;
