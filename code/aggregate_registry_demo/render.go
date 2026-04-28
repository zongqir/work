package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// BizAggregateRequest 是发给业务方聚合接口的请求。
// 这里故意保留 config_body 这类 JSON 字段，表示请求侧可以更灵活。
type BizAggregateRequest struct {
	TenantID    string          `json:"tenant_id"`
	WindowStart time.Time       `json:"window_start"`
	WindowEnd   time.Time       `json:"window_end"`
	ConfigBody  json.RawMessage `json:"config_body"`
}

// MessageRenderInput 是模板最终拿到的输入。
type MessageRenderInput struct {
	WindowLabel string
	Payload     any
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

func loadBizAggregateResult(path string) (*BizAggregateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result BizAggregateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func buildMessageRenderInput(req *BizAggregateRequest, result *BizAggregateResult) (string, any, error) {
	if result.MessageType == "" {
		return "", nil, fmt.Errorf("message_type is required")
	}
	if len(result.Payload) == 0 {
		return "", nil, fmt.Errorf("payload is required")
	}

	meta, ok := bizAggregateResultRegistry[result.MessageType]
	if !ok {
		return "", nil, fmt.Errorf("unsupported message_type: %s", result.MessageType)
	}

	payload := meta.NewPayload()
	if err := json.Unmarshal(result.Payload, payload); err != nil {
		return "", nil, fmt.Errorf("decode payload failed: %w", err)
	}

	return result.MessageType, MessageRenderInput{
		WindowLabel: formatWindowLabel(req.WindowStart, req.WindowEnd),
		Payload:     payload,
	}, nil
}

func renderMessage(templateCode string, input any, channel, templateRoot string) (string, error) {
	templatePath := filepath.Join(templateRoot, templateCode, channel+".tmpl")
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

	return buf.String(), nil
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
