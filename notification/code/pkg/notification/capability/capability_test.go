package capability

import (
	"encoding/json"
	"errors"
	"testing"
	"testing/fstest"
	"time"

	_ "work/notification/code/samples/handlers/sample_aggregate_only"
	_ "work/notification/code/samples/handlers/sample_both"
	_ "work/notification/code/samples/handlers/sample_realtime_only"
	"work/notification/code/pkg/notification/contract"
)

func TestLoadAllUsesMessageTypeFromJSON(t *testing.T) {
	items, err := loadAll(fstest.MapFS{
		"message_capabilities/random_folder/capability.json": {
			Data: []byte(`{
				"message_type": "json_message_type",
				"realtime_supported": true,
				"channels": [{"channel": "email"}]
			}`),
		},
		"message_capabilities/random_folder/schema.json": {
			Data: []byte(`{"type":"object"}`),
		},
	})
	if err != nil {
		t.Fatalf("loadAll failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Capability.MessageType != "json_message_type" {
		t.Fatalf("expected message_type from JSON, got %s", items[0].Capability.MessageType)
	}
}

func TestAllCoversRegisteredMessageTypes(t *testing.T) {
	items, err := All()
	if err != nil {
		t.Fatalf("All failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected capabilities")
	}

	byType := map[string]MessageCapability{}
	for _, item := range items {
		byType[item.MessageType] = item
	}

	for _, messageType := range contract.RegisteredMessageTypes() {
		if _, ok := byType[messageType]; !ok {
			t.Fatalf("missing capability for registered message_type %s", messageType)
		}
	}
}

func TestValidateConfigAcceptsSupportedConfig(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:            "sample_both",
		RealtimeEnabled:        true,
		AggregateEnabled:       true,
		AggregatePeriodMinutes: 60,
		Filter:                 json.RawMessage(`{"severity":["high","critical"],"sample_limit":3}`),
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_both_default",
			Audience: AudienceConfig{
				To: []string{"owner@example.com"},
			},
		},
	})
	if err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}
}

func TestValidateConfigRejectsUnsupportedFilterField(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Filter:          json.RawMessage(`{"tenant_name":"x"}`),
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_both_default",
			Audience:     AudienceConfig{To: []string{"owner@example.com"}},
		},
	})
	if !errors.Is(err, contract.ErrUnsupportedConfig) {
		t.Fatalf("expected unsupported config, got %v", err)
	}
}

func TestValidateConfigRejectsInvalidFilterValue(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Filter:          json.RawMessage(`{"severity":["unknown"]}`),
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_both_default",
			Audience:     AudienceConfig{To: []string{"owner@example.com"}},
		},
	})
	if !errors.Is(err, contract.ErrUnsupportedConfig) {
		t.Fatalf("expected unsupported config, got %v", err)
	}
}

func TestValidateConfigRejectsUnsupportedMode(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:            "sample_realtime_only",
		AggregateEnabled:       true,
		AggregatePeriodMinutes: 60,
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_realtime_only_default",
			Audience:     AudienceConfig{To: []string{"owner@example.com"}},
		},
	})
	if !errors.Is(err, contract.ErrUnsupportedConfig) {
		t.Fatalf("expected unsupported config, got %v", err)
	}
}

func TestValidateConfigRejectsUnsupportedChannel(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Channel: ChannelPolicy{
			Channel:      "sms",
			TemplateCode: "sample_both_default",
		},
	})
	if !errors.Is(err, contract.ErrUnsupportedConfig) {
		t.Fatalf("expected unsupported config, got %v", err)
	}
}

func TestValidateConfigRejectsExternalMetadata(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_both_default",
			Audience:     AudienceConfig{To: []string{"owner@example.com"}},
		},
		UpdatedBy: "user",
		UpdatedAt: time.Now(),
	})
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestValidateConfigRejectsMissingEmailAudience(t *testing.T) {
	err := ValidateConfig(&MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Channel:         ChannelPolicy{Channel: "email", TemplateCode: "sample_both_default"},
	})
	if !errors.Is(err, contract.ErrUnsupportedConfig) {
		t.Fatalf("expected unsupported config, got %v", err)
	}
}
