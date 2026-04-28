package main

import "fmt"

func main() {
	cfg, err := loadConfig("code/aggregate_render_demo/sample_config.json")
	if err != nil {
		panic(err)
	}

	result, err := loadResult("code/aggregate_render_demo/sample_result.json")
	if err != nil {
		panic(err)
	}

	templateCode, view, err := buildRenderView(cfg, result)
	if err != nil {
		panic(err)
	}

	for _, channel := range []string{"email", "wecom", "sms"} {
		rendered, err := renderSummary(
			templateCode,
			view,
			channel,
			"code/aggregate_render_demo/templates",
		)
		if err != nil {
			panic(err)
		}
		fmt.Printf("[%s]\n%s\n\n", channel, rendered)
	}
}
