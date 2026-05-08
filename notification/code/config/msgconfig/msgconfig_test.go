package msgconfig

import (
	"context"
	"testing"

	"work/notification/code/config"
	"work/notification/code/internal/model"
)

func TestLoaderLoadView(t *testing.T) {
	loader := &Loader{
		ConfigLoader: &config.MessageConfigLoader{
			Store: &stubTenantMessageConfigStore{
				items: []model.MessageConfig{
					{
						MessageType: "sample_both",
						Channel: model.ChannelPolicy{
							Channel:      "email",
							TemplateCode: "tenant",
							Audience: model.AudienceConfig{
								To: []string{"owner@example.com"},
							},
						},
					},
				},
			},
		},
	}

	view, err := loader.LoadView(context.Background(), "t_1", "sample_both")
	if err != nil {
		t.Fatalf("LoadView failed: %v", err)
	}
	if view == nil {
		t.Fatal("expected view")
	}
	if view.Config.TenantID != "t_1" {
		t.Fatalf("unexpected tenant_id: %s", view.Config.TenantID)
	}
	if view.Capability.MessageType != "sample_both" {
		t.Fatalf("unexpected capability: %+v", view.Capability)
	}
}

func TestValidateDelegatesToCapability(t *testing.T) {
	if err := Validate(&model.MessageConfig{
		MessageType:     "sample_both",
		RealtimeEnabled: true,
		Channel: model.ChannelPolicy{
			Channel:      "email",
			TemplateCode: "sample_both_default",
			Audience:     model.AudienceConfig{To: []string{"owner@example.com"}},
		},
	}); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

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
