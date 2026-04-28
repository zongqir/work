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

// AggregateRequest 是输入。
// 这里故意保留 config_body 这类 JSON 字段，表示请求侧可以更灵活。
type AggregateRequest struct {
	TenantID    string          `json:"tenant_id"`
	WindowStart time.Time       `json:"window_start"`
	WindowEnd   time.Time       `json:"window_end"`
	ConfigBody  json.RawMessage `json:"config_body"`
}

// RenderView 是模板最终拿到的输入。
type RenderView struct {
	WindowLabel string
	Payload     any
}

func loadRequest(path string) (*AggregateRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var req AggregateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func loadEnvelope(path string) (*AggregateEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var envelope AggregateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func buildRenderView(req *AggregateRequest, envelope *AggregateEnvelope) (string, any, error) {
	if envelope.MessageType == "" {
		return "", nil, fmt.Errorf("message_type is required")
	}
	if len(envelope.Payload) == 0 {
		return "", nil, fmt.Errorf("payload is required")
	}

	spec, ok := registry[envelope.MessageType]
	if !ok {
		return "", nil, fmt.Errorf("unsupported message_type: %s", envelope.MessageType)
	}

	payload := spec.NewPayload()
	if err := json.Unmarshal(envelope.Payload, payload); err != nil {
		return "", nil, fmt.Errorf("decode payload failed: %w", err)
	}

	return spec.TemplateCode, RenderView{
		WindowLabel: formatWindowLabel(req.WindowStart, req.WindowEnd),
		Payload:     payload,
	}, nil
}

func renderSummary(templateCode string, view any, channel, templateRoot string) (string, error) {
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
	if err := tmpl.Execute(&buf, view); err != nil {
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
