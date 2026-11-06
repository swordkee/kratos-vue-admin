package server

import (
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// 本地静态面（/static/*）正向锁：根内文件可匿名读，目录/空路径不列。
func TestLocalStaticHandlerServesInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("准备文件失败: %v", err)
	}

	h := newLocalStaticHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/static/a.txt", nil))
	if rec.Code != nethttp.StatusOK {
		t.Fatalf("根内文件应可读: HTTP %d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("响应体不符: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/static/", nil))
	if rec.Code != nethttp.StatusForbidden {
		t.Fatalf("目录/空路径应 403: HTTP %d", rec.Code)
	}
}

// 审核 P2 反例锁：根内链接指向根外目录 ⇒ 403，根外文件内容不得泄漏。
// 字面路径仍落在 root 内（Clean+Rel 校验看不出问题），只有解析真实路径才拦得住。
func TestLocalStaticHandlerBlocksLinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("top-secret"), 0o644); err != nil {
		t.Fatalf("准备根外文件失败: %v", err)
	}
	makeDirLink(t, outside, filepath.Join(root, "escape"))

	h := newLocalStaticHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(nethttp.MethodGet, "/static/escape/secret.txt", nil))
	if rec.Code != nethttp.StatusForbidden {
		t.Fatalf("链接逃逸应 403: HTTP %d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "top-secret" {
		t.Fatal("根外文件内容被泄漏")
	}
}

// withinRoot 双向锁（不经 handler，直接锁判据）：根内真实文件 true / 根外文件 false。
func TestWithinRootDecision(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatalf("准备目录失败: %v", err)
	}
	file := filepath.Join(inside, "x.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o644); err != nil {
		t.Fatalf("准备文件失败: %v", err)
	}
	if !withinRoot(root, file) {
		t.Fatal("根内文件应判为根内")
	}

	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("no"), 0o644); err != nil {
		t.Fatalf("准备根外文件失败: %v", err)
	}
	if withinRoot(root, secret) {
		t.Fatal("根外文件应判为根外")
	}
}

// 根内软链指向根内：仍应判为根内（不误伤合法软链）。
func TestWithinRootAllowsInnerSymlink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "x.txt")
	if err := os.WriteFile(file, []byte("ok"), 0o644); err != nil {
		t.Fatalf("准备文件失败: %v", err)
	}
	link := filepath.Join(root, "alias.txt")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("当前环境不支持创建符号链接（Windows 需开发者模式/管理员）: %v", err)
	}
	if !withinRoot(root, link) {
		t.Fatal("根内软链应判为根内")
	}
}

// makeDirLink 建「目录链接」：优先 os.Symlink；Windows 非管理员无符号链接特权时退回
// 目录联接（mklink /J，不需要特权）。两者都不可用则跳过（环境不支持，不算红）。
//
// 说明：Windows 目录联接对 Go 的 filepath.EvalSymlinks 会解析失败 ⇒ withinRoot 按
// fail-closed 拒绝；因此本函数只用于「逃逸必须被拒」的反例锁，不用于正向锁。
func makeDirLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	} else if runtime.GOOS != "windows" {
		t.Skipf("当前环境不支持创建符号链接: %v", err)
	}
	out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if jerr != nil {
		t.Skipf("符号链接与目录联接均不可用: %v (%s)", jerr, string(out))
	}
}
