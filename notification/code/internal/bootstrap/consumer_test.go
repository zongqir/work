package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"work/notification/code/contract"
	"work/notification/code/internal/delivery"
)

type stubRecorder struct{}

func (stubRecorder) Save(context.Context, *delivery.SendRecord) error {
	return nil
}

func TestNewConsumerRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		cfg    ConsumerConfig
		target string
	}{
		{
			name:   "missing pulsar url",
			cfg:    ConsumerConfig{},
			target: "pulsar url is required",
		},
		{
			name: "missing topic",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
			},
			target: "topic is required",
		},
		{
			name: "missing subscription",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
				Topic:               "topic",
			},
			target: "subscription is required",
		},
		{
			name: "missing load all",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
				Topic:               "topic",
				Subscription:        "sub",
			},
			target: "load_all is required",
		},
		{
			name: "missing template root",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
				Topic:               "topic",
				Subscription:        "sub",
				LoadAll:             stubLoadAll,
			},
			target: "template_root is required",
		},
		{
			name: "missing recorder",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
				Topic:               "topic",
				Subscription:        "sub",
				LoadAll:             stubLoadAll,
				TemplateRoot:        "templates",
			},
			target: "recorder is required",
		},
		{
			name: "missing delivery base url",
			cfg: ConsumerConfig{
				PulsarClientOptions: mustClientOptions("pulsar://127.0.0.1:6650"),
				Topic:               "topic",
				Subscription:        "sub",
				LoadAll:             stubLoadAll,
				TemplateRoot:        "templates",
				Recorder:            stubRecorder{},
			},
			target: "delivery_base_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConsumer(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, contract.ErrInvalidRequest) {
				t.Fatalf("expected invalid request error, got %v", err)
			}
			if err.Error() == contract.ErrInvalidRequest.Error() {
				t.Fatalf("expected contextual error, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.target) {
				t.Fatalf("expected error to contain %q, got %v", tt.target, err)
			}
		})
	}
}

func stubLoadAll(context.Context) (map[string]map[string]json.RawMessage, error) {
	return map[string]map[string]json.RawMessage{}, nil
}

func mustClientOptions(url string) pulsar.ClientOptions {
	return pulsar.ClientOptions{URL: url}
}
