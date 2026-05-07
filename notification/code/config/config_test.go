package config

import (
	"context"
	"encoding/json"
	"testing"

	"work/notification/code/render"
)

func TestLoadMessageConfig(t *testing.T) {
	cfg, err := LoadMessageConfig(context.Background(), "t_1", "m_1", nil, func(context.Context) (map[string]map[string]json.RawMessage, error) {
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"m_1": json.RawMessage(`{"channel":{"channel":"sms","template_code":"SMS_001"}}`),
			},
		}, nil
	}, nil)
	if err != nil {
		t.Fatalf("LoadMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Channel.Channel != "sms" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseMessageConfigWithChannel(t *testing.T) {
	cfg, err := ParseMessageConfig(json.RawMessage(`{
		"channel": {"channel":"webhook","template_code":"sample_both"}
	}`))
	if err != nil {
		t.Fatalf("ParseMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Channel.TemplateCode != "sample_both" {
		t.Fatalf("unexpected channel: %+v", cfg.Channel)
	}
}

func TestMessageConfigEffectiveChannel(t *testing.T) {
	cfg := &MessageConfig{
		Channel: render.ChannelPolicy{Channel: "webhook"},
	}

	channel, ok := cfg.EffectiveChannel()
	if !ok || channel.Channel != "webhook" {
		t.Fatalf("unexpected channel: %+v", channel)
	}

	empty := &MessageConfig{}
	if channel, ok := empty.EffectiveChannel(); ok {
		t.Fatalf("expected no channel, got %+v", channel)
	}
}
