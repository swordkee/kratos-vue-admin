package admin

// 登录失败锁定接线锁（biz/admin/auth.go ×× security_lock/security_config）：
//
//	1. 达阈值锁定后，**即使密码正确也拒绝登录**（gva 口径：锁定期间不校验密码）；
//	2. 用户名不存在与密码错误同样计入失败计数；
//	3. 完整登录成功（needMfa=false 且 needSms=false）清计数与锁态；
//	4. TOTP / 短信待验证阶段（needMfa / needSms=true）**不清**计数——
//	   cleanup 条件必须两个标志同时为 false；
//	5. 人工解锁后恢复登录；
//	6. lockSvc 为 nil（未装配）时登录行为与改动前完全一致（不做任何计数/拒绝）。
//
// 变异自检口径：把 auth.go 的 accountLocked 短路成恒 false，第 1 条必须转红。

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

// newAuthWithLock 构造带登录锁定用例的 AuthUseCase（mfaSvc 固定 nil：TOTP 能力总闸关，
// 走单因素降级分支，便于只关注锁定接线）。
func newAuthWithLock(t *testing.T, loginMethod int32, withSms bool, lock *SecurityLockUseCase) (*AuthUseCase, *model.SysUsers, *fakeSmsLoginRepo) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("生成 ECDSA 私钥失败: %v", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	authConf := &conf.Auth{
		EcdsaPrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})),
		Expires:         durationpb.New(time.Hour),
	}
	user := &model.SysUsers{
		ID:          77,
		Username:    "lock_user",
		Phone:       "13800000000",
		Password:    util.BcryptHash(loginMethodTestPassword),
		RoleID:      1,
		Status:      1,
		LoginMethod: loginMethod,
	}
	var smsRepo SmsLoginCodeRepo
	fakeSms := &fakeSmsLoginRepo{}
	if withSms {
		smsRepo = fakeSms
	}
	auth, err := NewAuthUseCase(authConf, &loginStubUserRepo{user: user}, loginStubRoleRepo{}, nil, smsRepo, lock, newMessageTestLogger())
	if err != nil {
		t.Fatalf("构造 AuthUseCase 失败: %v", err)
	}
	return auth, user, fakeSms
}

func newEnabledLock(t *testing.T, threshold int32) *SecurityLockUseCase {
	t.Helper()
	lock, _ := newLockFixture(t, model.SysSecurityConfig{
		ID: SecurityConfigRowID, LockEnable: true, LockThreshold: threshold, LockDuration: 30,
	})
	return lock
}

// 1 + 4：达阈值锁定后密码正确也拒绝；解锁后恢复正常
func TestLoginFailLockBlocksLogin(t *testing.T) {
	lock := newEnabledLock(t, 2)
	auth, user, _ := newAuthWithLock(t, LoginMethodTOTP, false, lock)
	ctx := context.Background()

	bad := func() {
		if _, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: "wrong-password"}); err == nil {
			t.Fatal("错误密码应登录失败")
		}
	}
	good := func() error {
		_, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: loginMethodTestPassword})
		return err
	}

	bad()
	if st := lock.Status(ctx, user.Username); st.FailCount != 1 || st.Locked {
		t.Fatalf("阈值 2 时一次失败应只计数不锁定: %+v", st)
	}
	bad()
	if st := lock.Status(ctx, user.Username); !st.Locked {
		t.Fatalf("两次失败应锁定: %+v", st)
	}

	// 关键断言：锁定期间即使密码正确也拒绝，且提示锁定时长
	err := good()
	if err == nil {
		t.Fatal("锁定期间应拒绝登录")
	}
	if !strings.Contains(err.Error(), "锁定") {
		t.Fatalf("锁定提示应包含「锁定」, got %v", err)
	}

	// 人工解锁后恢复
	if uerr := lock.Unlock(ctx, user.Username); uerr != nil {
		t.Fatalf("人工解锁失败: %v", uerr)
	}
	if err := good(); err != nil {
		t.Fatalf("解锁后应可登录: %v", err)
	}
}

// 2：用户名不存在同样计入失败计数
func TestLoginUnknownUsernameCountsFail(t *testing.T) {
	lock := newEnabledLock(t, 5)
	auth, _, _ := newAuthWithLock(t, LoginMethodTOTP, false, lock)
	ctx := context.Background()

	if _, err := auth.Login(ctx, &pb.LoginRequest{Username: "ghost_user", Password: "whatever"}); err == nil {
		t.Fatal("不存在的用户名应登录失败")
	}
	if st := lock.Status(ctx, "ghost_user"); st.FailCount != 1 {
		t.Fatalf("不存在的用户名也应计数, got %+v", st)
	}
}

// 3：完整登录成功清计数与锁态
func TestLoginSuccessClearsFailCount(t *testing.T) {
	lock := newEnabledLock(t, 5)
	auth, user, _ := newAuthWithLock(t, LoginMethodTOTP, false, lock)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: "wrong-password"})
	}
	if st := lock.Status(ctx, user.Username); st.FailCount != 2 {
		t.Fatalf("两次失败应计数 2, got %+v", st)
	}
	if _, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: loginMethodTestPassword}); err != nil {
		t.Fatalf("正确密码应登录成功: %v", err)
	}
	if st := lock.Status(ctx, user.Username); st.FailCount != 0 || st.Locked {
		t.Fatalf("完整登录成功后应清零, got %+v", st)
	}
}

// 4：短信待验证阶段（needSms=true）不清计数
func TestLoginSmsPendingKeepsFailCount(t *testing.T) {
	lock := newEnabledLock(t, 5)
	auth, user, _ := newAuthWithLock(t, LoginMethodSMS, true, lock)
	ctx := context.Background()

	if _, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: "wrong-password"}); err == nil {
		t.Fatal("错误密码应失败")
	}
	reply, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: loginMethodTestPassword})
	if err != nil {
		t.Fatalf("正确密码应进入短信待验证: %v", err)
	}
	if !reply.GetNeedSms() {
		t.Fatalf("login_method=2 应返回 needSms=true, got %+v", reply)
	}
	if st := lock.Status(ctx, user.Username); st.FailCount != 1 {
		t.Fatalf("待验证阶段不得清失败计数, got %+v", st)
	}
}

// 5：lockSvc 未装配（nil）时行为不变
func TestLoginNilLockNoop(t *testing.T) {
	auth, user, _ := newAuthWithLock(t, LoginMethodTOTP, false, nil)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if _, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: "wrong-password"}); err == nil {
			t.Fatal("错误密码应失败")
		}
	}
	if _, err := auth.Login(ctx, &pb.LoginRequest{Username: user.Username, Password: loginMethodTestPassword}); err != nil {
		t.Fatalf("未装配锁定用例时登录应不受影响: %v", err)
	}
}
