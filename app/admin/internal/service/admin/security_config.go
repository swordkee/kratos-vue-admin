package admin

import (
	"context"
	"errors"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

// SysSecurityConfigService 登录安全策略（gva api/v1/system/sys_security_config.go 迁入）。
type SysSecurityConfigService struct {
	pb.UnimplementedSysSecurityConfigServer
	cfg  *biz.SecurityConfigUseCase
	lock *biz.SecurityLockUseCase
	log  *log.Helper
}

func NewSysSecurityConfigService(cfg *biz.SecurityConfigUseCase, lock *biz.SecurityLockUseCase, logger log.Logger) *SysSecurityConfigService {
	return &SysSecurityConfigService{
		cfg:  cfg,
		lock: lock,
		log:  log.NewHelper(log.With(logger, "module", "service/sys_security_config")),
	}
}

func convertSecurityConfig(cfg model.SysSecurityConfig) *pb.SecurityConfigItem {
	return &pb.SecurityConfigItem{
		Id:                         uint64(cfg.ID),
		CaptchaOpen:                cfg.CaptchaOpen,
		CaptchaTimeout:             cfg.CaptchaTimeout,
		KeyLong:                    cfg.KeyLong,
		ImgWidth:                   cfg.ImgWidth,
		ImgHeight:                  cfg.ImgHeight,
		PwdMinLength:               cfg.PwdMinLength,
		PwdRequireUpper:            cfg.PwdRequireUpper,
		PwdRequireLower:            cfg.PwdRequireLower,
		PwdRequireDigit:            cfg.PwdRequireDigit,
		PwdRequireSpecial:          cfg.PwdRequireSpecial,
		LimitEnable:                cfg.LimitEnable,
		LimitWindow:                cfg.LimitWindow,
		LimitCount:                 cfg.LimitCount,
		LockEnable:                 cfg.LockEnable,
		LockThreshold:              cfg.LockThreshold,
		LockDuration:               cfg.LockDuration,
		PwdExpireEnable:            cfg.PwdExpireEnable,
		PwdExpireDays:              cfg.PwdExpireDays,
		ForceNewUserChangePassword: cfg.ForceNewUserChangePassword,
		MfaRequired:                cfg.MfaRequired,
		CreatedAt:                  cfg.CreatedAt.Format(sysParamsTimeLayout),
		UpdatedAt:                  cfg.UpdatedAt.Format(sysParamsTimeLayout),
	}
}

// sysParamsTimeLayout 时间字段展示格式（对齐仓内其它 service 的 "2006-01-02 15:04:05"）。
const sysParamsTimeLayout = "2006-01-02 15:04:05"

func (s *SysSecurityConfigService) GetSecurityConfig(ctx context.Context, _ *pb.GetSecurityConfigRequest) (*pb.GetSecurityConfigReply, error) {
	cfg, err := s.cfg.Get(ctx)
	if err != nil {
		s.log.WithContext(ctx).Errorf("获取安全配置失败: %v", err)
		return nil, errors.New("获取安全配置失败")
	}
	s.cfg.Current(ctx) // 预热缓存（Get 成功后再 Current 直接命中）
	return &pb.GetSecurityConfigReply{Config: convertSecurityConfig(cfg)}, nil
}

func (s *SysSecurityConfigService) UpdateSecurityConfig(ctx context.Context, req *pb.UpdateSecurityConfigRequest) (*pb.UpdateSecurityConfigReply, error) {
	cfg, err := s.cfg.Update(ctx, model.SysSecurityConfig{
		CaptchaOpen:                req.GetCaptchaOpen(),
		CaptchaTimeout:             req.GetCaptchaTimeout(),
		KeyLong:                    req.GetKeyLong(),
		ImgWidth:                   req.GetImgWidth(),
		ImgHeight:                  req.GetImgHeight(),
		PwdMinLength:               req.GetPwdMinLength(),
		PwdRequireUpper:            req.GetPwdRequireUpper(),
		PwdRequireLower:            req.GetPwdRequireLower(),
		PwdRequireDigit:            req.GetPwdRequireDigit(),
		PwdRequireSpecial:          req.GetPwdRequireSpecial(),
		LimitEnable:                req.GetLimitEnable(),
		LimitWindow:                req.GetLimitWindow(),
		LimitCount:                 req.GetLimitCount(),
		LockEnable:                 req.GetLockEnable(),
		LockThreshold:              req.GetLockThreshold(),
		LockDuration:               req.GetLockDuration(),
		PwdExpireEnable:            req.GetPwdExpireEnable(),
		PwdExpireDays:              req.GetPwdExpireDays(),
		ForceNewUserChangePassword: req.GetForceNewUserChangePassword(),
		MfaRequired:                req.GetMfaRequired(),
	})
	if err != nil {
		s.log.WithContext(ctx).Errorf("设置安全配置失败: %v", err)
		return nil, errors.New("设置安全配置失败")
	}
	return &pb.UpdateSecurityConfigReply{Config: convertSecurityConfig(cfg)}, nil
}

func (s *SysSecurityConfigService) GetLoginLock(ctx context.Context, req *pb.GetLoginLockRequest) (*pb.GetLoginLockReply, error) {
	if req.GetUsername() == "" {
		return nil, errors.New("缺少用户名")
	}
	st := s.lock.Status(ctx, req.GetUsername())
	return &pb.GetLoginLockReply{
		Username:      st.Username,
		LockEnable:    st.LockEnable,
		Locked:        st.Locked,
		FailCount:     st.FailCount,
		RemainSeconds: st.RemainSeconds,
		LockThreshold: st.Threshold,
		LockDuration:  st.DurationMin,
	}, nil
}

func (s *SysSecurityConfigService) UnlockLogin(ctx context.Context, req *pb.UnlockLoginRequest) (*pb.UnlockLoginReply, error) {
	if req.GetUsername() == "" {
		return nil, errors.New("缺少用户名")
	}
	if err := s.lock.Unlock(ctx, req.GetUsername()); err != nil {
		s.log.WithContext(ctx).Errorf("解锁失败: %v", err)
		return nil, errors.New("解锁失败")
	}
	return &pb.UnlockLoginReply{Username: req.GetUsername(), Cleared: true}, nil
}
