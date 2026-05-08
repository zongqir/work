package config

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"work/notification/code/internal/render"
	"work/notification/code/model"
)

type stubTenantMessageConfigStore struct {
	items []model.MessageConfig
	err   error
}

func (s *stubTenantMessageConfigStore) ListTenantMessageConfigs(context.Context, string) ([]model.MessageConfig, error) {
	return s.items, s.err
}

func (s *stubTenantMessageConfigStore) SaveTenantMessageConfig(context.Context, *model.MessageConfig) error {
	return nil
}

func (s *stubTenantMessageConfigStore) DeleteTenantMessageConfig(context.Context, string, string) error {
	return nil
}

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

func TestLoadDefaultMessageConfigsFromFile(t *testing.T) {
	items, err := LoadDefaultMessageConfigsFromFile(filepath.Join("..", "..", "default_message_configs.json"))
	if err != nil {
		t.Fatalf("LoadDefaultMessageConfigsFromFile failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected default configs")
	}

	var found bool
	for _, item := range items {
		if item.MessageType == "sample_both" {
			found = true
			if item.Channel.Channel != "email" {
				t.Fatalf("unexpected channel: %+v", item.Channel)
			}
		}
	}
	if !found {
		t.Fatal("expected sample_both default config")
	}
}

func TestMessageConfigLoaderFallsBackToDefault(t *testing.T) {
	loader := &MessageConfigLoader{
		Store: &stubTenantMessageConfigStore{},
		Defaults: []model.MessageConfig{
			{
				MessageType: "m_1",
				Channel: model.ChannelPolicy{
					Channel:      "sms",
					TemplateCode: "base",
				},
			},
		},
	}
	cfg, err := loader.Load(context.Background(), "t_1", "m_1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Channel.Channel != "sms" || cfg.Channel.TemplateCode != "base" {
		t.Fatalf("unexpected default config: %+v", cfg.Channel)
	}
}

func TestMessageConfigLoaderUsesTenantRecordDirectly(t *testing.T) {
	loader := &MessageConfigLoader{
		Store: &stubTenantMessageConfigStore{
			items: []model.MessageConfig{
				{
					MessageType:            "m_1",
					RealtimeEnabled:        false,
					AggregateEnabled:       false,
					AggregatePeriodMinutes: 15,
					Channel: model.ChannelPolicy{
						Channel:      "email",
						TemplateCode: "tenant",
					},
				},
			},
		},
		Defaults: []model.MessageConfig{
			{
				MessageType:            "m_1",
				RealtimeEnabled:        true,
				AggregateEnabled:       true,
				AggregatePeriodMinutes: 30,
				Channel: model.ChannelPolicy{
					Channel:      "sms",
					TemplateCode: "base",
				},
			},
		},
	}
	cfg, err := loader.Load(context.Background(), "t_1", "m_1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.RealtimeEnabled {
		t.Fatal("expected realtime_enabled from tenant record")
	}
	if cfg.AggregateEnabled {
		t.Fatal("expected aggregate_enabled from tenant record")
	}
	if cfg.AggregatePeriodMinutes != 15 {
		t.Fatalf("unexpected aggregate period: %d", cfg.AggregatePeriodMinutes)
	}
	if cfg.Channel.Channel != "email" || cfg.Channel.TemplateCode != "tenant" {
		t.Fatalf("unexpected tenant channel: %+v", cfg.Channel)
	}
}
