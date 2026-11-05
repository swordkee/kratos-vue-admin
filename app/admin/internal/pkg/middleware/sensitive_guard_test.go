package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v3/middleware"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/sensitive"
)

// runGuard 用给定 claims + checker 跑一次中间件，返回下游观察到的 reveal 判定。
func runGuard(t *testing.T, claims *authz.TokenClaims, checker SensitiveRevealChecker) bool {
	t.Helper()
	seen := false
	h := SensitiveInfoGuard(checker)(func(ctx context.Context, req any) (any, error) {
		seen = sensitive.CanReveal(ctx)
		return nil, nil
	})
	var ctx context.Context = context.Background()
	if claims != nil {
		ctx = authz.NewContext(ctx, claims)
	}
	// 类型断言走 middleware.Handler（与 Kratos 装配一致）
	var downstream middleware.Handler = h
	if _, err := downstream(ctx, nil); err != nil {
		t.Fatalf("guard 不应返回错误: %v", err)
	}
	return seen
}

// 无 claims：未授权（fail-closed）。
func TestSensitiveGuard_NoClaims(t *testing.T) {
	called := false
	got := runGuard(t, nil, func(context.Context, string) (bool, error) {
		called = true
		return true, nil
	})
	if got {
		t.Fatal("未登录请求不得可见明文")
	}
	if called {
		t.Fatal("无 claims 不应调用 checker")
	}
}

// 超管旁路：checker 不被调用即放行。
func TestSensitiveGuard_SuperAdminBypass(t *testing.T) {
	called := false
	got := runGuard(t, &authz.TokenClaims{RoleKey: authz.SuperAdminRoleKey}, func(context.Context, string) (bool, error) {
		called = true
		return false, nil
	})
	if !got {
		t.Fatal("超管应可见明文")
	}
	if called {
		t.Fatal("超管不应经过 casbin 判定")
	}
}

// 非超管：checker 结果决定可见性。
func TestSensitiveGuard_CheckedRole(t *testing.T) {
	if got := runGuard(t, &authz.TokenClaims{RoleKey: "ops"}, func(_ context.Context, role string) (bool, error) {
		if role != "ops" {
			t.Fatalf("roleKey=%q", role)
		}
		return true, nil
	}); !got {
		t.Fatal("已授予 /sensitive/view 的角色应可见明文")
	}
	if got := runGuard(t, &authz.TokenClaims{RoleKey: "ops"}, func(context.Context, string) (bool, error) {
		return false, nil
	}); got {
		t.Fatal("未授予 /sensitive/view 的角色必须掩码")
	}
}

// 判定故障 fail-closed：不阻断请求，但按未授权处理。
func TestSensitiveGuard_CheckerErrorFailClosed(t *testing.T) {
	got := runGuard(t, &authz.TokenClaims{RoleKey: "ops"}, func(context.Context, string) (bool, error) {
		return true, errors.New("casbin unavailable")
	})
	if got {
		t.Fatal("checker 出错必须 fail-closed 掩码")
	}
}
