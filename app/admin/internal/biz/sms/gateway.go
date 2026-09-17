package sms

import (
	"encoding/json"
	"time"
)

// TableNameSysSmsGateway 短信网关配置表（三端统一，替代 go-payAdmin 的 pre_sms_gateway）
const TableNameSysSmsGateway = "sys_sms_gateway"

// SysSmsGateway 短信网关配置（sys_sms_gateway）。
// 按功能 ID（feature_id）+ 类型（sms_type）选择网关；status=1 启用。
type SysSmsGateway struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement:true;comment:序号" json:"id"`
	Name            string    `gorm:"column:name;comment:网关名称" json:"name"`
	FeatureID       int32     `gorm:"column:feature_id;index;comment:功能 ID（区分业务场景，201=手机登录）" json:"feature_id"`
	SmsType         string    `gorm:"column:sms_type;comment:短信类型（generic/aliyun 等）" json:"sms_type"`
	SignName        string    `gorm:"column:sign_name;comment:短信签名" json:"sign_name"`
	AccessKeyID     string    `gorm:"column:access_key_id;comment:AccessKey ID" json:"access_key_id"`
	AccessKeySecret string    `gorm:"column:access_key_secret;comment:AccessKey Secret" json:"access_key_secret"`
	SmsURL          string    `gorm:"column:sms_url;comment:短信网关 URL" json:"sms_url"`
	TemplateCode    string    `gorm:"column:template_code;comment:模板内容/代码（#code# 为验证码占位符）" json:"template_code"`
	Status          int32     `gorm:"column:status;not null;default:1;comment:是否可用（0=禁用，1=启用）" json:"status"`
	Ext             string    `gorm:"column:ext;comment:扩展配置（JSON）" json:"ext"`
	CreatedAt       time.Time `gorm:"column:created_at;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;comment:更新时间" json:"updated_at"`
}

// TableName 表名
func (*SysSmsGateway) TableName() string { return TableNameSysSmsGateway }

// ToProviderConfig 网关记录 → ProviderConfig
func ToProviderConfig(g *SysSmsGateway) *ProviderConfig {
	cfg := &ProviderConfig{
		Name:            g.SmsType,
		GatewayURL:      g.SmsURL,
		AccessKeyID:     g.AccessKeyID,
		AccessKeySecret: g.AccessKeySecret,
		SignName:        g.SignName,
		TemplateCode:    g.TemplateCode,
		Extra:           make(map[string]string),
	}
	if g.Ext != "" {
		_ = json.Unmarshal([]byte(g.Ext), &cfg.Extra)
	}
	return cfg
}
