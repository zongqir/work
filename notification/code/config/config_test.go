package config

import (
	"context"
	"encoding/json"
	"testing"
)

func TestLoadMessageConfig(t *testing.T) {
	cfg, err := LoadMessageConfig(context.Background(), "t_1", "m_1", func(context.Context) (map[string]map[string]json.RawMessage, error) {
		return map[string]map[string]json.RawMessage{
			"t_1": {
				"m_1": json.RawMessage(`{"channels":[{"channel":"sms","template_code":"SMS_001"}]}`),
			},
		}, nil
	})
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
