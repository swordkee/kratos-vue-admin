import request from '@/utils/request';


/**
 * 获取验证码
 * @param params 要传的参数值
 * @returns 返回接口数据
 */
export function captcha() {
	return request({
		url: '/system/user/getCaptcha',
		method: 'get',
	});
}

/**
 * 用户登录
 * @param params 要传的参数值
 * @returns 返回接口数据
 */
export function signIn(params: object) {
	return request({
		url: '/system/user/login',
		method: 'post',
		data: params,
	});
}

/**
 * 手机验证码登录第二步（login_method=2）：短信待验证 token + 短信码 → 正式 token
 * @param mfaToken 登录第一步返回的 mfaToken（needSms=true 时）
 * @param code 短信验证码
 */
export function smsVerifyLogin(mfaToken: string, code: string) {
	return request({
		url: '/system/user/phone/verify',
		method: 'post',
		data: { mfaToken, code },
	});
}

/**
 * 用户退出登录
 * @param params 要传的参数值
 * @returns 返回接口数据
 */
export function signOut(params: object) {
	return request({
		url: '/system/user/logout',
		method: 'post',
		data: params,
	});
}
