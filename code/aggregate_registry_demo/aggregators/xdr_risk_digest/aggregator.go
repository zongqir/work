package xdrriskdigest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "notes/code/aggregate_registry_demo"
	"notes/code/aggregate_registry_demo/aggregators"
	"notes/code/aggregate_registry_demo/messages"
)

const MessageType = "xdr_risk_digest"

// Config 只是演示业务方如何解释 config_body。
type Config struct {
	Severity    []string `json:"severity"`
	SampleLimit int      `json:"sample_limit"`
	GroupBy     string   `json:"group_by"`
}

// Aggregator 是给业务方看的最小实现样板。
// 重点是实现 AES 定义的接口，并返回 message_type + biz_vars。
type Aggregator struct{}

var _ core.Aggregator = (*Aggregator)(nil)
var _ core.TypedAggregator = (*Aggregator)(nil)

func init() {
	aggregators.MustRegister(&Aggregator{})
}

func (a *Aggregator) MessageType() string {
	return MessageType
}

func (a *Aggregator) Aggregate(_ context.Context, req *core.BizAggregateRequest) (*messages.BizAggregateResult, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is nil", core.ErrInvalidRequest)
	}
	if strings.TrimSpace(req.TenantID) == "" {
		return nil, fmt.Errorf("%w: tenant_id is required", core.ErrInvalidRequest)
	}
	if !req.WindowEnd.After(req.WindowStart) {
		return nil, fmt.Errorf("%w: invalid window", core.ErrInvalidRequest)
	}

	cfg, err := parseConfig(req.ConfigBody)
	if err != nil {
		return nil, err
	}

	// 业务实现方在这里完成自己的查询和计算，最后只需要填好 biz_vars 返回即可。
	examples := []map[string]string{
		{
			"object_name": "host-a",
			"risk_type":   "暴力破解",
			"event_count": "6",
		},
		{
			"object_name": "user-b",
			"risk_type":   "恶意登录",
			"event_count": "4",
		},
		{
			"object_name": "host-c",
			"risk_type":   "权限提升",
			"event_count": "3",
		},
	}
	if cfg.SampleLimit > 0 && cfg.SampleLimit < len(examples) {
		examples = examples[:cfg.SampleLimit]
	}

	return &messages.BizAggregateResult{
		MessageType: MessageType,
		BizVars: messages.TemplateVars{
			"total_count":    "23",
			"category_count": "3",
			"examples":       examples,
		},
	}, nil
}

func parseConfig(raw json.RawMessage) (*Config, error) {
	cfg := &Config{
		SampleLimit: 3,
		GroupBy:     "policy",
	}
	if len(raw) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%w: decode config_body failed: %v", core.ErrInvalidRequest, err)
	}
	if cfg.SampleLimit <= 0 {
		cfg.SampleLimit = 3
	}
	if strings.TrimSpace(cfg.GroupBy) == "" {
		cfg.GroupBy = "policy"
	}
	return cfg, nil
}
