package config

import (
	"context"
	"encoding/json"
	"testing"

	"work/notification/code/contract"
	"work/notification/code/render"
)

func TestLoadMessageConfig(t *testing.T) {
	cfg, err := LoadMessageConfig(context.Background(), "t_1", "m_1", nil, func(context.Context) (map[string]map[string]json.RawMessage, error) {
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"m_1": json.RawMessage(`{"channels":[{"channel":"sms","template_code":"SMS_001"}]}`),
			},
		}, nil
	}, nil)
	if err != nil {
		t.Fatalf("LoadMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if len(cfg.Channels) != 1 || cfg.Channels[0].Channel != "sms" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseMessageConfigWithSourceChannels(t *testing.T) {
	cfg, err := ParseMessageConfig(json.RawMessage(`{
		"realtime_channels": [{"channel":"sms","template_key":"SMS_REALTIME"}],
		"aggregate_channels": [{"channel":"email","template_code":"sample_both_aggregate"}],
		"channels": [{"channel":"webhook","template_code":"sample_both_fallback"}]
	}`))
	if err != nil {
		t.Fatalf("ParseMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if len(cfg.RealtimeChannels) != 1 || cfg.RealtimeChannels[0].TemplateKey != "SMS_REALTIME" {
		t.Fatalf("unexpected realtime_channels: %+v", cfg.RealtimeChannels)
	}
	if len(cfg.AggregateChannels) != 1 || cfg.AggregateChannels[0].TemplateCode != "sample_both_aggregate" {
		t.Fatalf("unexpected aggregate_channels: %+v", cfg.AggregateChannels)
	}
	if len(cfg.Channels) != 1 || cfg.Channels[0].TemplateCode != "sample_both_fallback" {
		t.Fatalf("unexpected fallback channels: %+v", cfg.Channels)
	}
}

func TestMessageConfigChannelsForSource(t *testing.T) {
	t.Run("use source specific channels", func(t *testing.T) {
		cfg := &MessageConfig{
			RealtimeChannels: []render.ChannelPolicy{{Channel: "sms"}},
			AggregateChannels: []render.ChannelPolicy{{
				Channel: "email",
			}},
			Channels: []render.ChannelPolicy{{Channel: "webhook"}},
		}

		realtimeChannels := cfg.ChannelsForSource(contract.DispatchSourceRealtime)
		if len(realtimeChannels) != 1 || realtimeChannels[0].Channel != "sms" {
			t.Fatalf("unexpected realtime channels: %+v", realtimeChannels)
		}

		aggregateChannels := cfg.ChannelsForSource(contract.DispatchSourceAggregate)
		if len(aggregateChannels) != 1 || aggregateChannels[0].Channel != "email" {
			t.Fatalf("unexpected aggregate channels: %+v", aggregateChannels)
		}
	})

	t.Run("fallback to generic channels", func(t *testing.T) {
		cfg := &MessageConfig{
			Channels: []render.ChannelPolicy{{Channel: "webhook"}},
		}

		realtimeChannels := cfg.ChannelsForSource(contract.DispatchSourceRealtime)
		if len(realtimeChannels) != 1 || realtimeChannels[0].Channel != "webhook" {
			t.Fatalf("unexpected realtime fallback channels: %+v", realtimeChannels)
		}

		aggregateChannels := cfg.ChannelsForSource(contract.DispatchSourceAggregate)
		if len(aggregateChannels) != 1 || aggregateChannels[0].Channel != "webhook" {
			t.Fatalf("unexpected aggregate fallback channels: %+v", aggregateChannels)
		}
	})
}
