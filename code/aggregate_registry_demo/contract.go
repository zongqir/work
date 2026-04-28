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

// Aggregator 是业务侧需要实现的生产级聚合接口。
// AES 只关心请求和结果，不关心业务方内部如何查库或编排聚合逻辑。
type Aggregator interface {
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error)
}

// TypedAggregator 在聚合接口之上补充稳定的 message_type 标识。
// 这样每个实现可以通过 init 自注册，而不用在外部重复填写 message_type。
type TypedAggregator interface {
	Aggregator
	MessageType() string
}
