package sms

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// Service 短信服务接口
type Service interface {
	SendCode(ctx context.Context, mobile string, code string) error
	SendTemplate(ctx context.Context, mobile string, templateCode string, params map[string]string) error
}

// Cache 短信缓存接口（redis.UniversalClient 天然满足）
type Cache interface {
	Get(ctx context.Context, key string) string
	Set(ctx context.Context, key string, value string, expire time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Provider 短信提供商接口
type Provider interface {
	SendCode(ctx context.Context, mobile string, code string) error
	SendTemplate(ctx context.Context, mobile string, templateCode string, params map[string]string) error
}

// UseCase 短信验证码用例
type UseCase struct {
	config      *conf.Message
	cache       Cache
	gatewayRepo GatewayRepo
	providers   map[string]Provider
	log         *logx.Logger
}

// NewUseCase 创建短信用例；构造不查库，启用网关在后台初始化（表缺失仅告警，不阻断启动）
func NewUseCase(config *conf.Message, rdb redis.UniversalClient, gatewayRepo GatewayRepo, logger *logx.Logger) *UseCase {
	uc := &UseCase{
		config:      config,
		cache:       NewRedisCache(rdb),
		gatewayRepo: gatewayRepo,
		providers:   make(map[string]Provider),
		log:         logger,
	}
	if config != nil && config.Enabled {
		uc.initProvidersFromDB(logger)
	}
	return uc
}

// GenerateCode 生成验证码（crypto/rand；numeric=纯数字，simple=大写字母+数字去除易混淆字符）
func (uc *UseCase) GenerateCode(length int32, codeType string) string {
	if length <= 0 {
		length = 6
	}
	var chars string
	switch codeType {
	case "numeric":
		chars = "0123456789"
	default:
		chars = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	}
	b := make([]byte, length)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for _, v := range b {
		sb.WriteByte(chars[int(v)%len(chars)])
	}
	return sb.String()
}

// GetCacheKey 缓存 key
func (uc *UseCase) GetCacheKey(prefix, identifier string) string {
	return fmt.Sprintf("%s%s", prefix, identifier)
}

// SaveCode 保存验证码
func (uc *UseCase) SaveCode(ctx context.Context, key, code string, expireMinutes int32) error {
	return uc.cache.Set(ctx, key, code, time.Duration(expireMinutes)*time.Minute)
}

// VerifyCode 校验验证码（成功即删除，一次性）
func (uc *UseCase) VerifyCode(ctx context.Context, key, code string) (bool, error) {
	cachedCode := uc.cache.Get(ctx, key)
	if cachedCode == "" {
		return false, nil
	}
	if strings.EqualFold(cachedCode, code) {
		_ = uc.cache.Delete(ctx, key)
		return true, nil
	}
	return false, nil
}

// SendSmsCode 发送验证码短信（feature_id=201）；验证码先缓存后发送，发送失败不影响缓存（可重发）
func (uc *UseCase) SendSmsCode(ctx context.Context, mobile, code string, expireMinutes int32) error {
	uc.log.Infow(ctx, "msg", "开始发送验证码", "mobile", mobile, "expireMinutes", expireMinutes)
	provider, err := uc.getProvider(201)
	if err != nil {
		return fmt.Errorf("获取短信服务失败：%w", err)
	}
	if err := provider.SendCode(ctx, mobile, code); err != nil {
		return fmt.Errorf("发送验证码失败：%w", err)
	}
	if expireMinutes > 0 {
		cacheKey := uc.GetCacheKey(uc.config.GetPhoneLogin().GetCachePrefix(), mobile)
		if err := uc.SaveCode(ctx, cacheKey, code, expireMinutes); err != nil {
			uc.log.Errorw(ctx, "msg", "验证码缓存失败", "error", err)
		}
	}
	return nil
}

// IsLoginCaptchaEnabled 登录图形验证码开关（图形码本体由 base64Captcha 下发，此为配置位）
func (uc *UseCase) IsLoginCaptchaEnabled() bool {
	if uc.config == nil || uc.config.LoginCaptcha == nil {
		return false
	}
	return uc.config.Enabled && uc.config.LoginCaptcha.Enabled
}

// IsPhoneLoginEnabled 手机验证码登录开关
func (uc *UseCase) IsPhoneLoginEnabled() bool {
	if uc.config == nil || uc.config.PhoneLogin == nil {
		return false
	}
	return uc.config.Enabled && uc.config.PhoneLogin.Enabled
}

// GetPhoneLoginConfig 手机登录配置
func (uc *UseCase) GetPhoneLoginConfig() *conf.PhoneLogin {
	if uc.config == nil {
		return nil
	}
	return uc.config.PhoneLogin
}

// initProvidersFromDB 从 sys_sms_gateway 加载启用网关（feature_<id> 为 key）
func (uc *UseCase) initProvidersFromDB(logger *logx.Logger) {
	ctx := context.Background()
	gateways, err := uc.gatewayRepo.FindAllEnabled(ctx)
	if err != nil {
		uc.log.Warnw(ctx, "msg", "从数据库加载短信网关配置失败", "error", err)
		return
	}
	uc.log.Infow(ctx, "msg", "开始初始化短信提供商", "count", len(gateways))
	for _, g := range gateways {
		cfg := ToProviderConfig(g)
		// 未知类型统一走通用 HTTP 提供商
		uc.providers[fmt.Sprintf("feature_%d", g.FeatureID)] = NewGenericProvider(cfg, logger)
		uc.log.Infow(ctx, "msg", "短信提供商初始化完成", "name", g.Name, "type", g.SmsType, "feature", g.FeatureID)
	}
}

// getProvider 表注册优先；缺省回退 YAML providers 中 defaultProvider 指定项
func (uc *UseCase) getProvider(featureID int32) (Provider, error) {
	if uc.config == nil || !uc.config.Enabled {
		return nil, fmt.Errorf("消息服务未启用")
	}
	if p, ok := uc.providers[fmt.Sprintf("feature_%d", featureID)]; ok {
		return p, nil
	}
	// YAML 兜底：defaultProvider 命名的提供商
	if name := uc.config.GetDefaultProvider(); name != "" {
		if mp := uc.config.GetProviders(); mp != nil {
			if pc, ok := mp[name]; ok && pc.GetGatewayUrl() != "" {
				return NewGenericProvider(&ProviderConfig{
					Name:            name,
					GatewayURL:      pc.GetGatewayUrl(),
					AccessKeyID:     pc.GetAccessKeyId(),
					AccessKeySecret: pc.GetAccessKeySecret(),
					SignName:        pc.GetSignName(),
					TemplateCode:    pc.GetTemplateCode(),
					Extra:           pc.GetExtra(),
				}, uc.log), nil
			}
		}
	}
	return nil, fmt.Errorf("未找到短信提供商配置 (feature_id=%d)", featureID)
}
