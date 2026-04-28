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
	TemplateCode string    `json:"template_code"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
}

type AggregateResult struct {
	Data map[string]interface{} `json:"data"`
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

	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderSummary(
			cfg,
			result,
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

func renderSummary(cfg *RenderConfig, result *AggregateResult, channel, templateRoot string) (string, error) {
	templatePath := filepath.Join(templateRoot, cfg.TemplateCode, channel+".tmpl")
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
	if err := tmpl.Execute(&buf, buildTemplateArgs(cfg, result)); err != nil {
		return "", fmt.Errorf("render template failed: %w", err)
	}

	return buf.String(), nil
}

func buildTemplateArgs(cfg *RenderConfig, result *AggregateResult) map[string]interface{} {
	args := make(map[string]interface{}, len(result.Data)+1)
	for k, v := range result.Data {
		args[k] = v
	}
	args["window_label"] = formatWindowLabel(cfg.WindowStart, cfg.WindowEnd)
	return args
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
