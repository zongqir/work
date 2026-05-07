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

func TestParseMessageConfigWithSourceChannel(t *testing.T) {
	cfg, err := ParseMessageConfig(json.RawMessage(`{
		"realtime_channel": {"channel":"sms","template_key":"SMS_REALTIME"},
		"aggregate_channel": {"channel":"email","template_code":"sample_both_aggregate"},
		"channel": {"channel":"webhook","template_code":"sample_both_fallback"}
	}`))
	if err != nil {
		t.Fatalf("ParseMessageConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.RealtimeChannel.TemplateKey != "SMS_REALTIME" {
		t.Fatalf("unexpected realtime_channel: %+v", cfg.RealtimeChannel)
	}
	if cfg.AggregateChannel.TemplateCode != "sample_both_aggregate" {
		t.Fatalf("unexpected aggregate_channel: %+v", cfg.AggregateChannel)
	}
	if cfg.Channel.TemplateCode != "sample_both_fallback" {
		t.Fatalf("unexpected fallback channel: %+v", cfg.Channel)
	}
}

func TestMessageConfigChannelForSource(t *testing.T) {
	t.Run("use source specific channel", func(t *testing.T) {
		cfg := &MessageConfig{
			RealtimeChannel:  render.ChannelPolicy{Channel: "sms"},
			AggregateChannel: render.ChannelPolicy{Channel: "email"},
			Channel:          render.ChannelPolicy{Channel: "webhook"},
		}

		realtimeChannel, ok := cfg.ChannelForSource(contract.DispatchSourceRealtime)
		if !ok || realtimeChannel.Channel != "sms" {
			t.Fatalf("unexpected realtime channel: %+v", realtimeChannel)
		}

		aggregateChannel, ok := cfg.ChannelForSource(contract.DispatchSourceAggregate)
		if !ok || aggregateChannel.Channel != "email" {
			t.Fatalf("unexpected aggregate channel: %+v", aggregateChannel)
		}
	})

	t.Run("fallback to generic channel", func(t *testing.T) {
		cfg := &MessageConfig{
			Channel: render.ChannelPolicy{Channel: "webhook"},
		}

		realtimeChannel, ok := cfg.ChannelForSource(contract.DispatchSourceRealtime)
		if !ok || realtimeChannel.Channel != "webhook" {
			t.Fatalf("unexpected realtime fallback channel: %+v", realtimeChannel)
		}

		aggregateChannel, ok := cfg.ChannelForSource(contract.DispatchSourceAggregate)
		if !ok || aggregateChannel.Channel != "webhook" {
			t.Fatalf("unexpected aggregate fallback channel: %+v", aggregateChannel)
		}
	})
}
