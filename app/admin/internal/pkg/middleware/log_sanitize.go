package middleware

import (
	"encoding/json"
	"regexp"
	"strings"
)

// 操作日志载荷「入库前脱敏」（2026-10-05）。
//
// 动机：OperationRecord 中间件原样记录请求体/响应体（如
// `{"username":"admin","password":"123456"}`、登录响应里的 token），
// 明文随 sys_logs 落库 ⇒ 任何能读日志的人都可见凭证/个人敏感信息。
//
// 口径（按字段名，大小写与 _ / - 归一后匹配）：
//   - 凭证类（password/pwd/secret/token/authorization/credential/privateKey/
//     accessKey/appKey/apiKey/signature/certToken…）→ 值整体替换为 `****`；
//   - 身份证类（idCard/certNo/identity/idNumber…）→ 保留前 3 后 4；
//   - 手机号类（mobile/phone/tel/telephone…）→ 保留前 3 后 4；
//   - 银行卡类（bankCard/cardNo/bankAccount/acctNo…）→ 保留前 4 后 4。
//
// 实现：JSON 按字段递归掩码；非 JSON（表单/畸形串）走正则兜底；
// 无法解析时原样返回（不阻断记录）。掩码在 truncateBody 之前执行，
// 避免「先截断把敏感值切一半再掩码不到」。

// credentialKeys 凭证类字段名片段（归一后 contains 匹配）。
var credentialKeys = []string{
	"password", "passwd", "pwd", "secret", "token", "authorization",
	"credential", "privatekey", "accesskey", "appkey", "apikey",
	"signature", "certtoken",
}

// normalizeKey 字段名归一：小写并去掉 _ / -（AppSecret → appsecret，app_secret → appsecret）。
func normalizeKey(k string) string {
	return strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(k))
}

// isCredentialKey 是否凭证类字段（整体掩码）。
func isCredentialKey(k string) bool {
	n := normalizeKey(k)
	for _, s := range credentialKeys {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

// piiMask 个人敏感字段的掩码类型；"" 表示非敏感字段。
func piiMask(k string) string {
	n := normalizeKey(k)
	switch {
	case strings.Contains(n, "idcard"), strings.Contains(n, "certno"),
		strings.Contains(n, "identity"), strings.Contains(n, "idnumber"):
		return "idcard"
	case strings.Contains(n, "mobile"), strings.Contains(n, "phone"),
		strings.Contains(n, "telephone"), n == "tel":
		return "mobile"
	case strings.Contains(n, "bankcard"), strings.Contains(n, "cardno"),
		strings.Contains(n, "bankaccount"), strings.Contains(n, "acctno"):
		return "bankcard"
	}
	return ""
}

// maskKeepEnds 通用两端保留掩码；空串原样；过短（head+tail 放不下）整串替换为等长 '*'。
func maskKeepEnds(v string, head, tail int) string {
	if v == "" {
		return v
	}
	n := len([]rune(v))
	if n <= head+tail {
		return strings.Repeat("*", n)
	}
	r := []rune(v)
	return string(r[:head]) + strings.Repeat("*", n-head-tail) + string(r[n-tail:])
}

// maskStringValue 按字段名掩码单个字符串值；非敏感字段原样。
func maskStringValue(key, val string) string {
	if isCredentialKey(key) {
		if val == "" {
			return ""
		}
		return "****"
	}
	switch piiMask(key) {
	case "idcard":
		return maskKeepEnds(val, 3, 4)
	case "mobile":
		return maskKeepEnds(val, 3, 4)
	case "bankcard":
		return maskKeepEnds(val, 4, 4)
	}
	return val
}

// maskJSONNode 递归掩码 JSON 节点（对象按字段名、数组逐项）。
func maskJSONNode(node any) any {
	switch t := node.(type) {
	case map[string]any:
		for k, v := range t {
			if s, ok := v.(string); ok {
				t[k] = maskStringValue(k, s)
			} else {
				t[k] = maskJSONNode(v)
			}
		}
		return t
	case []any:
		for i := range t {
			if _, ok := t[i].(string); ok {
				// 数组元素无字段名，不做值级猜测（避免误伤单号等数字串）
				continue
			}
			t[i] = maskJSONNode(t[i])
		}
		return t
	}
	return node
}

// rawKVPattern 非 JSON 载荷的兜底：匹配 `"key":"value"` / `key=value` 形态的凭证字段。
var rawKVPattern = regexp.MustCompile(
	`(?i)"?([a-z0-9_]*(?:password|passwd|pwd|secret|token|authorization|credential|privatekey|accesskey|appkey|apikey|signature|certtoken)[a-z0-9_]*)"?(\s*[:=]\s*)"?([^",}&\s]*)`)

// sanitizeLogPayload 操作日志载荷脱敏（入库前调用）。JSON 走结构化掩码，
// 非 JSON 走正则兜底；无法识别则原样返回（由调用方截断）。
func sanitizeLogPayload(payload string) string {
	if payload == "" {
		return payload
	}
	trimmed := strings.TrimSpace(payload)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var node any
		if err := json.Unmarshal([]byte(trimmed), &node); err == nil {
			if b, mErr := json.Marshal(maskJSONNode(node)); mErr == nil {
				return string(b)
			}
		}
	}
	// 非 JSON / 解析失败：正则兜底（把凭证值替换为 ****）
	return rawKVPattern.ReplaceAllString(payload, `${1}${2}"****"`)
}
