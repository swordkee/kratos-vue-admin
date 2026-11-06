package media

// 媒体库纯函数锁（无 DB、无 IO）：对象键生成/校验、摘要解析、分类树组装。
//
// 这些判据是整个断点续传完整性链的底座：
//   - parseDigest 决定「申报摘要能否被接受」，空串/长度错必须 fail-closed（审核 C3 的根因）；
//   - ValidateKey 是本地存储与 OSS 两条路径的共同前置，必须拦住 `..`、反斜杠、盘符；
//   - BuildKey 命名需与源端 utils/upload/local.go 同构（md5(基名)_时间戳.ext）。

import (
	"strings"
	"testing"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

func TestParseDigestAcceptsTwoForms(t *testing.T) {
	const md5Hex = "d41d8cd98f00b204e9800998ecf8427e"
	const sha256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	cases := []struct {
		name string
		in   string
		ok   bool
		alg  string
	}{
		{"裸 32 位十六进制按 MD5", md5Hex, true, digestMD5},
		{"md5: 前缀", "md5:" + md5Hex, true, digestMD5},
		{"裸 64 位十六进制按 SHA-256", sha256Hex, true, digestSHA256},
		{"sha256: 前缀", "sha256:" + sha256Hex, true, digestSHA256},
		{"大写同样接受（规范化小写比对）", strings.ToUpper(sha256Hex), true, digestSHA256},
		{"空串拒绝（摘要必填）", "", false, ""},
		{"长度不对拒绝", "0123456789abcdef", false, ""},
		{"非十六进制拒绝", strings.Repeat("z", 32), false, ""},
		{"前缀但内容非法拒绝", "sha256:nothex", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			alg, want, ok := parseDigest(c.in)
			if ok != c.ok {
				t.Fatalf("parseDigest(%q) ok=%v, 期望 %v", c.in, ok, c.ok)
			}
			if !ok {
				if alg != "" || want != "" {
					t.Fatalf("非法输入不应返回算法/摘要: %q/%q", alg, want)
				}
				return
			}
			if alg != c.alg {
				t.Fatalf("算法=%q, 期望 %q", alg, c.alg)
			}
			if want != strings.ToLower(want) {
				t.Fatalf("摘要应规范化为小写: %q", want)
			}
		})
	}
}

func TestChunkDigestMatchFailsClosed(t *testing.T) {
	data := []byte("chunk-payload")
	if !chunkDigestMatch(data, md5Hex(data)) {
		t.Fatal("正确 md5 应匹配")
	}
	// 空摘要必须不匹配（旧实现留空即放行，等于分片完整性可整体绕过）
	if chunkDigestMatch(data, "") {
		t.Fatal("空摘要必须判为不匹配")
	}
	if chunkDigestMatch(data, "not-a-digest") {
		t.Fatal("非法摘要必须判为不匹配")
	}
	if chunkDigestMatch(data, md5Hex([]byte("other"))) {
		t.Fatal("内容不符必须判为不匹配")
	}
}

func TestValidateKeyRejectsTraversal(t *testing.T) {
	good := []string{"a.png", "sub/a.png", "abc_20260101120000.webp"}
	for _, k := range good {
		if err := ValidateKey(k); err != nil {
			t.Fatalf("%q 应通过: %v", k, err)
		}
	}
	bad := []string{"", "../x.png", "sub/../../x.png", `sub\x.png`, `C:x.png`, ".."}
	for _, k := range bad {
		if err := ValidateKey(k); err == nil {
			t.Fatalf("%q 应被拒绝", k)
		}
	}
}

func TestBuildKeyShape(t *testing.T) {
	key := BuildKey("photo.PNG")
	if strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		t.Fatalf("对象键形态非法: %q", key)
	}
	if !strings.HasSuffix(key, ".PNG") {
		t.Fatalf("应保留扩展名（含大小写）: %q", key)
	}
	// md5(基名) 前缀（32 位）+ 下划线 + 14 位时间戳
	parts := strings.Split(strings.TrimSuffix(key, ".PNG"), "_")
	if len(parts) != 2 || len(parts[0]) != 32 || len(parts[1]) != 14 {
		t.Fatalf("对象键应为 md5_时间戳 形态: %q", key)
	}
}

func TestBuildCategoryTreeFromFlat(t *testing.T) {
	rows := []*model.ExaAttachmentCategory{
		{ID: 1, Name: "根", Pid: 0},
		{ID: 2, Name: "子A", Pid: 1},
		{ID: 3, Name: "子B", Pid: 1},
		{ID: 4, Name: "孙", Pid: 2},
	}
	tree := buildCategoryTree(rows, 0)
	if len(tree) != 1 || tree[0].Name != "根" {
		t.Fatalf("根层应只有 1 个节点: %+v", tree)
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("根应有 2 个子节点，实际 %d", len(tree[0].Children))
	}
	if len(tree[0].Children[0].Children) != 1 || tree[0].Children[0].Children[0].Name != "孙" {
		t.Fatalf("孙节点挂错了层: %+v", tree[0].Children[0].Children)
	}
}
