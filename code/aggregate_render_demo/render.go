package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func loadResult(path string) (*AggregateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result AggregateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func buildRenderView(req *AggregateRequest, result *AggregateResult) (string, any, error) {
	if result.MessageType == "" {
		return "", nil, fmt.Errorf("message_type is required")
	}

	rv := reflect.ValueOf(result)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return "", nil, fmt.Errorf("result must be a non-nil pointer")
	}

	elem := rv.Elem()
	typ := elem.Type()

	activeCount := 0
	activeType := ""
	var activePayload any

	for i := 0; i < elem.NumField(); i++ {
		fieldType := typ.Field(i)
		if fieldType.Name == "MessageType" {
			continue
		}

		tag := strings.Split(fieldType.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}

		fieldValue := elem.Field(i)
		if fieldValue.Kind() != reflect.Ptr {
			continue
		}
		if fieldValue.IsNil() {
			continue
		}

		activeCount++
		activeType = tag
		activePayload = fieldValue.Interface()
	}

	if activeCount != 1 {
		return "", nil, fmt.Errorf("exactly one message payload must be set")
	}

	if activeType != result.MessageType {
		return "", nil, fmt.Errorf("message_type=%s but active payload is %s", result.MessageType, activeType)
	}

	return activeType, RenderView{
		WindowLabel: formatWindowLabel(req.WindowStart, req.WindowEnd),
		Payload:     activePayload,
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
