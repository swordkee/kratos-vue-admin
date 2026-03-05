import axios from 'axios';
import { ElMessage, ElMessageBox } from 'element-plus';
import { Session } from '@/utils/storage';

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
		if (!res.code) {
			return response;
		}

		// 成功响应
		if (res.code === 200 || res.code === 0) {
			// 如果有警告消息，显示警告
			if (res.message && res.message !== 'success') {
				ElMessage.warning(res.message);
			}
			// 返回 data 字段的内容，解包 Kratos 响应格式
			return res.data || response.data.data;
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
