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

// RenderConfig 是渲染时的运行参数，不是上游请求或返回的一部分。
type RenderConfig struct {
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// RenderView 是模板最终拿到的输入。
type RenderView struct {
	WindowLabel string
	Payload     any
}

func loadConfig(path string) (*RenderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg RenderConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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

func buildRenderView(cfg *RenderConfig, envelope *AggregateEnvelope) (string, any, error) {
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
		WindowLabel: formatWindowLabel(cfg.WindowStart, cfg.WindowEnd),
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
