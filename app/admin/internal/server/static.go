package server

import (
	"errors"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"
)

// newLocalStaticHandler 本地存储的公开静态面（/static/*）。
//
// 与源 api-admin 的 gin static 语义对齐：媒体图片需要能被 <img> 直接预览、能复制链接给运营用。
// 安全要点：
//   - 只服务 root 之内的文件（Clean + Rel 双校验，`..` 一律 403）；
//   - 只允许 GET/HEAD；
//   - 不列目录（路径为目录时 404）。
func newLocalStaticHandler(root string) nethttp.Handler {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	fs := nethttp.FileServer(nethttp.Dir(absRoot))
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet && r.Method != nethttp.MethodHead {
			w.WriteHeader(nethttp.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/static/")
		if name == "" || strings.Contains(name, "..") {
			w.WriteHeader(nethttp.StatusForbidden)
			return
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		full := filepath.Join(absRoot, clean)
		if rel, rerr := filepath.Rel(absRoot, full); rerr != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
			w.WriteHeader(nethttp.StatusForbidden)
			return
		}
		info, statErr := os.Stat(full)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				w.WriteHeader(nethttp.StatusNotFound)
				return
			}
			w.WriteHeader(nethttp.StatusInternalServerError)
			return
		}
		if info.IsDir() {
			w.WriteHeader(nethttp.StatusNotFound)
			return
		}
		// 符号链接防御（审核 P2）：上面用的是字面路径，root 内若有软链指向根外，
		// Rel 校验看不出问题 ⇒ 先解析真实路径再按根内复判一次。
		if !withinRoot(absRoot, full) {
			w.WriteHeader(nethttp.StatusForbidden)
			return
		}
		// 回写 /static/ 前缀目录结构后交给 FileServer（其自身也有路径清理）。
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + clean
		fs.ServeHTTP(w, r2)
	})
}

// withinRoot 判定 full（已存在）的真实路径是否仍在 root 之内。
//
// 用「解析真实路径 + 逐级向上做 os.SameFile 同文件判定」而不是比较路径字符串：
// 路径字符串比较在 Windows 上会被盘符大小写 / 8.3 短名 / 目录联接的目标写法干扰，
// 既可能漏判（安全洞）也可能误拒（根内软链被当成逃逸）。SameFile 看的是文件身份。
// 解析失败（链接悬空、目录联接 junction 等）按「不在根内」处理（fail-closed）。
func withinRoot(root, full string) bool {
	realFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false
	}
	for dir := realFull; ; {
		if info, statErr := os.Stat(dir); statErr == nil && os.SameFile(info, rootInfo) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
