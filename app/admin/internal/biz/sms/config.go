package sms

// ProviderConfig 消息（短信）提供商配置
type ProviderConfig struct {
	Name            string            `json:"name"`
	GatewayURL      string            `json:"gatewayUrl"`
	AccessKeyID     string            `json:"accessKeyId"`
	AccessKeySecret string            `json:"accessKeySecret"`
	SignName        string            `json:"signName"`
	TemplateCode    string            `json:"templateCode"`
	Extra           map[string]string `json:"extra"`
}
