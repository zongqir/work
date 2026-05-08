package config

import (
	"context"
	"testing"

	"work/notification/code/dao"
	"work/notification/code/model"
)

type stubTenantMessageConfigStore struct {
	items []model.MessageConfig
	err   error
}

func (s *stubTenantMessageConfigStore) ListTenantMessageConfigs(context.Context, string, dao.MessageConfigQuery) ([]model.MessageConfig, error) {
	return s.items, s.err
}

func (s *stubTenantMessageConfigStore) SaveTenantMessageConfig(context.Context, *model.MessageConfig) error {
	return nil
}

func (s *stubTenantMessageConfigStore) DeleteTenantMessageConfig(context.Context, string, string) error {
	return nil
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
