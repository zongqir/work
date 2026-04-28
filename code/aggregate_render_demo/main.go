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

type RenderConfig struct {
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

type AggregateResult struct {
	MessageType   string         `json:"message_type"`
	XdrRiskDigest *XdrRiskDigest `json:"xdr_risk_digest,omitempty"`
}

type XdrRiskDigest struct {
	TotalCount    int              `json:"total_count"`
	CategoryCount int              `json:"category_count"`
	Examples      []XdrRiskExample `json:"examples"`
}

type XdrRiskExample struct {
	ObjectName string `json:"object_name"`
	RiskType   string `json:"risk_type"`
	EventCount int    `json:"event_count"`
}

type RenderView struct {
	WindowLabel string
	Payload     any
}

func main() {
	cfg, err := loadConfig("code/aggregate_render_demo/sample_config.json")
	if err != nil {
		panic(err)
	}

	result, err := loadResult("code/aggregate_render_demo/sample_result.json")
	if err != nil {
		panic(err)
	}

	templateCode, view, err := buildRenderView(cfg, result)
	if err != nil {
		panic(err)
	}

	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderSummary(
			templateCode,
			view,
			channel,
			"code/aggregate_render_demo/templates",
		)
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

func buildRenderView(cfg *RenderConfig, result *AggregateResult) (string, any, error) {
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
		WindowLabel: formatWindowLabel(cfg.WindowStart, cfg.WindowEnd),
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
