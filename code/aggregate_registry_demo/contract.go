package aggregate

import (
	"context"
	"errors"

	"notes/code/aggregate_registry_demo/messages"
)

var (
	ErrInvalidRequest    = errors.New("invalid aggregate request")
	ErrUnsupportedConfig = errors.New("unsupported aggregate config")
	ErrTemporaryFailure  = errors.New("temporary aggregate failure")
	ErrAggregatorNotFound = errors.New("aggregator not found")
)

// Aggregator 是业务侧需要实现的最小生产契约。
// 实现方自己定义 message_type，自己注册自己，自己完成聚合并返回结果。
type Aggregator interface {
	MessageType() string
	MustRegister()
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error)
}
