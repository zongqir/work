package config

import (
	"context"
	"encoding/json"
	"testing"

	"work/notification/code/dao"
	"work/notification/code/render"
)

type stubTenantMessageConfigStore struct {
	item *dao.TenantMessageConfig
	err  error
}

func (s *stubTenantMessageConfigStore) ListTenantMessageConfigs(context.Context, string, dao.MessageConfigQuery) ([]dao.TenantMessageConfig, error) {
	return nil, nil
}

func (s *stubTenantMessageConfigStore) GetTenantMessageConfig(context.Context, string, string) (*dao.TenantMessageConfig, error) {
	return s.item, s.err
}

func (s *stubTenantMessageConfigStore) SaveTenantMessageConfig(context.Context, *dao.TenantMessageConfig) error {
	return nil
}

func (s *stubTenantMessageConfigStore) DeleteTenantMessageConfig(context.Context, string, string) error {
	return nil
}

func TestLoadEffectiveMessageConfigFallsBackToDefault(t *testing.T) {
	store := &stubTenantMessageConfigStore{err: dao.ErrNotFound}
	cfg, err := LoadEffectiveMessageConfig(context.Background(), "t_1", "m_1", func(context.Context, string) (json.RawMessage, error) {
		return json.RawMessage(`{"channel":{"channel":"sms","template_code":"base"}}`), nil
	}, store)
	if err != nil {
		t.Fatalf("LoadEffectiveMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Channel.Channel != "sms" || cfg.Channel.TemplateCode != "base" {
		t.Fatalf("unexpected default config: %+v", cfg.Channel)
	}
}

func TestLoadEffectiveMessageConfigUsesTenantRecordDirectly(t *testing.T) {
	store := &stubTenantMessageConfigStore{
		item: &dao.TenantMessageConfig{
			RealtimeEnabled:        false,
			AggregateEnabled:       false,
			AggregatePeriodMinutes: 15,
			Channel: render.ChannelPolicy{
				Channel:      "email",
				TemplateCode: "tenant",
			},
		},
	}
	cfg, err := LoadEffectiveMessageConfig(context.Background(), "t_1", "m_1", func(context.Context, string) (json.RawMessage, error) {
		return json.RawMessage(`{"realtime_enabled":true,"aggregate_enabled":true,"aggregate_period_minutes":30,"channel":{"channel":"sms","template_code":"base"}}`), nil
	}, store)
	if err != nil {
		t.Fatalf("LoadEffectiveMessageConfig failed: %v", err)
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
