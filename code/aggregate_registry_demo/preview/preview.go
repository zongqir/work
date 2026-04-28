package preview

import (
	"encoding/json"
	"os"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/messages"
)

type Result struct {
	TemplateContext map[string]any                `json:"template_context,omitempty"`
	Rendered        []core.RenderedChannelMessage `json:"rendered"`
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

	input, err := core.BuildMessageRenderInput(req, result)
	if err != nil {
		return nil, err
	}

	rendered, err := core.RenderByPolicy(req, result, policy, templateRoot)
	if err != nil {
		return nil, err
	}

	out := &Result{
		Rendered: rendered,
	}
	if showContext {
		context := make(map[string]any, len(input.Vars))
		for key, value := range input.Vars {
			context[key] = value
		}
		out.TemplateContext = context
	}

	return out, nil
}

func Marshal(result *Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func loadRequest(path string) (*core.BizAggregateRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var req core.BizAggregateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func loadResult(path string) (*messages.BizAggregateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result messages.BizAggregateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadPolicy(path string) (*core.EffectivePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy core.EffectivePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}
