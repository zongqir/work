package preview

import (
	"encoding/json"
	"os"

	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/messages"
	"notes/code/aggregate_registry_demo/runtime"
)

type Result struct {
	TemplateContext map[string]any                   `json:"template_context,omitempty"`
	Rendered        []runtime.RenderedChannelMessage `json:"rendered"`
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

	input, err := runtime.BuildMessageRenderInput(req, result)
	if err != nil {
		return nil, err
	}

	rendered, err := runtime.RenderByPolicy(req, result, policy, templateRoot)
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

func loadPolicy(path string) (*runtime.EffectivePolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy runtime.EffectivePolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}
