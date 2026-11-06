/**
 * 媒体库前端小工具（模板版新增；wallpaper/go-kva-admin 里这些能力散在 utils/wallpaper.ts，
 * 模板不引入壁纸域工具，故媒体页专用函数集中在本文件）。
 */

/** 毫秒时间戳 → `YYYY-MM-DD HH:mm:ss`；非法/空值返回空串。 */
export function formatMillis(ms: any): string {
	if (ms === undefined || ms === null || ms === '') return '';
	const n = Number(ms);
	if (!isFinite(n) || n <= 0) return '';
	const d = new Date(n);
	if (isNaN(d.getTime())) return '';
	const p = (x: number) => (x < 10 ? '0' + x : '' + x);
	return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

/** 字节数 → 人类可读（B/KB/MB/GB）；列表「大小」列用。 */
export function formatBytes(size: any): string {
	const n = Number(size);
	if (!isFinite(n) || n <= 0) return '0 B';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let v = n;
	let i = 0;
	while (v >= 1024 && i < units.length - 1) {
		v /= 1024;
		i++;
	}
	return `${i === 0 ? v : v.toFixed(2)} ${units[i]}`;
}
