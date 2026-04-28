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
// 这里故意保留 config_body 这类 JSON 字段，表示请求侧可以更灵活。
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
	WindowLabel string
	Payload     any
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

func buildMessageRenderInput(req *BizAggregateRequest, result *messages.BizAggregateResult) (messages.BizAggregateResultMeta, MessageRenderInput, error) {
	if result.MessageType == "" {
		return messages.BizAggregateResultMeta{}, MessageRenderInput{}, fmt.Errorf("message_type is required")
	}
	if len(result.Payload) == 0 {
		return messages.BizAggregateResultMeta{}, MessageRenderInput{}, fmt.Errorf("payload is required")
	}

	meta, ok := messages.LookupBizAggregateResultMeta(result.MessageType)
	if !ok {
		return messages.BizAggregateResultMeta{}, MessageRenderInput{}, fmt.Errorf("unsupported message_type: %s", result.MessageType)
	}

	payload := meta.NewPayload()
	if err := json.Unmarshal(result.Payload, payload); err != nil {
		return messages.BizAggregateResultMeta{}, MessageRenderInput{}, fmt.Errorf("decode payload failed: %w", err)
	}

	return meta, MessageRenderInput{
		WindowLabel: formatWindowLabel(req.WindowStart, req.WindowEnd),
		Payload:     payload,
	}, nil
}

func renderByPolicy(req *BizAggregateRequest, result *messages.BizAggregateResult, policy *EffectivePolicy, templateRoot string) ([]RenderedChannelMessage, error) {
	if policy.TenantID != req.TenantID {
		return nil, fmt.Errorf("policy tenant_id mismatch: %s", policy.TenantID)
	}
	if policy.MessageType != result.MessageType {
		return nil, fmt.Errorf("policy message_type mismatch: %s", policy.MessageType)
	}

	meta, input, err := buildMessageRenderInput(req, result)
	if err != nil {
		return nil, err
	}

	renderedMessages := make([]RenderedChannelMessage, 0, len(policy.Channels))
	for _, channelPolicy := range policy.Channels {
		rendered, err := renderChannel(meta, input, channelPolicy, templateRoot)
		if err != nil {
			return nil, err
		}
		renderedMessages = append(renderedMessages, rendered)
	}

	return renderedMessages, nil
}

func renderChannel(meta messages.BizAggregateResultMeta, input MessageRenderInput, policy ChannelPolicy, templateRoot string) (RenderedChannelMessage, error) {
	switch policy.Channel {
	case "email":
		subject, err := renderTextTemplate(filepath.Join(templateRoot, "email", policy.TemplateCode+".subject.tmpl"), input)
		if err != nil {
			return RenderedChannelMessage{}, err
		}
		body, err := renderTextTemplate(filepath.Join(templateRoot, "email", policy.TemplateCode+".body.tmpl"), input)
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
		content, err := renderTextTemplate(filepath.Join(templateRoot, "webhook", policy.TemplateCode+".tmpl"), input)
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
		if meta.BuildSMSParams == nil {
			return RenderedChannelMessage{}, fmt.Errorf("sms renderer is not defined for message_type")
		}
		params, err := meta.BuildSMSParams(input.WindowLabel, input.Payload)
		if err != nil {
			return RenderedChannelMessage{}, fmt.Errorf("build sms params failed: %w", err)
		}
		return RenderedChannelMessage{
			Channel: "sms",
			SMS: &RenderedSMS{
				TemplateCode: policy.TemplateCode,
				Params:       params,
			},
		}, nil
	default:
		return RenderedChannelMessage{}, fmt.Errorf("unsupported channel: %s", policy.Channel)
	}
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

// 统一生成窗口文案。
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
