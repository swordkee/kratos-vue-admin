package sensitive

import (
	"context"
	"testing"
)

// 判定：未注入/显式 false → 不可见（fail-closed）；true → 可见。
func TestCanReveal(t *testing.T) {
	if CanReveal(context.Background()) {
		t.Fatal("未注入 ctx 默认不可见（fail-closed）")
	}
	if CanReveal(WithReveal(context.Background(), false)) {
		t.Fatal("显式 false 不可见")
	}
	if !CanReveal(WithReveal(context.Background(), true)) {
		t.Fatal("显式 true 应可见")
	}
	// 伪造的公共 string key 不得改变判定（ctxKey 为私有类型）
	//nolint:staticcheck // 负例：故意用 string key 模拟外部伪造
	forged := context.WithValue(context.Background(), "reveal", true)
	if CanReveal(forged) {
		t.Fatal("string key 不得影响判定")
	}
}

// 掩码算法：两端保留、长度不变、过短全掩码。
func TestMaskIDCard(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"110101199001011234", "110***********1234"},
		{"", ""},
		{"1234567", "*******"}, // 长度<=7 全掩码
		{"12345678", "123*5678"},
	}
	for _, c := range cases {
		got := MaskIDCard(c.in)
		if got != c.want {
			t.Fatalf("MaskIDCard(%q)=%q want %q", c.in, got, c.want)
		}
		if c.in != "" && len([]rune(got)) != len([]rune(c.in)) {
			t.Fatalf("掩码不得改变长度: in=%q got=%q", c.in, got)
		}
	}
}

func TestMaskMobile(t *testing.T) {
	if got := MaskMobile("13800001234"); got != "138****1234" {
		t.Fatalf("MaskMobile=%q", got)
	}
	if got := MaskMobile("138"); got != "***" {
		t.Fatalf("短号应全掩码: %q", got)
	}
}

func TestMaskBankCard(t *testing.T) {
	if got := MaskBankCard("6225880100001234"); got != "6225********1234" {
		t.Fatalf("MaskBankCard=%q", got)
	}
}

// MaskPhoneLike：只对手机号形态掩码，其余原样（避免打碎微信 openid/昵称）。
func TestMaskPhoneLike(t *testing.T) {
	cases := []struct{ in, want string }{
		{"13800001234", "138****1234"},
		{"oX9f3kLmNoPqRsTu", "oX9f3kLmNoPqRsTu"},
		{"1380000123a", "1380000123a"},
		{"23800001234", "23800001234"},
		{"", ""},
	}
	for _, c := range cases {
		if got := MaskPhoneLike(c.in); got != c.want {
			t.Fatalf("MaskPhoneLike(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// HasMask：更新入口回环保护判定。
func TestHasMask(t *testing.T) {
	if !HasMask("138****1234") {
		t.Fatal("含 * 应判为掩码值")
	}
	if HasMask("13800001234") || HasMask("") {
		t.Fatal("明文/空串不应判为掩码值")
	}
}

// 掩码回环保护：掩码值不覆盖原值；明文值正常写入（红→绿基线：原实现直接覆盖）。
func TestResolveMasked(t *testing.T) {
	cases := []struct{ submitted, original, want string }{
		{"138****1234", "13800001234", "13800001234"}, // 回填掩码 → 保留原值
		{"13900005678", "13800001234", "13900005678"}, // 真改号 → 写入新值
		{"", "13800001234", ""},                       // 清空是真实意图，照写
		{"6225********1234", "6225880100001234", "6225880100001234"},
	}
	for _, c := range cases {
		if got := ResolveMasked(c.submitted, c.original); got != c.want {
			t.Fatalf("ResolveMasked(%q,%q)=%q want %q", c.submitted, c.original, got, c.want)
		}
	}
	// 反例锁：若把回环保护退化成「恒返回提交值」，本断言必须红。
	if got := ResolveMasked("138****1234", "13800001234"); got == "138****1234" {
		t.Fatal("掩码值不得回写覆盖明文")
	}
}
