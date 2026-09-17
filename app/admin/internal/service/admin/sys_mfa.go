package admin

// SysMfaService TOTP 双因素认证服务（R26 接入，设计 docs/design/2026-09-12-mfa-totp-integration.md）。
//
// 路由：/mfa/status|enroll|enable|disable|reset（需鉴权）+ /mfa/verify（公开，白名单）。
// 本人操作（status/enroll/enable/disable）取 JWT claims 的 UserID；Reset 需超管。
// 错误映射：pkg/mfa 错误 → pb.SysMfaErrorReason（sys_mfa_error.proto）。

import (
	"context"
	"errors"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
)

// SysMfaService 实现 v1.SysMfaServiceHTTPServer
type SysMfaService struct {
	pb.UnimplementedSysMfaServer
	uc  *admin.MfaUseCase
	log *logx.Logger
}

// NewSysMfaService 创建 MFA 服务
func NewSysMfaService(uc *admin.MfaUseCase, logger *logx.Logger) *SysMfaService {
	return &SysMfaService{uc: uc, log: logger}
}

// Status 查询当前用户 TOTP 状态
func (s *SysMfaService) Status(ctx context.Context, req *pb.MfaStatusReq) (*pb.MfaStatusRsp, error) {
	uid, err := authz.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.uc.Status(ctx, uid.UserID)
	if err != nil {
		return nil, mapMfaError(err)
	}
	rsp := &pb.MfaStatusRsp{Available: st.Available, Enabled: st.Enabled}
	if st.BoundAt != nil {
		rsp.BoundAt = st.BoundAt.UnixMilli()
	}
	return rsp, nil
}

// Enroll 生成 TOTP 密钥与 otpauth URI
func (s *SysMfaService) Enroll(ctx context.Context, req *pb.MfaEnrollReq) (*pb.MfaEnrollRsp, error) {
	uid, err := authz.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	secret, url, err := s.uc.Enroll(ctx, uid.UserID, req.Username)
	if err != nil {
		return nil, mapMfaError(err)
	}
	return &pb.MfaEnrollRsp{Secret: secret, Url: url}, nil
}

// Enable 校验动态码并启用，返回一次性恢复码
func (s *SysMfaService) Enable(ctx context.Context, req *pb.MfaCodeReq) (*pb.MfaEnableRsp, error) {
	uid, err := authz.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := s.uc.Enable(ctx, uid.UserID, req.Code)
	if err != nil {
		return nil, mapMfaError(err)
	}
	return &pb.MfaEnableRsp{RecoveryCodes: codes}, nil
}

// Disable 关闭 TOTP（需动态码或恢复码二次确认）
func (s *SysMfaService) Disable(ctx context.Context, req *pb.MfaCodeReq) (*pb.MfaRsp, error) {
	uid, err := authz.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.uc.Disable(ctx, uid.UserID, req.Code); err != nil {
		return nil, mapMfaError(err)
	}
	return &pb.MfaRsp{}, nil
}

// Reset 管理员重置指定用户 TOTP（仅超管，对齐参考 MfaReset 语义）
func (s *SysMfaService) Reset(ctx context.Context, req *pb.MfaResetReq) (*pb.MfaRsp, error) {
	claims, err := authz.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.IsSuperAdmin(claims.RoleKey) {
		return nil, pb.ErrorMfaRequired("仅超级管理员可重置他人二因素认证")
	}
	if err := s.uc.Reset(ctx, req.UserId); err != nil {
		return nil, mapMfaError(err)
	}
	return &pb.MfaRsp{}, nil
}

// Verify 两段式登录二次验证（公开，白名单）
func (s *SysMfaService) Verify(ctx context.Context, req *pb.MfaVerifyReq) (*pb.MfaVerifyRsp, error) {
	reply, err := s.uc.Verify(ctx, req.MfaToken, req.Code)
	if err != nil {
		return nil, err
	}
	return &pb.MfaVerifyRsp{Token: reply.Token, Expire: reply.Expire}, nil
}

// mapMfaError pkg/mfa 错误 → pb.SysMfaErrorReason 错误
func mapMfaError(err error) error {
	switch {
	case errors.Is(err, mfa.ErrDisabled):
		return pb.ErrorMfaDisabled("二因素认证未开启")
	case errors.Is(err, mfa.ErrAlreadyEnabled):
		return pb.ErrorMfaAlreadyEnabled("该用户已开启二因素认证")
	case errors.Is(err, mfa.ErrNotEnabled):
		return pb.ErrorMfaNotEnabled("该用户未开启二因素认证")
	case errors.Is(err, mfa.ErrCodeInvalid):
		return pb.ErrorMfaVerifyFailed("动态验证码错误或已过期")
	case errors.Is(err, mfa.ErrEnrollExpired):
		return pb.ErrorMfaEnrollExpired("绑定会话已过期，请重新生成密钥")
	case errors.Is(err, mfa.ErrNoUser):
		return pb.ErrorMfaNoUser("用户不存在")
	default:
		return err
	}
}