package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"notes/code/aggregate_registry_demo/contract"
	"notes/code/aggregate_registry_demo/runtime"
)

type Config struct {
	PulsarURL            string
	PulsarClientOptions  *pulsar.ClientOptions
	Topic                string
	LoadAll              func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError             func(ctx context.Context, msg string, err error)
	CacheTTL             time.Duration
	CacheMaxStale        time.Duration
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
}

type Service struct {
	dispatcher *runtime.Dispatcher
	publisher  *runtime.PulsarPublisher
	client     pulsar.Client
}

func New(config Config) (*Service, error) {
	clientOptions := pulsar.ClientOptions{}
	if config.PulsarClientOptions != nil {
		clientOptions = *config.PulsarClientOptions
	}
	if clientOptions.URL == "" {
		clientOptions.URL = config.PulsarURL
	}
	if clientOptions.URL == "" {
		return nil, fmt.Errorf("%w: pulsar url is required", contract.ErrInvalidRequest)
	}

	client, err := pulsar.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}

	publisher, err := runtime.NewPulsarPublisher(client, config.Topic)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &Service{
		dispatcher: runtime.NewDispatcher(runtime.Options{
			Publisher:            publisher,
			LoadAll:              config.LoadAll,
			LogError:             config.LogError,
			CacheTTL:             config.CacheTTL,
			CacheMaxStale:        config.CacheMaxStale,
			RealtimeExpireAfter:  config.RealtimeExpireAfter,
			AggregateExpireAfter: config.AggregateExpireAfter,
		}),
		publisher: publisher,
		client:    client,
	}, nil
}

func NewWithRuntime(options runtime.Options) *runtime.Dispatcher {
	return runtime.NewDispatcher(options)
}

func (s *Service) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	if s == nil || s.dispatcher == nil {
		return fmt.Errorf("%w: service is nil", contract.ErrInvalidRequest)
	}
	return s.dispatcher.SendAggregate(ctx, tenantID, messageType, windowStart, windowEnd)
}

func (s *Service) SendRealtime(ctx context.Context, tenantID, messageType string, event any) error {
	if s == nil || s.dispatcher == nil {
		return fmt.Errorf("%w: service is nil", contract.ErrInvalidRequest)
	}
	return s.dispatcher.SendRealtime(ctx, tenantID, messageType, event)
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	if s.publisher != nil {
		s.publisher.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
}
