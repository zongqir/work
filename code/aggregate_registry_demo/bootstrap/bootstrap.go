package bootstrap

import (
	"context"
	"encoding/json"
	"time"

	"notes/code/aggregate_registry_demo/runtime"
)

type Options struct {
	Publisher     runtime.MessagePublisher
	LoadAll       func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError      func(ctx context.Context, msg string, err error)
	CacheTTL      time.Duration
	CacheMaxStale time.Duration
}

func New(options Options) *runtime.Dispatcher {
	return runtime.NewDispatcher(runtime.Options{
		Publisher:     options.Publisher,
		LoadAll:       options.LoadAll,
		LogError:      options.LogError,
		CacheTTL:      options.CacheTTL,
		CacheMaxStale: options.CacheMaxStale,
	})
}
