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

type RenderConfig struct {
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// AggregateEnvelope 是上游返回结果。
// 顶层固定为 message_type + payload。
type AggregateEnvelope struct {
	MessageType string          `json:"message_type"`
	Payload     json.RawMessage `json:"payload"`
}

// XdrRiskDigest 是 xdr_risk_digest 这类消息对应的返回结构。
type XdrRiskDigest struct {
	TotalCount    int              `json:"total_count"`
	CategoryCount int              `json:"category_count"`
	Examples      []XdrRiskExample `json:"examples"`
}

// XdrRiskExample 是案例明细。
type XdrRiskExample struct {
	ObjectName string `json:"object_name"`
	RiskType   string `json:"risk_type"`
	EventCount int    `json:"event_count"`
}

// RenderView 是模板最终拿到的输入。
type RenderView struct {
	WindowLabel string
	Payload     any
}

// MessageSpec 描述某一种消息该怎么解码、该用哪套模板。
type MessageSpec struct {
	TemplateCode string
	NewPayload   func() any
}

var registry = map[string]MessageSpec{
	"xdr_risk_digest": {
		TemplateCode: "xdr_risk_digest",
		NewPayload: func() any {
			return &XdrRiskDigest{}
		},
	},
}

func main() {
	// 读取渲染参数。
	cfg, err := loadConfig("code/aggregate_registry_demo/sample_config.json")
	if err != nil {
		panic(err)
	}

	// 读取上游返回结果。
	envelope, err := loadEnvelope("code/aggregate_registry_demo/sample_result.json")
	if err != nil {
		panic(err)
	}

	// 通过注册表把上游返回结果转换成模板输入。
	templateCode, view, err := buildRenderView(cfg, envelope)
	if err != nil {
		panic(err)
	}

	// 按不同渠道分别渲染最终消息。
	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderSummary(templateCode, view, channel, "code/aggregate_registry_demo/templates")
		if err != nil {
			panic(err)
		}
		fmt.Printf("[%s]\n%s\n\n", channel, rendered)
	}
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

	// 先创建目标 payload 结构，再把原始 payload 解码进去。
	payload := spec.NewPayload()
	if err := json.Unmarshal(envelope.Payload, payload); err != nil {
		return "", nil, fmt.Errorf("decode payload failed: %w", err)
	}

	// 模板输入固定为：窗口文案 + 已解码的消息体。
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
