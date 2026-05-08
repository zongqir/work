package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDefaultMessageConfigsFromFile(t *testing.T) {
	items, err := LoadDefaultMessageConfigsFromFile(filepath.Join("..", "default_message_configs.json"))
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
