package render

import (
	"errors"
	"path/filepath"
	"testing"

	"work/notification/code/contract"
)

func TestBuildTemplateContextRejectsNil(t *testing.T) {
	_, err := BuildTemplateContext(nil, &contract.BizAggregateResult{
		BizVars: contract.TemplateVars{"k": "v"},
	})
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil request, got %v", err)
	}

	_, err = BuildTemplateContext(&contract.BizAggregateRequest{}, nil)
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

func TestRenderDispatchRejectsNil(t *testing.T) {
	policy := &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "xdr_risk_digest",
	}

	_, err := RenderDispatch(nil, policy, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil message, got %v", err)
	}

	_, err = RenderDispatch(&contract.DispatchMessage{
		TenantID:    "t_1",
		MessageType: "xdr_risk_digest",
		BizVars:     contract.TemplateVars{"k": "v"},
	}, nil, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil policy, got %v", err)
	}
}
