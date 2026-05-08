package render

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"work/notification/code/pkg/notification/contract"
)

type RenderedChannelMessage struct {
	Channel string           `json:"channel"`
	Email   *RenderedEmail   `json:"email,omitempty"`
	Webhook *RenderedWebhook `json:"webhook,omitempty"`
	SMS     *RenderedSMS     `json:"sms,omitempty"`
	WeCom   *RenderedWeCom   `json:"wecom,omitempty"`
}

type RenderedEmail struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type RenderedWebhook struct {
	Content string `json:"content"`
}

type RenderedSMS struct {
	TemplateParams []RenderedParam `json:"template_params"`
}

type RenderedWeCom struct {
	Text string `json:"text"`
}

type RenderedParam struct {
	ParamName  string `json:"param_name"`
	ParamValue string `json:"param_value"`
}

type RenderInput struct {
	TenantID    string
	MessageType string
	BizVars     contract.TemplateVars
	SystemVars  contract.TemplateVars
}

func BuildTemplateContext(input RenderInput) (map[string]contract.TemplateVars, error) {
	bizVars := input.BizVars
	if bizVars == nil {
		bizVars = contract.TemplateVars{}
	}

	sysVars := input.SystemVars
	if sysVars == nil {
		sysVars = contract.TemplateVars{}
	}

	return map[string]contract.TemplateVars{
		"biz": bizVars,
		"sys": sysVars,
	}, nil
}

func WindowSystemVars(start, end time.Time) contract.TemplateVars {
	if start.IsZero() || end.IsZero() {
		return contract.TemplateVars{}
	}
	return contract.TemplateVars{
		"window_label": formatWindowLabel(start, end),
	}
}

func Render(input RenderInput, policy *EffectivePolicy, templateRoot string) ([]RenderedChannelMessage, error) {
	if input.TenantID == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if input.MessageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}
	if policy == nil {
		return nil, fmt.Errorf("%w: effective policy is required", contract.ErrInvalidRequest)
	}
	if policy.TenantID != input.TenantID {
		return nil, fmt.Errorf("policy tenant_id mismatch: %s", policy.TenantID)
	}
	if policy.MessageType != input.MessageType {
		return nil, fmt.Errorf("policy message_type mismatch: %s", policy.MessageType)
	}

	context, err := BuildTemplateContext(input)
	if err != nil {
		return nil, err
	}

	if policy.Channel.Channel == "" {
		return nil, fmt.Errorf("%w: channel is required", contract.ErrUnsupportedConfig)
	}

	rendered, err := renderChannel(context, policy.Channel, templateRoot)
	if err != nil {
		return nil, err
	}
	return []RenderedChannelMessage{rendered}, nil
}

func renderChannel(context map[string]contract.TemplateVars, policy ChannelPolicy, templateRoot string) (RenderedChannelMessage, error) {
	switch policy.Channel {
	case "email":
		templateCode := policy.TemplateCode
		subjectPath, err := templatePath(templateRoot, "email", templateCode+".subject.tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		subject, err := renderTextTemplate(subjectPath, context)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		bodyPath, err := templatePath(templateRoot, "email", templateCode+".body.tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		body, err := renderTextTemplate(bodyPath, context)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		return RenderedChannelMessage{
			Channel: "email",
			Email: &RenderedEmail{
				Subject: subject,
				Body:    body,
			},
		}, nil
	case "webhook":
		contentPath, err := templatePath(templateRoot, "webhook", policy.TemplateCode+".tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		content, err := renderTextTemplate(contentPath, context)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		return RenderedChannelMessage{
			Channel: "webhook",
			Webhook: &RenderedWebhook{
				Content: content,
			},
		}, nil
	case "sms":
		return RenderedChannelMessage{
			Channel: "sms",
			SMS: &RenderedSMS{
				TemplateParams: buildSMSParams(context),
			},
		}, nil
	case "wecom":
		contentPath, err := templatePath(templateRoot, "wecom", policy.TemplateCode+".tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		text, err := renderTextTemplate(contentPath, context)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		return RenderedChannelMessage{
			Channel: "wecom",
			WeCom: &RenderedWeCom{
				Text: text,
			},
		}, nil
	default:
		return RenderedChannelMessage{}, fmt.Errorf("unsupported channel: %s", policy.Channel)
	}
}

func templatePath(templateRoot, channelDir, templateName string) (string, error) {
	if templateName == "" {
		return "", fmt.Errorf("%w: template name is required", contract.ErrInvalidRequest)
	}

	channelRoot := filepath.Clean(filepath.Join(templateRoot, channelDir))
	fullPath := filepath.Clean(filepath.Join(channelRoot, templateName))
	rel, err := filepath.Rel(channelRoot, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve template path failed: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: template path escapes channel root", contract.ErrInvalidRequest)
	}
	return fullPath, nil
}

func buildSMSParams(context map[string]contract.TemplateVars) []RenderedParam {
	params := make([]RenderedParam, 0, len(context["biz"])+len(context["sys"]))
	for key, value := range context["biz"] {
		params = append(params, RenderedParam{
			ParamName:  key,
			ParamValue: fmt.Sprint(value),
		})
	}
	for key, value := range context["sys"] {
		params = append(params, RenderedParam{
			ParamName:  key,
			ParamValue: fmt.Sprint(value),
		})
	}
	return params
}

func renderTextTemplate(templatePath string, input any) (string, error) {
	tmpl, err := defaultTemplateCache.get(templatePath)
	if err != nil {
		return "", fmt.Errorf("load template failed: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render template failed: %w", err)
	}

	return strings.TrimRight(buf.String(), "\r\n"), nil
}

func formatWindowLabel(start, end time.Time) string {
	duration := end.Sub(start)
	switch duration {
	case time.Hour:
		return "过去1小时"
	case 24 * time.Hour:
		return "过去1天"
	default:
		return fmt.Sprintf("%s - %s", start.Format("2006-01-02 15:04"), end.Format("15:04"))
	}
}
