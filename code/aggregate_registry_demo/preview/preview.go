package preview

import (
	"encoding/json"
	"os"

	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/render"
)

type Result struct {
	TemplateContext map[string]contract.TemplateVars `json:"template_context,omitempty"`
	Rendered        []render.RenderedChannelMessage  `json:"rendered"`
}

func FromFiles(requestPath, resultPath, policyPath, templateRoot string, showContext bool) (*Result, error) {
	req, err := loadRequest(requestPath)
	if err != nil {
		return nil, err
	}

	result, err := loadResult(resultPath)
	if err != nil {
		return nil, err
	}

	policy, err := loadPolicy(policyPath)
	if err != nil {
		return nil, err
	}

	templateContext, err := render.BuildTemplateContext(req, result)
	if err != nil {
		return nil, err
	}

	rendered, err := render.RenderByPolicy(req, result, policy, templateRoot)
	if err != nil {
		return nil, err
	}

	out := &Result{
		Rendered: rendered,
	}
	if showContext {
		out.TemplateContext = templateContext
	}

	return out, nil
}

func loadRequest(path string) (*contract.BizAggregateRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var req contract.BizAggregateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func loadResult(path string) (*contract.BizAggregateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result contract.BizAggregateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadPolicy(path string) (*render.EffectivePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy render.EffectivePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}
