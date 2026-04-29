package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

type AggregateSender interface {
	SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) error
}

type AggregateWatermarkStore interface {
	LastWindowEnd(ctx context.Context, tenantID, messageType string) (time.Time, error)
	SaveWindowEnd(ctx context.Context, tenantID, messageType string, windowEnd time.Time) error
}

type AggregateScheduler struct {
	LoadAll        func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	Sender         AggregateSender
	WatermarkStore AggregateWatermarkStore
	LogError       func(ctx context.Context, msg string, err error)
	PollInterval   time.Duration
	Now            func() time.Time
}

type AggregateSchedulerOptions struct {
	LoadAll        func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	Sender         AggregateSender
	WatermarkStore AggregateWatermarkStore
	LogError       func(ctx context.Context, msg string, err error)
	PollInterval   time.Duration
	Now            func() time.Time
}

func NewAggregateScheduler(options AggregateSchedulerOptions) *AggregateScheduler {
	return &AggregateScheduler{
		LoadAll:        options.LoadAll,
		Sender:         options.Sender,
		WatermarkStore: options.WatermarkStore,
		LogError:       options.LogError,
		PollInterval:   options.PollInterval,
		Now:            options.Now,
	}
}

func (s *AggregateScheduler) Run(ctx context.Context) error {
	if s == nil || s.LoadAll == nil {
		return fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}
	if s.Sender == nil {
		return fmt.Errorf("%w: aggregate sender is required", contract.ErrInvalidRequest)
	}
	if s.WatermarkStore == nil {
		return fmt.Errorf("%w: watermark store is required", contract.ErrInvalidRequest)
	}

	interval := s.PollInterval
	if interval <= 0 {
		interval = time.Minute
	}

	if err := s.Tick(ctx); err != nil && s.LogError != nil {
		s.LogError(ctx, "aggregate scheduler tick failed", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.Tick(ctx); err != nil && s.LogError != nil {
				s.LogError(ctx, "aggregate scheduler tick failed", err)
			}
		}
	}
}

func (s *AggregateScheduler) Tick(ctx context.Context) error {
	if s == nil || s.LoadAll == nil {
		return fmt.Errorf("%w: load_all is required", contract.ErrInvalidRequest)
	}
	if s.Sender == nil {
		return fmt.Errorf("%w: aggregate sender is required", contract.ErrInvalidRequest)
	}
	if s.WatermarkStore == nil {
		return fmt.Errorf("%w: watermark store is required", contract.ErrInvalidRequest)
	}

	all, err := s.LoadAll(ctx)
	if err != nil {
		return err
	}

	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	current := now()

	for tenantID, tenantConfigs := range all {
		for messageType, raw := range tenantConfigs {
			if err := s.tickOne(ctx, current, tenantID, messageType, raw); err != nil {
				if s.LogError != nil {
					s.LogError(ctx, "aggregate scheduler handle config failed", err)
					continue
				}
				return err
			}
		}
	}
	return nil
}

func (s *AggregateScheduler) tickOne(
	ctx context.Context,
	current time.Time,
	tenantID, messageType string,
	raw json.RawMessage,
) error {
	if len(raw) == 0 {
		return nil
	}

	var config messageConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if !config.Enabled || config.AggregatePeriodMinutes <= 0 {
		return nil
	}

	period := time.Duration(config.AggregatePeriodMinutes) * time.Minute
	windowEnd := truncateToPeriod(current.UTC(), period)
	if windowEnd.IsZero() {
		return nil
	}

	lastWindowEnd, err := s.WatermarkStore.LastWindowEnd(ctx, tenantID, messageType)
	if err != nil {
		return err
	}
	if !lastWindowEnd.IsZero() {
		nextWindowEnd := lastWindowEnd.Add(period)
		if nextWindowEnd.After(windowEnd) {
			return nil
		}
		windowEnd = nextWindowEnd
	}

	windowStart := windowEnd.Add(-period)
	if err := s.Sender.SendAggregate(ctx, tenantID, messageType, windowStart, windowEnd); err != nil {
		return err
	}
	return s.WatermarkStore.SaveWindowEnd(ctx, tenantID, messageType, windowEnd)
}

func truncateToPeriod(value time.Time, period time.Duration) time.Time {
	if period <= 0 {
		return time.Time{}
	}
	return value.Truncate(period)
}
