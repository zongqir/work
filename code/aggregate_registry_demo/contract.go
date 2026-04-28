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
)

// Aggregator 是业务侧需要实现的生产级聚合接口。
// AES 只关心请求和结果，不关心业务方内部如何查库或编排聚合逻辑。
type Aggregator interface {
	Aggregate(ctx context.Context, req *BizAggregateRequest) (*messages.BizAggregateResult, error)
}
