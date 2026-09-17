package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// GenericProvider 通用短信服务实现（HTTP form 接口，account/pswd/mobile/msg 协议）
type GenericProvider struct {
	config     *ProviderConfig
	httpClient *http.Client
	log        *logx.Logger
}

// NewGenericProvider 创建通用短信服务实例
func NewGenericProvider(config *ProviderConfig, logger *logx.Logger) *GenericProvider {
	return &GenericProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: logger,
	}
}

// SendCode 发送验证码短信（TemplateCode 中 #code# 为验证码占位符）
func (p *GenericProvider) SendCode(ctx context.Context, mobile string, code string) error {
	p.log.Infow(ctx, "msg", "开始发送验证码短信", "mobile", mobile)

	content := strings.Replace(p.config.TemplateCode, "#code#", code, 1)

	params := url.Values{}
	params.Set("account", p.config.AccessKeyID)
	params.Set("pswd", p.config.AccessKeySecret)
	params.Set("mobile", mobile)
	params.Set("msg", content)
	params.Set("resptype", "json")
	params.Set("needstatus", "true")
	for k, v := range p.config.Extra {
		params.Set(k, v)
	}

	respBody, err := p.sendRequest(ctx, params)
	if err != nil {
		return fmt.Errorf("发送验证码失败：%w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析响应失败：%w", err)
	}
	if retCode, ok := result["ret"].(string); ok && retCode != "0" {
		retMsg, _ := result["msg"].(string)
		return fmt.Errorf("短信发送失败：%s - %s", retCode, retMsg)
	}
	if codeVal, ok := result["code"].(string); ok && codeVal != "OK" && codeVal != "200" {
		msg, _ := result["message"].(string)
		if msg == "" {
			msg, _ = result["msg"].(string)
		}
		return fmt.Errorf("短信发送失败：%s - %s", codeVal, msg)
	}
	p.log.Infow(ctx, "msg", "验证码发送成功", "mobile", mobile)
	return nil
}

// SendTemplate 发送模板短信（#key# 占位符替换）
func (p *GenericProvider) SendTemplate(ctx context.Context, mobile, templateCode string, params map[string]string) error {
	content := templateCode
	for k, v := range params {
		content = strings.ReplaceAll(content, "#"+k+"#", v)
	}
	form := url.Values{}
	form.Set("account", p.config.AccessKeyID)
	form.Set("pswd", p.config.AccessKeySecret)
	form.Set("mobile", mobile)
	form.Set("msg", content)
	form.Set("resptype", "json")
	for k, v := range p.config.Extra {
		form.Set(k, v)
	}
	if _, err := p.sendRequest(ctx, form); err != nil {
		return fmt.Errorf("发送模板短信失败：%w", err)
	}
	return nil
}

func (p *GenericProvider) sendRequest(ctx context.Context, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.config.GatewayURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
