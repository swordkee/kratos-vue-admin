import request from '@/utils/request';

/**
 * TOTP 双因素认证 API（对应 admin api/admin/v1 sys_mfa 协议）
 * 说明：KVA 模板 request 拦截器与 go-exAdmin 一致，成功时返回解包后的 data，
 * 因此各接口直接拿到业务对象（如 { available, enabled, boundAt }）。
 */

/**
 * 查询当前用户 TOTP 状态
 * @returns { available: boolean, enabled: boolean, boundAt: number }
 */
export function mfaStatus() {
	return request({
		url: '/mfa/status',
		method: 'get',
	});
}

/**
 * 生成 TOTP 密钥与 otpauth URI（扫码绑定第一步）
 * @param username otpauth AccountName（一般传登录用户名）
 * @returns { secret: string, url: string }
 */
export function mfaEnroll(username: string) {
	return request({
		url: '/mfa/enroll',
		method: 'post',
		data: { username },
	});
}

/**
 * 校验动态码并启用（绑定第二步），返回一次性恢复码
 * @param code 动态验证码
 * @returns { recoveryCodes: string[] }
 */
export function mfaEnable(code: string) {
	return request({
		url: '/mfa/enable',
		method: 'post',
		data: { code },
	});
}

/**
 * 关闭 TOTP（需动态码或恢复码二次确认）
 * @param code 动态码或恢复码
 */
export function mfaDisable(code: string) {
	return request({
		url: '/mfa/disable',
		method: 'post',
		data: { code },
	});
}

/**
 * 管理员重置指定用户 TOTP
 * @param userId 目标用户 ID
 */
export function mfaReset(userId: number) {
	return request({
		url: '/mfa/reset',
		method: 'post',
		data: { userId },
	});
}

/**
 * 两段式登录二次验证：MfaPending token + 动态码/恢复码 → 正式 token
 * @param mfaToken 登录第一步返回的待验证 token
 * @param code 动态码或恢复码
 * @returns { token: string, expire: number }
 */
export function mfaVerify(mfaToken: string, code: string) {
	return request({
		url: '/mfa/verify',
		method: 'post',
		data: { mfaToken, code },
	});
}
