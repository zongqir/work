package render

type EffectivePolicy struct {
	TenantID    string          `json:"tenant_id"`
	MessageType string          `json:"message_type"`
	Channels    []ChannelPolicy `json:"channels"`
}

type ChannelPolicy struct {
	Channel      string         `json:"channel"`
	TemplateCode string         `json:"template_code,omitempty"`
	Template     TemplateConfig `json:"template,omitempty"`
	Audience     AudienceConfig `json:"audience,omitempty"`
	Delivery     DeliveryConfig `json:"delivery,omitempty"`
}

type TemplateConfig struct {
	TemplateCode string `json:"template_code,omitempty"`
	TemplateKey  string `json:"template_key,omitempty"`
}

type AudienceConfig struct {
	To         []string `json:"to,omitempty"`
	Cc         []string `json:"cc,omitempty"`
	Bcc        []string `json:"bcc,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Phone      []string `json:"phone,omitempty"`
}

type DeliveryConfig struct {
	Platform string            `json:"platform,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	AgentID  string            `json:"agent_id,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

func (p ChannelPolicy) ResolvedTemplateCode() string {
	if p.Template.TemplateCode != "" {
		return p.Template.TemplateCode
	}
	return p.TemplateCode
}

func (p ChannelPolicy) ResolvedTemplateKey() string {
	if p.Template.TemplateKey != "" {
		return p.Template.TemplateKey
	}
	return p.ResolvedTemplateCode()
}
