package config

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLoadDefaultMessageConfigsFromFile(t *testing.T) {
	loader, err := LoadDefaultMessageConfigsFromFile(filepath.Join("..", "default_message_configs.json"))
	if err != nil {
		t.Fatalf("LoadDefaultMessageConfigsFromFile failed: %v", err)
	}

	raw, err := loader(context.Background(), "sample_both")
	if err != nil {
		t.Fatalf("loader failed: %v", err)
	}

	var cfg MessageConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if cfg.Channel.Channel != "email" {
		t.Fatalf("unexpected channel: %+v", cfg.Channel)
	}
}
