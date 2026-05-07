package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	configpkg "work/notification/code/config"
	"work/notification/code/consumer"
	"work/notification/code/contract"
	"work/notification/code/delivery"
	"work/notification/code/delivery/channels"
)

type ConsumerConfig struct {
	PulsarClientOptions pulsar.ClientOptions
	Topic               string
	Subscription        string
	LoadAll             func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	LogError            func(ctx context.Context, msg string, err error)

	TemplateRoot        string
	Recorder            delivery.Recorder
	HTTPClient          *http.Client
	DeliveryBaseURL     string
	RetryDelay          time.Duration
	MaxRetry            int
	CacheTTL            time.Duration
	CacheMaxStale       time.Duration
	CacheRefreshTimeout time.Duration
}

type ConsumerService struct {
	processor *delivery.Processor
	consumer  *consumer.PulsarConsumer
	client    pulsar.Client
}

func NewConsumer(config ConsumerConfig) (*ConsumerService, error) {
	if config.PulsarClientOptions.URL == "" {
		return nil, fmt.Errorf("%w: pulsar url is required", contract.ErrInvalidRequest)
	}
	if config.Topic == "" {
		return nil, fmt.Errorf("%w: topic is required", contract.ErrInvalidRequest)
	}
	if config.Subscription == "" {
		return nil, fmt.Errorf("%w: subscription is required", contract.ErrInvalidRequest)
	}
	if config.LoadAll == nil {
		return nil, fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}
	if config.TemplateRoot == "" {
		return nil, fmt.Errorf("%w: template_root is required", contract.ErrInvalidRequest)
	}
	if config.Recorder == nil {
		return nil, fmt.Errorf("%w: recorder is required", contract.ErrInvalidRequest)
	}
	if config.DeliveryBaseURL == "" {
		return nil, fmt.Errorf("%w: delivery_base_url is required", contract.ErrInvalidRequest)
	}

	client, err := pulsar.NewClient(config.PulsarClientOptions)
	if err != nil {
		return nil, err
	}

	cache := &configpkg.Cache{
		TTL:            config.CacheTTL,
		MaxStale:       config.CacheMaxStale,
		RefreshTimeout: config.CacheRefreshTimeout,
	}

	processor := &delivery.Processor{
		LoadConfig: func(ctx context.Context, tenantID, messageType string) (*configpkg.MessageConfig, error) {
			raw, err := cache.Pick(ctx, tenantID, messageType, config.LoadAll, config.LogError)
			if err != nil {
				return nil, err
			}
			if len(raw) == 0 {
				return nil, nil
			}

			var cfg configpkg.MessageConfig
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		},
		TemplateRoot: config.TemplateRoot,
		Senders:      channels.NewSenders(config.HTTPClient, config.DeliveryBaseURL),
		Recorder:     config.Recorder,
		RetryDelay:   config.RetryDelay,
		MaxRetry:     config.MaxRetry,
	}

	rawConsumer, err := consumer.NewPulsarConsumer(client, config.Topic, config.Subscription, processor)
	if err != nil {
		client.Close()
		return nil, err
	}
	rawConsumer.LogError = config.LogError

	return &ConsumerService{
		processor: processor,
		consumer:  rawConsumer,
		client:    client,
	}, nil
}

func (s *ConsumerService) Run(ctx context.Context) error {
	if s == nil || s.consumer == nil {
		return fmt.Errorf("%w: consumer service is nil", contract.ErrInvalidRequest)
	}
	return s.consumer.Run(ctx)
}

func (s *ConsumerService) Processor() *delivery.Processor {
	if s == nil {
		return nil
	}
	return s.processor
}

func (s *ConsumerService) Close() {
	if s == nil {
		return
	}
	if s.consumer != nil {
		s.consumer.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
}
