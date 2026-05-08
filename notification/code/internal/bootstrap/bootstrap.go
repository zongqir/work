package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"work/notification/code/contract"
	"work/notification/code/dispatch"
	"work/notification/code/handlers"
	"work/notification/code/internal/publisher"
)

type Config struct {
	PulsarClientOptions  pulsar.ClientOptions
	Topic                string
	LoadAll              func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError             func(ctx context.Context, msg string, err error)
	CacheTTL             time.Duration
	CacheMaxStale        time.Duration
	RealtimeExpireAfter  time.Duration
	AggregateExpireAfter time.Duration
}

type Service struct {
	dispatcher *dispatch.Dispatcher
	publisher  *publisher.PulsarPublisher
	client     pulsar.Client
}

func New(config Config) (*Service, error) {
	handlers.Register()

	if config.PulsarClientOptions.URL == "" {
		return nil, fmt.Errorf("%w: pulsar url is required", contract.ErrInvalidRequest)
	}

	client, err := pulsar.NewClient(config.PulsarClientOptions)
	if err != nil {
		return nil, err
	}

	publisher, err := publisher.NewPulsarPublisher(client, config.Topic)
	if err != nil {
		client.Close()
		return nil, err
	}

	return &Service{
		dispatcher: &dispatch.Dispatcher{
			Publisher:            publisher,
			LoadAll:              config.LoadAll,
			LogError:             config.LogError,
			CacheTTL:             config.CacheTTL,
			CacheMaxStale:        config.CacheMaxStale,
			RealtimeExpireAfter:  config.RealtimeExpireAfter,
			AggregateExpireAfter: config.AggregateExpireAfter,
		},
		publisher: publisher,
		client:    client,
	}, nil
}

func (s *Service) SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error {
	if s == nil || s.dispatcher == nil {
		return fmt.Errorf("%w: service is nil", contract.ErrInvalidRequest)
	}
	_, err := s.dispatcher.SendAggregate(ctx, tenantID, messageType, windowStart, windowEnd)
	return err
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
