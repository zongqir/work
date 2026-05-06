package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

func TestBuildMessageRenderInputRejectsNil(t *testing.T) {
	_, err := BuildMessageRenderInput(nil, &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{"k": "v"},
	})
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil request, got %v", err)
	}

	_, err = BuildMessageRenderInput(&contract.BizAggregateRequest{}, nil)
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil result, got %v", err)
	}
}

func TestRenderByPolicyRejectsNil(t *testing.T) {
	req := &contract.BizAggregateRequest{TenantID: "t_1"}
	result := &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{"total_count": "1"},
	}
	policy := &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "xdr_risk_digest",
	}

	_, err := RenderByPolicy(nil, result, policy, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil request, got %v", err)
	}

	_, err = RenderByPolicy(req, nil, policy, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil result, got %v", err)
	}

	_, err = RenderByPolicy(req, result, nil, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil policy, got %v", err)
	}
}

func TestRenderByPolicyRejectsEscapingTemplatePath(t *testing.T) {
	req := &contract.BizAggregateRequest{TenantID: "t_1"}
	result := &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{"total_count": "1"},
	}
	policy := &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "xdr_risk_digest",
		Channels: []ChannelPolicy{
			{
				Channel:      "email",
				TemplateCode: "..\\escape",
			},
		},
	}

	_, err := RenderByPolicy(req, result, policy, filepath.Join("testdata", "templates"))
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for escaping template path, got %v", err)
	}
}

func TestConfigCacheAsyncRefreshUsesTimeout(t *testing.T) {
	done := make(chan error, 1)
	cache := configCache{
		TTL:            5 * time.Minute,
		MaxStale:       30 * time.Minute,
		RefreshTimeout: 20 * time.Millisecond,
		now: func() time.Time {
			return time.Date(2026, 4, 29, 12, 10, 0, 0, time.UTC)
		},
		items: map[string]map[string]json.RawMessage{
			"t_1": {
				"send_test": json.RawMessage(`{"enabled":true}`),
			},
		},
		loadedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
	}

	_, err := cache.pick(
		context.Background(),
		"t_1",
		"send_test",
		func(ctx context.Context) (map[string]map[string]json.RawMessage, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		func(_ context.Context, _ string, err error) {
			done <- err
		},
	)
	if err != nil {
		t.Fatalf("pick failed: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context deadline exceeded, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("expected async refresh timeout to be logged")
	}
}
