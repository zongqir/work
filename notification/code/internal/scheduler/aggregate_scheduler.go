package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"work/notification/code/config"
	"work/notification/code/internal/dao"
	"work/notification/code/pkg/notification/contract"
)

type AggregateSender interface {
	SendAggregate(ctx context.Context, tenantID, messageType string, windowStart, windowEnd time.Time) (bool, error)
}

type AggregateScheduler struct {
	LoadAll        func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
	Sender         AggregateSender
	WatermarkStore dao.AggregateWatermarkStore
	LogError       func(ctx context.Context, msg string, err error)
	PollInterval   time.Duration
	Now            func() time.Time
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

	if err := s.Tick(ctx); err != nil {
		if s.LogError == nil {
			return err
		}
		s.LogError(ctx, "aggregate scheduler tick failed", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.Tick(ctx); err != nil {
				if s.LogError == nil {
					return err
				}
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

	cfg, err := config.ParseMessageConfig(raw)
	if err != nil {
		return err
	}
	if cfg == nil || !cfg.AggregateEnabled || cfg.AggregatePeriodMinutes <= 0 {
		return nil
	}

	period := time.Duration(cfg.AggregatePeriodMinutes) * time.Minute
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
	sent, err := s.Sender.SendAggregate(ctx, tenantID, messageType, windowStart, windowEnd)
	if err != nil {
		return err
	}
	if !sent {
		return nil
	}
	if err := s.WatermarkStore.SaveWindowEnd(ctx, tenantID, messageType, windowEnd); err != nil {
		return err
	}
	return nil
}

func truncateToPeriod(value time.Time, period time.Duration) time.Time {
	if period <= 0 {
		return time.Time{}
	}
	return value.Truncate(period)
}
