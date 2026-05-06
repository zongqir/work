package render

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

type RenderedChannelMessage struct {
	Channel string           `json:"channel"`
	Email   *RenderedEmail   `json:"email,omitempty"`
	Webhook *RenderedWebhook `json:"webhook,omitempty"`
	SMS     *RenderedSMS     `json:"sms,omitempty"`
}

type RenderedEmail struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type RenderedWebhook struct {
	Content string `json:"content"`
}

type RenderedSMS struct {
	TemplateCode string            `json:"template_code"`
	Params       map[string]string `json:"params"`
}

func BuildTemplateContext(req *contract.BizAggregateRequest, result *contract.BizAggregateResult) (map[string]contract.TemplateVars, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: aggregate request is required", contract.ErrInvalidRequest)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: aggregate result is required", contract.ErrInvalidRequest)
	}
	if len(result.BizVars) == 0 {
		return nil, fmt.Errorf("%w: biz_vars is required", contract.ErrInvalidRequest)
	}
	sysVars := contract.TemplateVars{
		"window_label": formatWindowLabel(req.WindowStart, req.WindowEnd),
	}

	return map[string]contract.TemplateVars{
		"biz": result.BizVars,
		"sys": sysVars,
	}, nil
}

func RenderByPolicy(req *contract.BizAggregateRequest, result *contract.BizAggregateResult, policy *EffectivePolicy, templateRoot string) ([]RenderedChannelMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: aggregate request is required", contract.ErrInvalidRequest)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: aggregate result is required", contract.ErrInvalidRequest)
	}
	if policy == nil {
		return nil, fmt.Errorf("%w: effective policy is required", contract.ErrInvalidRequest)
	}
	if policy.TenantID != req.TenantID {
		return nil, fmt.Errorf("policy tenant_id mismatch: %s", policy.TenantID)
	}
	if policy.MessageType == "" {
		return nil, fmt.Errorf("%w: policy message_type is required", contract.ErrInvalidRequest)
	}

	context, err := BuildTemplateContext(req, result)
	if err != nil {
		return nil, err
	}

	renderedMessages := make([]RenderedChannelMessage, 0, len(policy.Channels))
	for _, channelPolicy := range policy.Channels {
		rendered, err := renderChannel(context, channelPolicy, templateRoot)
		if err != nil {
			return nil, err
		}
		renderedMessages = append(renderedMessages, rendered)
	}

	return renderedMessages, nil
}

func renderChannel(context map[string]contract.TemplateVars, policy ChannelPolicy, templateRoot string) (RenderedChannelMessage, error) {
	switch policy.Channel {
	case "email":
		subjectPath, err := templatePath(templateRoot, "email", policy.TemplateCode+".subject.tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		subject, err := renderTextTemplate(subjectPath, context)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		bodyPath, err := templatePath(templateRoot, "email", policy.TemplateCode+".body.tmpl")
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
				TemplateCode: policy.TemplateCode,
				Params:       buildSMSParams(context),
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

func buildSMSParams(context map[string]contract.TemplateVars) map[string]string {
	params := make(map[string]string)
	for key, value := range context["biz"] {
		params[key] = fmt.Sprint(value)
	}
	for key, value := range context["sys"] {
		params[key] = fmt.Sprint(value)
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
