package render

import (
	"errors"
	"path/filepath"
	"testing"

	"work/notification/code/contract"
)

func TestBuildTemplateContextAllowsEmptyBizVars(t *testing.T) {
	context, err := BuildTemplateContext(RenderInput{})
	if err != nil {
		t.Fatalf("BuildTemplateContext failed: %v", err)
	}
	if context["biz"] == nil {
		t.Fatal("expected empty biz vars map")
	}
	if len(context["biz"]) != 0 {
		t.Fatalf("expected empty biz vars, got %v", context["biz"])
	}
}

func TestRenderRejectsInvalidInput(t *testing.T) {
	policy := &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "sample_both",
	}

	_, err := Render(RenderInput{}, policy, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for empty input, got %v", err)
	}

	_, err = Render(RenderInput{
		TenantID:    "t_1",
		MessageType: "sample_both",
		BizVars:     contract.TemplateVars{"k": "v"},
	}, nil, ".")
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for nil policy, got %v", err)
	}
}

func TestRenderRejectsEscapingTemplatePath(t *testing.T) {
	input := RenderInput{
		TenantID:    "t_1",
		MessageType: "sample_both",
		BizVars:     contract.TemplateVars{"total_count": "1"},
	}
	policy := &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "sample_both",
		Channel: ChannelPolicy{
			Channel:      "email",
			TemplateCode: "..\\escape",
		},
	}

	_, err := Render(input, policy, filepath.Join("testdata", "templates"))
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for escaping template path, got %v", err)
	}
}

func TestRenderRejectsPolicyMismatch(t *testing.T) {
	input := RenderInput{
		TenantID:    "t_1",
		MessageType: "sample_both",
		BizVars:     contract.TemplateVars{"k": "v"},
	}

	_, err := Render(input, &EffectivePolicy{
		TenantID:    "t_2",
		MessageType: "sample_both",
	}, ".")
	if err == nil {
		t.Fatal("expected tenant mismatch")
	}

	_, err = Render(input, &EffectivePolicy{
		TenantID:    "t_1",
		MessageType: "other",
	}, ".")
	if err == nil {
		t.Fatal("expected message type mismatch")
	}
}
