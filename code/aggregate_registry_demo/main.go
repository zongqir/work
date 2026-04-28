package main

import "fmt"

func main() {
	req, err := loadRequest("code/aggregate_registry_demo/sample_request.json")
	if err != nil {
		panic(err)
	}

	envelope, err := loadEnvelope("code/aggregate_registry_demo/sample_result.json")
	if err != nil {
		panic(err)
	}

	templateCode, view, err := buildRenderView(req, envelope)
	if err != nil {
		panic(err)
	}

	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderSummary(templateCode, view, channel, "code/aggregate_registry_demo/templates")
		if err != nil {
			panic(err)
		}
		fmt.Printf("[%s]\n%s\n\n", channel, rendered)
	}
}
