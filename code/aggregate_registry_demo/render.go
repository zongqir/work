package aggregate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"notes/code/aggregate_registry_demo/messages"
)

// EffectivePolicy 是通知执行层根据 tenant_id + message_type 查到的生效策略。
type EffectivePolicy struct {
	TenantID    string          `json:"tenant_id"`
	MessageType string          `json:"message_type"`
	Channels    []ChannelPolicy `json:"channels"`
}

type ChannelPolicy struct {
	Channel      string `json:"channel"`
	TemplateCode string `json:"template_code"`
}

// MessageRenderInput 是模板最终拿到的输入。
type MessageRenderInput struct {
	Vars map[string]messages.TemplateVars
}

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

func loadBizAggregateRequest(path string) (*BizAggregateRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var req BizAggregateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func loadBizAggregateResult(path string) (*messages.BizAggregateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result messages.BizAggregateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadEffectivePolicy(path string) (*EffectivePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy EffectivePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

func BuildMessageRenderInput(req *BizAggregateRequest, result *messages.BizAggregateResult) (MessageRenderInput, error) {
	if req == nil {
		return MessageRenderInput{}, fmt.Errorf("%w: aggregate request is required", ErrInvalidRequest)
	}
	if result == nil {
		return MessageRenderInput{}, fmt.Errorf("%w: aggregate result is required", ErrInvalidRequest)
	}
	if len(result.BizVars) == 0 {
		return MessageRenderInput{}, fmt.Errorf("%w: biz_vars is required", ErrInvalidRequest)
	}

	bizVars := make(messages.TemplateVars, len(result.BizVars))
	for key, value := range result.BizVars {
		bizVars[key] = value
	}
	sysVars := messages.TemplateVars{
		"window_label": formatWindowLabel(req.WindowStart, req.WindowEnd),
	}

	return MessageRenderInput{
		Vars: map[string]messages.TemplateVars{
			"biz": bizVars,
			"sys": sysVars,
		},
	}, nil
}

func RenderByPolicy(req *BizAggregateRequest, result *messages.BizAggregateResult, policy *EffectivePolicy, templateRoot string) ([]RenderedChannelMessage, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: aggregate request is required", ErrInvalidRequest)
	}
	if result == nil {
		return nil, fmt.Errorf("%w: aggregate result is required", ErrInvalidRequest)
	}
	if policy == nil {
		return nil, fmt.Errorf("%w: effective policy is required", ErrInvalidRequest)
	}
	if policy.TenantID != req.TenantID {
		return nil, fmt.Errorf("policy tenant_id mismatch: %s", policy.TenantID)
	}
	if policy.MessageType == "" {
		return nil, fmt.Errorf("%w: policy message_type is required", ErrInvalidRequest)
	}

	input, err := BuildMessageRenderInput(req, result)
	if err != nil {
		return nil, err
	}

	renderedMessages := make([]RenderedChannelMessage, 0, len(policy.Channels))
	for _, channelPolicy := range policy.Channels {
		rendered, err := renderChannel(input, channelPolicy, templateRoot)
		if err != nil {
			return nil, err
		}
		renderedMessages = append(renderedMessages, rendered)
	}

	return renderedMessages, nil
}

func renderChannel(input MessageRenderInput, policy ChannelPolicy, templateRoot string) (RenderedChannelMessage, error) {
	switch policy.Channel {
	case "email":
		subjectPath, err := templatePath(templateRoot, "email", policy.TemplateCode+".subject.tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		subject, err := renderTextTemplate(subjectPath, input.Vars)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		bodyPath, err := templatePath(templateRoot, "email", policy.TemplateCode+".body.tmpl")
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		body, err := renderTextTemplate(bodyPath, input.Vars)
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
		content, err := renderTextTemplate(contentPath, input.Vars)
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
				Params:       buildSMSParams(input),
			},
		}, nil
	default:
		return RenderedChannelMessage{}, fmt.Errorf("unsupported channel: %s", policy.Channel)
	}
}

func templatePath(templateRoot, channelDir, templateName string) (string, error) {
	if templateName == "" {
		return "", fmt.Errorf("%w: template name is required", ErrInvalidRequest)
	}

	channelRoot := filepath.Clean(filepath.Join(templateRoot, channelDir))
	fullPath := filepath.Clean(filepath.Join(channelRoot, templateName))
	rel, err := filepath.Rel(channelRoot, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve template path failed: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: template path escapes channel root", ErrInvalidRequest)
	}
	return fullPath, nil
}

func buildSMSParams(input MessageRenderInput) map[string]string {
	params := make(map[string]string)
	for key, value := range input.Vars["biz"] {
		switch v := value.(type) {
		case string:
			params[key] = v
		case fmt.Stringer:
			params[key] = v.String()
		case int:
			params[key] = fmt.Sprintf("%d", v)
		case int8:
			params[key] = fmt.Sprintf("%d", v)
		case int16:
			params[key] = fmt.Sprintf("%d", v)
		case int32:
			params[key] = fmt.Sprintf("%d", v)
		case int64:
			params[key] = fmt.Sprintf("%d", v)
		case uint:
			params[key] = fmt.Sprintf("%d", v)
		case uint8:
			params[key] = fmt.Sprintf("%d", v)
		case uint16:
			params[key] = fmt.Sprintf("%d", v)
		case uint32:
			params[key] = fmt.Sprintf("%d", v)
		case uint64:
			params[key] = fmt.Sprintf("%d", v)
		case float32:
			params[key] = fmt.Sprintf("%g", v)
		case float64:
			params[key] = fmt.Sprintf("%g", v)
		case bool:
			params[key] = fmt.Sprintf("%t", v)
		}
	}
	for key, value := range input.Vars["sys"] {
		switch v := value.(type) {
		case string:
			params[key] = v
		case fmt.Stringer:
			params[key] = v.String()
		case int:
			params[key] = fmt.Sprintf("%d", v)
		case int8:
			params[key] = fmt.Sprintf("%d", v)
		case int16:
			params[key] = fmt.Sprintf("%d", v)
		case int32:
			params[key] = fmt.Sprintf("%d", v)
		case int64:
			params[key] = fmt.Sprintf("%d", v)
		case uint:
			params[key] = fmt.Sprintf("%d", v)
		case uint8:
			params[key] = fmt.Sprintf("%d", v)
		case uint16:
			params[key] = fmt.Sprintf("%d", v)
		case uint32:
			params[key] = fmt.Sprintf("%d", v)
		case uint64:
			params[key] = fmt.Sprintf("%d", v)
		case float32:
			params[key] = fmt.Sprintf("%g", v)
		case float64:
			params[key] = fmt.Sprintf("%g", v)
		case bool:
			params[key] = fmt.Sprintf("%t", v)
		}
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
