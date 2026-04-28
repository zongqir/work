package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"notes/code/aggregate_registry_demo/messages"
)

// BizAggregateRequest 是发给业务方聚合接口的请求。
// 这里只保留平台自己需要的上下文，例如 tenant_id 和查询条件。
type BizAggregateRequest struct {
	TenantID    string          `json:"tenant_id"`
	WindowStart time.Time       `json:"window_start"`
	WindowEnd   time.Time       `json:"window_end"`
	ConfigBody  json.RawMessage `json:"config_body"`
}

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

func buildMessageRenderInput(req *BizAggregateRequest, result *messages.BizAggregateResult) (MessageRenderInput, error) {
	if result.MessageType == "" {
		return MessageRenderInput{}, fmt.Errorf("message_type is required")
	}
	if len(result.BizVars) == 0 {
		return MessageRenderInput{}, fmt.Errorf("biz_vars is required")
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

func renderByPolicy(req *BizAggregateRequest, result *messages.BizAggregateResult, policy *EffectivePolicy, templateRoot string) ([]RenderedChannelMessage, error) {
	if policy.TenantID != req.TenantID {
		return nil, fmt.Errorf("policy tenant_id mismatch: %s", policy.TenantID)
	}
	if policy.MessageType != result.MessageType {
		return nil, fmt.Errorf("policy message_type mismatch: %s", policy.MessageType)
	}

	input, err := buildMessageRenderInput(req, result)
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
		subject, err := renderTextTemplate(filepath.Join(templateRoot, "email", policy.TemplateCode+".subject.tmpl"), input.Vars)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		body, err := renderTextTemplate(filepath.Join(templateRoot, "email", policy.TemplateCode+".body.tmpl"), input.Vars)
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
		content, err := renderTextTemplate(filepath.Join(templateRoot, "webhook", policy.TemplateCode+".tmpl"), input.Vars)
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
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read template failed: %w", err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).
		Option("missingkey=error").
		Parse(string(data))
	if err != nil {
		return "", fmt.Errorf("parse template failed: %w", err)
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
