package server_test

// 请求解码类型保持锁（回灌 mall 3f23139「保留后台请求参数类型并修正上传」的 Go 侧）。
//
// 口径：前端不再把 protojson 的 int64 字符串强行转 number（字符串 ID 如订单号会丢精度），
// 改由服务端 CustomRequestDecoder 同时接受「数字」与「数字字符串」两种形态，
// 而真正的字符串字段（captchaId/用户名等）必须原样保留。

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	adminpb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/server"
)

func TestRequestDecoderPreservesStringIDsAndAcceptsNumericIDs(t *testing.T) {
	decode := func(body string, dst any) {
		t.Helper()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		require.NoError(t, server.CustomRequestDecoder(req, dst))
	}

	// 字符串字段原样保留（不得被前端转成数字）
	var login adminpb.LoginRequest
	decode(`{"username":"test","captchaId":"123456"}`, &login)
	require.Equal(t, "123456", login.GetCaptchaId())
	require.Equal(t, "test", login.GetUsername())

	// int64 同时接受字符串与数字；超出 JS 安全整数的字符串不得丢精度
	var roles adminpb.CreateRolesRequest
	decode(`{"menuIds":["1","9007199254740993"],"parentId":"2"}`, &roles)
	require.Equal(t, []int64{1, 9007199254740993}, roles.GetMenuIds())
	require.EqualValues(t, 2, roles.GetParentId())

	var rolesNum adminpb.CreateRolesRequest
	decode(`{"menuIds":[1,2],"parentId":3}`, &rolesNum)
	require.Equal(t, []int64{1, 2}, rolesNum.GetMenuIds())
	require.EqualValues(t, 3, rolesNum.GetParentId())
}