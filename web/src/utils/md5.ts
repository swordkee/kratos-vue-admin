/**
 * 纯 TS MD5（无第三方依赖，分片上传的秒传/整文件校验用；后端 biz/media 以 md5 hex 比对）。
 * 输入 ArrayBuffer，输出小写 hex。仅用于文件完整性标识，非安全场景。
 * 实现参照经典 Joseph Myers/blueimp MD5（arraybuffer 变体），与 crypto-js/SparkMD5 结果一致。
 * 与 go-kvAdmin 同名文件同源（同产品线，口径必须一致）。
 */

function safeAdd(x: number, y: number): number {
	const lsw = (x & 0xffff) + (y & 0xffff);
	const msw = (x >> 16) + (y >> 16) + (lsw >> 16);
	return (msw << 16) | (lsw & 0xffff);
}

function bitRol(num: number, cnt: number): number {
	return (num << cnt) | (num >>> (32 - cnt));
}

function md5cycle(x: Int32Array, k: Int32Array) {
	let a = x[0];
	let b = x[1];
	let c = x[2];
	let d = x[3];

	a = ff(a, b, c, d, k[0], 7, -680876936);
	d = ff(d, a, b, c, k[1], 12, -389564586);
	c = ff(c, d, a, b, k[2], 17, 606105819);
	b = ff(b, c, d, a, k[3], 22, -1044525330);
	a = ff(a, b, c, d, k[4], 7, -176418897);
	d = ff(d, a, b, c, k[5], 12, 1200080426);
	c = ff(c, d, a, b, k[6], 17, -1473231341);
	b = ff(b, c, d, a, k[7], 22, -45705983);
	a = ff(a, b, c, d, k[8], 7, 1770035416);
	d = ff(d, a, b, c, k[9], 12, -1958414417);
	c = ff(c, d, a, b, k[10], 17, -42063);
	b = ff(b, c, d, a, k[11], 22, -1990404162);
	a = ff(a, b, c, d, k[12], 7, 1804603682);
	d = ff(d, a, b, c, k[13], 12, -40341101);
	c = ff(c, d, a, b, k[14], 17, -1502002290);
	b = ff(b, c, d, a, k[15], 22, 1236535329);

	a = gg(a, b, c, d, k[1], 5, -165796510);
	d = gg(d, a, b, c, k[6], 9, -1069501632);
	c = gg(c, d, a, b, k[11], 14, 643717713);
	b = gg(b, c, d, a, k[0], 20, -373897302);
	a = gg(a, b, c, d, k[5], 5, -701558691);
	d = gg(d, a, b, c, k[10], 9, 38016083);
	c = gg(c, d, a, b, k[15], 14, -660478335);
	b = gg(b, c, d, a, k[4], 20, -405537848);
	a = gg(a, b, c, d, k[9], 5, 568446438);
	d = gg(d, a, b, c, k[14], 9, -1019803690);
	c = gg(c, d, a, b, k[3], 14, -187363961);
	b = gg(b, c, d, a, k[8], 20, 1163531501);
	a = gg(a, b, c, d, k[13], 5, -1444681467);
	d = gg(d, a, b, c, k[2], 9, -51403784);
	c = gg(c, d, a, b, k[7], 14, 1735328473);
	b = gg(b, c, d, a, k[12], 20, -1926607734);

	a = hh(a, b, c, d, k[5], 4, -378558);
	d = hh(d, a, b, c, k[8], 11, -2022574463);
	c = hh(c, d, a, b, k[11], 16, 1839030562);
	b = hh(b, c, d, a, k[14], 23, -35309556);
	a = hh(a, b, c, d, k[1], 4, -1530992060);
	d = hh(d, a, b, c, k[4], 11, 1272893353);
	c = hh(c, d, a, b, k[7], 16, -155497632);
	b = hh(b, c, d, a, k[10], 23, -1094730640);
	a = hh(a, b, c, d, k[13], 4, 681279174);
	d = hh(d, a, b, c, k[0], 11, -358537222);
	c = hh(c, d, a, b, k[3], 16, -722521979);
	b = hh(b, c, d, a, k[6], 23, 76029189);
	a = hh(a, b, c, d, k[9], 4, -640364487);
	d = hh(d, a, b, c, k[12], 11, -421815835);
	c = hh(c, d, a, b, k[15], 16, 530742520);
	b = hh(b, c, d, a, k[2], 23, -995338651);

	a = ii(a, b, c, d, k[0], 6, -198630844);
	d = ii(d, a, b, c, k[7], 10, 1126891415);
	c = ii(c, d, a, b, k[14], 15, -1416354905);
	b = ii(b, c, d, a, k[5], 21, -57434055);
	a = ii(a, b, c, d, k[12], 6, 1700485571);
	d = ii(d, a, b, c, k[3], 10, -1894986606);
	c = ii(c, d, a, b, k[10], 15, -1051523);
	b = ii(b, c, d, a, k[1], 21, -2054922799);
	a = ii(a, b, c, d, k[8], 6, 1873313359);
	d = ii(d, a, b, c, k[15], 10, -30611744);
	c = ii(c, d, a, b, k[6], 15, -1560198380);
	b = ii(b, c, d, a, k[13], 21, 1309151649);
	a = ii(a, b, c, d, k[4], 6, -145523070);
	d = ii(d, a, b, c, k[11], 10, -1120210379);
	c = ii(c, d, a, b, k[2], 15, 718787259);
	b = ii(b, c, d, a, k[9], 21, -343485551);

	x[0] = safeAdd(a, x[0]);
	x[1] = safeAdd(b, x[1]);
	x[2] = safeAdd(c, x[2]);
	x[3] = safeAdd(d, x[3]);
}

function cmn(q: number, a: number, b: number, x: number, s: number, t: number) {
	return safeAdd(bitRol(safeAdd(safeAdd(a, q), safeAdd(x, t)), s), b);
}
function ff(a: number, b: number, c: number, d: number, x: number, s: number, t: number) {
	return cmn((b & c) | (~b & d), a, b, x, s, t);
}
function gg(a: number, b: number, c: number, d: number, x: number, s: number, t: number) {
	return cmn((b & d) | (c & ~d), a, b, x, s, t);
}
function hh(a: number, b: number, c: number, d: number, x: number, s: number, t: number) {
	return cmn(b ^ c ^ d, a, b, x, s, t);
}
function ii(a: number, b: number, c: number, d: number, x: number, s: number, t: number) {
	return cmn(c ^ (b | ~d), a, b, x, s, t);
}

/** 计算 ArrayBuffer 的 MD5（小写 hex） */
export function md5ArrayBuffer(buf: ArrayBuffer): string {
	// 尾部留 9 字节（0x80 + 8 字节长度）
	const n = buf.byteLength;
	const blocks = ((n + 8) >> 6) + 1;
	const bytes = new Uint8Array(blocks * 64);
	bytes.set(new Uint8Array(buf));
	bytes[n] = 0x80;

	// 以小端 Int32 视图处理（需 4 字节对齐：blocks*64 必为 4 的倍数）
	const words = new Int32Array(bytes.buffer);
	// 长度位：bit 数，小端写入最后两个字
	words[blocks * 16 - 2] = (n * 8) & 0xffffffff;
	words[blocks * 16 - 1] = n < 0x1fffffffe ? 0 : Math.floor(n / 0x20000000) >>> 0;

	const state = new Int32Array([1732584193, -271733879, -1732584194, 271733878]);
	for (let i = 0; i < blocks; i++) {
		md5cycle(state, words.subarray(i * 16, i * 16 + 16) as unknown as Int32Array);
	}

	const hexChars = '0123456789abcdef';
	let out = '';
	for (let i = 0; i < 4; i++) {
		const word = state[i];
		for (let j = 0; j < 4; j++) {
			const byte = (word >>> (j * 8)) & 0xff;
			out += hexChars[(byte >>> 4) & 0x0f] + hexChars[byte & 0x0f];
		}
	}
	return out;
}

/** 计算 Blob 的 MD5（小写 hex） */
export async function md5Blob(blob: Blob): Promise<string> {
	return md5ArrayBuffer(await blob.arrayBuffer());
}
