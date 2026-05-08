package notificationd

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"work/notification/code/internal/bootstrap"
	"work/notification/code/internal/dao"
	"work/notification/code/pkg/notification/contract"
)

type stubWatermarkStore struct{}

func (stubWatermarkStore) LastWindowEnd(context.Context, string, string) (time.Time, error) {
	return time.Time{}, nil
}

func (stubWatermarkStore) SaveWindowEnd(context.Context, string, string, time.Time) error {
	return nil
}

func TestNewRejectsEmptyConfig(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestNewRejectsSchedulerWithoutDispatcher(t *testing.T) {
	_, err := New(Config{
		Scheduler: &SchedulerConfig{
			LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
				return nil, nil
			},
			WatermarkStore: stubWatermarkStore{},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestNewRejectsMetricsWithoutAddr(t *testing.T) {
	_, err := New(Config{
		Metrics: &MetricsConfig{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestNewRejectsInvalidDispatcherConfig(t *testing.T) {
	_, err := New(Config{
		Dispatcher: &bootstrap.Config{
			PulsarClientOptions: pulsar.ClientOptions{},
			LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
				return nil, nil
			},
			Topic: "topic",
		},
		Scheduler: &SchedulerConfig{
			LoadAll: func(context.Context) (map[string]map[string]json.RawMessage, error) {
				return nil, nil
			},
			WatermarkStore: stubWatermarkStore{},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, contract.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

var _ dao.AggregateWatermarkStore = stubWatermarkStore{}
