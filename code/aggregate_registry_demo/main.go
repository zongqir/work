package main

import "fmt"

func main() {
	req, err := loadBizAggregateRequest("code/aggregate_registry_demo/sample_request.json")
	if err != nil {
		panic(err)
	}

	result, err := loadBizAggregateResult("code/aggregate_registry_demo/sample_result.json")
	if err != nil {
		panic(err)
	}

	templateCode, input, err := buildMessageRenderInput(req, result)
	if err != nil {
		panic(err)
	}

	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderMessage(templateCode, input, channel, "code/aggregate_registry_demo/templates")
		if err != nil {
			panic(err)
		}
		fmt.Printf("[%s]\n%s\n\n", channel, rendered)
	}
}
