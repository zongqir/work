package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	req, err := loadBizAggregateRequest("sample_request.json")
	if err != nil {
		panic(err)
	}

	result, err := loadBizAggregateResult("sample_result.json")
	if err != nil {
		panic(err)
	}

	policy, err := loadEffectivePolicy("sample_policy.json")
	if err != nil {
		panic(err)
	}

	renderedMessages, err := renderByPolicy(req, result, policy, "templates")
	if err != nil {
		panic(err)
	}

	for _, rendered := range renderedMessages {
		data, err := json.MarshalIndent(rendered, "", "  ")
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n\n", data)
	}
}
