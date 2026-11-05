package admin

// 用户级登录方式（sys_users.login_method）登录分流锁：
//
//	login_method=1（TOTP，默认）：
//	  1. 已绑定 → MfaPending 中间态（NeedMfa=true），第二步 /mfa/verify；
//	  2. 未绑定 → 无条件 MustEnrollMfa（**不受 mfa.required 影响**，不允许绕过 TOTP）；
//	  3. auth.mfa.enabled=false（TOTP 能力总闸关）→ 不可执行，降级单因素（留 Error 日志）；
//	login_method=2（手机验证码短信）：
//	  4. 有手机号 → 发码 + SmsPending 中间态（NeedSms=true），第二步 /system/user/phone/verify；
//	  5. 无手机号 → 拒绝且文案明确（不发码）；
//	  6. 短信发送失败 → 拒绝（不签发无码可用的中间态 token）；
//	  7. 短信端口未装配（nil）→ 拒绝（不降级为单因素）；
//	PhoneLogin 收口：
//	  8. login_method=1 用户短信登录一律拒绝（强制 TOTP，不允许旁路）；
//	  9. login_method=2 用户短信登录直接签发正式 token（短信码本身即手机因素）；
//	第二步校验：
//	 10. 短信中间态 + 正确码 → 正式 token（中间态 flag 全清）；错码 → 拒绝；
//	 11. TOTP 中间态不得走短信校验接口（防串道），短信中间态不得走 /mfa/verify；
//	 12. 两步之间登录方式被改成 TOTP → 短信会话作废。

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
	"gorm.io/gorm"
)

const loginMethodTestPassword = "Admin@123456"

// fakeMFAStore pkg/mfa.MFAUserStore 的内存实现（登录分流只读 MFA 三列即够）
type fakeMFAStore struct{ row mfa.UserMFA }

func (s *fakeMFAStore) Under(*gorm.DB) mfa.MFAUserStore { return s }
func (s *fakeMFAStore) GetMFA(context.Context, int64) (mfa.UserMFA, bool, error) {
	return s.row, true, nil
}
func (s *fakeMFAStore) SetMFA(context.Context, int64, mfa.UserMFA) (int64, error) {
	return 1, nil
}

// fakeSmsLoginRepo admin.SmsLoginCodeRepo 的内存实现（记录发码/校验行为）
type fakeSmsLoginRepo struct {
	sendErr  error
	sentTo   []string
	sentUID  []int64
	code     string
	verified []string
}

func (f *fakeSmsLoginRepo) SendLoginCode(_ context.Context, userID int64, phone string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sentTo = append(f.sentTo, phone)
	f.sentUID = append(f.sentUID, userID)
	return nil
}

func (f *fakeSmsLoginRepo) VerifyLoginCode(_ context.Context, _ int64, code string) (bool, error) {
	f.verified = append(f.verified, code)
	if f.code == "" {
		return false, nil
	}
	return f.code == code, nil
}

// loginStubUserRepo 登录分流专用的 SysUserRepo 桩（模板仓此前无此桩，与 auth_lock_test 的
// stubUserRepo 独立，避免跨文件耦合）
type loginStubUserRepo struct{ user *model.SysUsers }

func (r *loginStubUserRepo) Save(context.Context, *model.SysUsers) (*model.SysUsers, error) {
	return r.user, nil
}
func (r *loginStubUserRepo) Delete(context.Context, int64) error { return nil }
func (r *loginStubUserRepo) UpdateByID(context.Context, int64, *model.SysUsers) error {
	return nil
}
func (r *loginStubUserRepo) Create(_ context.Context, g *model.SysUsers) (*model.SysUsers, error) {
	r.user = g
	return g, nil
}
func (r *loginStubUserRepo) FindByID(context.Context, int64) (*model.SysUsers, error) {
	return r.user, nil
}
func (r *loginStubUserRepo) FindByUsername(_ context.Context, name string) (*model.SysUsers, error) {
	if r.user == nil || r.user.Username != name {
		return nil, gorm.ErrRecordNotFound
	}
	return r.user, nil
}
func (r *loginStubUserRepo) FindByPhone(_ context.Context, phone string) (*model.SysUsers, error) {
	if r.user == nil || r.user.Phone != phone {
		return nil, gorm.ErrRecordNotFound
	}
	return r.user, nil
}
func (r *loginStubUserRepo) FindByPostId(context.Context, int64) ([]*model.SysUsers, error) {
	return nil, nil
}
func (r *loginStubUserRepo) ListPage(context.Context, int32, int32, UserListCondition) ([]*model.SysUsers, error) {
	return nil, nil
}
func (r *loginStubUserRepo) Count(context.Context, UserListCondition) (int32, error) { return 0, nil }
func (r *loginStubUserRepo) CountByRoleId(context.Context, int64) (int64, error)     { return 0, nil }
func (r *loginStubUserRepo) FindAll(context.Context) ([]*model.SysUsers, error)      { return nil, nil }
func (r *loginStubUserRepo) AddJwtToBlacklist(context.Context, string) error         { return nil }
func (r *loginStubUserRepo) IsJwtInBlacklist(context.Context, string) (bool, error) {
	return false, nil
}
func (r *loginStubUserRepo) CleanExpiredBlacklists(context.Context) error           { return nil }
func (r *loginStubUserRepo) AddIpToBlacklist(context.Context, string, string) error { return nil }
func (r *loginStubUserRepo) IsIpInBlacklist(context.Context, string) (bool, error)  { return false, nil }
func (r *loginStubUserRepo) RemoveIpFromBlacklist(context.Context, string) error    { return nil }

// loginStubRoleRepo SysRoleRepo 桩（只用到 FindByID）
type loginStubRoleRepo struct{}

func (loginStubRoleRepo) Create(context.Context, *model.SysRoles) error { return nil }
func (loginStubRoleRepo) Save(context.Context, *model.SysRoles) error   { return nil }
func (loginStubRoleRepo) Delete(context.Context, ...int64) error        { return nil }
func (loginStubRoleRepo) FindByID(_ context.Context, id int64) (*model.SysRoles, error) {
	return &model.SysRoles{ID: id, RoleKey: "admin", RoleName: "超级管理员"}, nil
}
func (loginStubRoleRepo) FindByIDList(context.Context, ...int64) ([]*model.SysRoles, error) {
	return nil, nil
}
func (loginStubRoleRepo) FindAll(context.Context) ([]*model.SysRoles, error) { return nil, nil }
func (loginStubRoleRepo) ListPage(context.Context, string, string, int32, int32, int32) ([]*model.SysRoles, error) {
	return nil, nil
}
func (loginStubRoleRepo) Count(context.Context, string, string, int32) (int32, error) { return 0, nil }
func (loginStubRoleRepo) Update(context.Context, *model.SysRoles) error               { return nil }

type loginMethodFixture struct {
	auth *AuthUseCase
	priv *ecdsa.PrivateKey
	user *model.SysUsers
	sms  *fakeSmsLoginRepo
}

// newLoginMethodFixture 构造登录分流夹具；mfaCfg.Enabled=false 等价「TOTP 能力总闸关闭」。
// bound 表示该用户是否已绑定 TOTP（mfa_enabled/mfa_secret/mfa_bound_at 三列）。
// smsRepo 传 nil 表示「短信端口未装配」。
func newLoginMethodFixture(t *testing.T, loginMethod int32, phone string, mfaCfg mfa.Config, bound bool, withSms bool) *loginMethodFixture {
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
		ID:          42,
		Username:    "lm_user",
		Phone:       phone,
		Password:    util.BcryptHash(loginMethodTestPassword),
		RoleID:      1,
		Status:      1,
		LoginMethod: loginMethod,
	}
	store := &fakeMFAStore{}
	if bound {
		store.row = mfa.UserMFA{Enabled: true, Secret: "enc", BoundAt: new(time.Time)}
	}
	var mfaSvc *mfa.Service
	if mfaCfg.EncryptionKey != "" || mfaCfg.Enabled {
		mfaSvc = mfa.NewService(store, nil, nil, mfaCfg, newMessageTestLogger())
	}
	var smsRepo SmsLoginCodeRepo
	fakeSms := &fakeSmsLoginRepo{}
	if withSms {
		smsRepo = fakeSms
	}
	auth, err := NewAuthUseCase(authConf, &loginStubUserRepo{user: user}, loginStubRoleRepo{}, mfaSvc, smsRepo, nil, newMessageTestLogger())
	if err != nil {
		t.Fatalf("构造 AuthUseCase 失败: %v", err)
	}
	return &loginMethodFixture{auth: auth, priv: priv, user: user, sms: fakeSms}
}

func (f *loginMethodFixture) login(t *testing.T) (*pb.LoginReply, error) {
	t.Helper()
	return f.auth.Login(context.Background(), &pb.LoginRequest{
		Username: f.user.Username,
		Password: loginMethodTestPassword,
	})
}

func (f *loginMethodFixture) claims(t *testing.T, token string) *authz.TokenClaims {
	t.Helper()
	claims, err := authz.ParseToken(token, &f.priv.PublicKey)
	if err != nil {
		t.Fatalf("解析中间态 token 失败: %v", err)
	}
	return claims
}

// ecdsaKeyPEM 把夹具私钥导出为 PEM（构造需要 conf.Auth 的 MfaUseCase 用）
func (f *loginMethodFixture) ecdsaKeyPEM(t *testing.T) string {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(f.priv)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

func mfaEnabledCfg() mfa.Config {
	return mfa.Config{Enabled: true, Required: false, EncryptionKey: "0123456789abcdef0123456789abcdef"}
}

// 锁 1：login_method=1 已绑定 → 必须走 TOTP 待验证（NeedMfa），不得直接发正式 token。
func TestLoginTOTPBoundRequiresPending(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodTOTP, "13800000000", mfaEnabledCfg(), true, true)
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("登录应成功返回中间态: %v", err)
	}
	if !reply.GetNeedMfa() || reply.GetMfaToken() == "" {
		t.Fatalf("TOTP 已绑定用户必须返回 NeedMfa + mfaToken, got %+v", reply)
	}
	if reply.GetToken() != "" {
		t.Fatal("二因素待验证阶段不得签发正式 token")
	}
	if reply.GetNeedSms() {
		t.Fatal("TOTP 用户不应带 needSms")
	}
	c := f.claims(t, reply.GetMfaToken())
	if !c.MfaPending || c.SmsPending {
		t.Fatalf("TOTP 中间态应为 MfaPending 且非 SmsPending, got %+v", c)
	}
}

// 锁 2（核心）：login_method=1 未绑定 → 无条件 MustEnroll，**mfa.required=false 也不放行**。
func TestLoginTOTPUnboundMustEnrollEvenIfNotRequired(t *testing.T) {
	cfg := mfaEnabledCfg()
	cfg.Required = false // 关键：不再依赖 required 才引导绑定
	f := newLoginMethodFixture(t, LoginMethodTOTP, "13800000000", cfg, false, true)
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("未绑定应返回绑定引导 token: %v", err)
	}
	if reply.GetToken() == "" || reply.GetNeedMfa() {
		t.Fatalf("未绑定的 TOTP 用户应返回 MustEnroll 引导 token, got %+v", reply)
	}
	c := f.claims(t, reply.GetToken())
	if !c.MustEnrollMfa {
		t.Fatalf("token 必须带 MustEnrollMfa（守卫仅放行绑定接口）, got %+v", c)
	}
}

// 锁 2b（豁免不外溢）：login_method=2 用户即便已绑定 TOTP 也不得走 TOTP 待验证。
func TestLoginSMSUserIgnoresBoundTOTP(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13800000000", mfaEnabledCfg(), true, true)
	f.sms.code = "123456"
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("短信登录方式应返回短信中间态: %v", err)
	}
	if reply.GetNeedMfa() || !reply.GetNeedSms() {
		t.Fatalf("login_method=2 用户必须走短信中间态, got %+v", reply)
	}
}

// 锁 3：login_method=1 但 auth.mfa.enabled=false → 降级单因素（不造成全员锁死），且不返回中间态。
func TestLoginTOTPDegradesWhenFeatureDisabled(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodTOTP, "13800000000", mfa.Config{Enabled: false}, false, true)
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("总闸关闭时应降级为单因素登录: %v", err)
	}
	if reply.GetToken() == "" || reply.GetNeedMfa() || reply.GetNeedSms() {
		t.Fatalf("总闸关闭应签发正式 token, got %+v", reply)
	}
}

// 锁 4：login_method=2 有手机号 → 发码 + 短信中间态。
func TestLoginSMSIssuesPendingAndSendsCode(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13900001111", mfaEnabledCfg(), false, true)
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("短信登录方式应返回短信中间态: %v", err)
	}
	if !reply.GetNeedSms() || reply.GetMfaToken() == "" || reply.GetToken() != "" {
		t.Fatalf("应返回 NeedSms + mfaToken 且无正式 token, got %+v", reply)
	}
	if len(f.sms.sentTo) != 1 || f.sms.sentTo[0] != "13900001111" {
		t.Fatalf("应向该用户手机号发码一次, got %v", f.sms.sentTo)
	}
	if len(f.sms.sentUID) != 1 || f.sms.sentUID[0] != f.user.ID {
		t.Fatalf("发码必须按 userID 维度, got %v", f.sms.sentUID)
	}
	c := f.claims(t, reply.GetMfaToken())
	if !c.MfaPending || !c.SmsPending {
		t.Fatalf("短信中间态应为 MfaPending+SmsPending, got %+v", c)
	}
}

// 锁 5：login_method=2 但手机号为空 → 拒绝且文案明确（不发码）。
func TestLoginSMSWithoutPhoneRejected(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "", mfaEnabledCfg(), false, true)
	_, err := f.login(t)
	if err == nil {
		t.Fatal("未绑定手机号的短信登录方式必须拒绝")
	}
	if !strings.Contains(err.Error(), "未绑定手机号") {
		t.Fatalf("错误文案应明确「未绑定手机号」, got %v", err)
	}
	if len(f.sms.sentTo) != 0 {
		t.Fatalf("拒绝时不应发码, got %v", f.sms.sentTo)
	}
}

// 锁 6：短信发送失败 → 拒绝（不签发中间态 token，否则用户拿不到码却可继续流程）。
func TestLoginSMSSendFailureRejected(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13900001111", mfaEnabledCfg(), false, true)
	f.sms.sendErr = errors.New("gateway down")
	_, err := f.login(t)
	if err == nil {
		t.Fatal("短信发送失败时必须拒绝登录")
	}
	if !strings.Contains(err.Error(), "短信验证码发送失败") {
		t.Fatalf("错误文案应提示发送失败, got %v", err)
	}
}

// 锁 7：短信端口未装配（nil）→ 拒绝（模板仓默认无短信网关，fail-closed 不降级）。
func TestLoginSMSWithoutGatewayRejected(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13900001111", mfaEnabledCfg(), false, false)
	_, err := f.login(t)
	if err == nil {
		t.Fatal("短信端口未装配时必须拒绝 login_method=2 登录")
	}
	if !strings.Contains(err.Error(), "短信验证码登录未启用") {
		t.Fatalf("错误文案应提示短信登录未启用, got %v", err)
	}
}

// 锁 8：PhoneLogin 对 login_method=1（TOTP）用户一律拒绝（不允许旁路 TOTP）。
func TestPhoneLoginRejectsTOTPUser(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodTOTP, "13800000000", mfaEnabledCfg(), true, true)
	_, err := f.auth.PhoneLogin(context.Background(), "13800000000")
	if err == nil {
		t.Fatal("login_method=1 用户不得用短信验证码登录")
	}
	if !strings.Contains(err.Error(), "不支持短信验证码登录") {
		t.Fatalf("错误文案应说明不支持短信登录, got %v", err)
	}
}

// 锁 9（正向）：PhoneLogin 对 login_method=2 用户签发正式 token（短信码即手机因素）。
func TestPhoneLoginAllowsSMSUser(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13800000000", mfaEnabledCfg(), false, true)
	reply, err := f.auth.PhoneLogin(context.Background(), "13800000000")
	if err != nil {
		t.Fatalf("login_method=2 用户短信登录应成功: %v", err)
	}
	if reply.GetToken() == "" || reply.GetNeedMfa() || reply.GetNeedSms() {
		t.Fatalf("应直接签发正式 token, got %+v", reply)
	}
}

// 锁 10：短信第二步——正确码换正式 token（中间态 flag 全清），错码拒绝。
func TestVerifySmsLoginConsumesCode(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13800000000", mfaEnabledCfg(), false, true)
	f.sms.code = "654321"
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("第一步应返回短信中间态: %v", err)
	}

	if _, verr := f.auth.VerifySmsLogin(context.Background(), reply.GetMfaToken(), "000000"); verr == nil {
		t.Fatal("错误短信码必须拒绝")
	} else if !strings.Contains(verr.Error(), "短信验证码错误") {
		t.Fatalf("错误文案应提示验证码错误, got %v", verr)
	}

	final, err := f.auth.VerifySmsLogin(context.Background(), reply.GetMfaToken(), "654321")
	if err != nil {
		t.Fatalf("正确短信码应换正式 token: %v", err)
	}
	if final.GetToken() == "" {
		t.Fatal("应签发正式 token")
	}
	c := f.claims(t, final.GetToken())
	if c.MfaPending || c.SmsPending || c.MustEnrollMfa {
		t.Fatalf("正式 token 不得携带任何中间态 flag, got %+v", c)
	}
}

// 锁 11（防串道）：TOTP 中间态走短信校验 / 短信中间态走 /mfa/verify 都必须拒绝。
func TestPendingTokenChannelIsolated(t *testing.T) {
	// 11a：TOTP 中间态 → 短信校验接口拒绝
	totpFixture := newLoginMethodFixture(t, LoginMethodTOTP, "13800000000", mfaEnabledCfg(), true, true)
	reply, err := totpFixture.login(t)
	if err != nil {
		t.Fatalf("TOTP 第一步应成功: %v", err)
	}
	if _, err := totpFixture.auth.VerifySmsLogin(context.Background(), reply.GetMfaToken(), "123456"); err == nil {
		t.Fatal("TOTP 中间态不得通过短信校验接口换 token")
	}

	// 11b：短信中间态 → MfaUseCase.Verify（TOTP）拒绝
	smsFixture := newLoginMethodFixture(t, LoginMethodSMS, "13800000000", mfaEnabledCfg(), false, true)
	smsFixture.sms.code = "123456"
	smsReply, err := smsFixture.login(t)
	if err != nil {
		t.Fatalf("短信第一步应成功: %v", err)
	}
	authConf := &conf.Auth{
		EcdsaPrivateKey: smsFixture.ecdsaKeyPEM(t),
		Expires:         durationpb.New(time.Hour),
	}
	mfaUC, err := NewMfaUseCase(authConf, smsFixture.auth.mfaSvc, &loginStubUserRepo{user: smsFixture.user},
		loginStubRoleRepo{}, newMessageTestLogger())
	if err != nil {
		t.Fatalf("构造 MfaUseCase 失败: %v", err)
	}
	if _, err := mfaUC.Verify(context.Background(), smsReply.GetMfaToken(), "123456"); err == nil {
		t.Fatal("短信中间态不得通过 /mfa/verify 换 token")
	}
}

// 锁 12：两步之间登录方式被改成 TOTP → 短信会话作废（不允许用旧中间态继续换 token）。
func TestVerifySmsLoginRejectsWhenMethodSwitched(t *testing.T) {
	f := newLoginMethodFixture(t, LoginMethodSMS, "13800000000", mfaEnabledCfg(), false, true)
	f.sms.code = "111111"
	reply, err := f.login(t)
	if err != nil {
		t.Fatalf("第一步应返回短信中间态: %v", err)
	}
	f.user.LoginMethod = LoginMethodTOTP // 管理员在两步之间改成 TOTP
	_, err = f.auth.VerifySmsLogin(context.Background(), reply.GetMfaToken(), "111111")
	if err == nil {
		t.Fatal("登录方式已变更为 TOTP 时，短信中间态必须作废")
	}
	if !strings.Contains(err.Error(), "登录方式已变更") {
		t.Fatalf("错误文案应提示登录方式已变更, got %v", err)
	}
}

// 锁 13（归一）：login_method 脏值（0 / 未知）按 TOTP 处理（落到更严的一侧）。
func TestNormalizeLoginMethodFailsClosed(t *testing.T) {
	cases := map[int32]int32{
		0:  LoginMethodTOTP,
		1:  LoginMethodTOTP,
		2:  LoginMethodSMS,
		3:  LoginMethodTOTP,
		-1: LoginMethodTOTP,
	}
	for in, want := range cases {
		if got := normalizeLoginMethod(in); got != want {
			t.Fatalf("normalizeLoginMethod(%d)=%d, want %d", in, got, want)
		}
	}
}
