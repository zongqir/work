package consumer

import (
	"context"
	"fmt"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

type Sender interface {
	Send(ctx context.Context, msg *contract.DispatchMessage) error
}

type RetryPublisher interface {
	Publish(ctx context.Context, msg *contract.DispatchMessage) error
}

type Recorder interface {
	Save(ctx context.Context, record *SendRecord) error
}

type Status string

const (
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	DefaultMaxRetry        = 3
)

type SendRecord struct {
	MessageID      string    `json:"message_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	TenantID       string    `json:"tenant_id"`
	MessageType    string    `json:"message_type"`
	Source         string    `json:"source"`
	Status         Status    `json:"status"`
	RetryCount     int       `json:"retry_count"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpectedSendAt time.Time `json:"expected_send_at"`
	ExpireAt       time.Time `json:"expire_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Options struct {
	Sender         Sender
	RetryPublisher RetryPublisher
	Recorder       Recorder
	LogError       func(ctx context.Context, msg string, err error)
	RetryDelay     time.Duration
	MaxRetry       int
	Now            func() time.Time
}

type Consumer struct {
	Sender         Sender
	RetryPublisher RetryPublisher
	Recorder       Recorder
	LogError       func(ctx context.Context, msg string, err error)
	RetryDelay     time.Duration
	MaxRetry       int
	Now            func() time.Time
}

func New(options Options) *Consumer {
	return &Consumer{
		Sender:         options.Sender,
		RetryPublisher: options.RetryPublisher,
		Recorder:       options.Recorder,
		LogError:       options.LogError,
		RetryDelay:     options.RetryDelay,
		MaxRetry:       options.MaxRetry,
		Now:            options.Now,
	}
}

func (c *Consumer) Consume(ctx context.Context, msg *contract.DispatchMessage) (err error) {
	if c == nil || c.Sender == nil {
		return fmt.Errorf("%w: sender is required", contract.ErrInvalidRequest)
	}
	if c.Recorder == nil {
		return fmt.Errorf("%w: recorder is required", contract.ErrInvalidRequest)
	}
	if err := validateMessage(msg); err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = c.handleFailure(ctx, msg, fmt.Errorf("consume panic: %v", recovered))
		}
	}()

	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	current := now()

	if current.Before(msg.ExpectedSendAt) {
		return c.publish(ctx, msg, false)
	}
	if current.After(msg.ExpireAt) {
		return c.saveRecord(ctx, msg, StatusExpired, "")
	}
	if err := c.Sender.Send(ctx, msg); err != nil {
		return c.handleFailure(ctx, msg, err)
	}

	return c.saveRecord(ctx, msg, StatusSuccess, "")
}

func validateMessage(msg *contract.DispatchMessage) error {
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", contract.ErrInvalidRequest)
	}
	if msg.MessageID == "" {
		return fmt.Errorf("%w: message_id is required", contract.ErrInvalidRequest)
	}
	if msg.IdempotencyKey == "" {
		return fmt.Errorf("%w: idempotency_key is required", contract.ErrInvalidRequest)
	}
	if msg.TenantID == "" {
		return fmt.Errorf("%w: tenant_id is required", contract.ErrInvalidRequest)
	}
	if msg.MessageType == "" {
		return fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}
	if msg.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", contract.ErrInvalidRequest)
	}
	if msg.ExpectedSendAt.IsZero() {
		return fmt.Errorf("%w: expected_send_at is required", contract.ErrInvalidRequest)
	}
	if msg.ExpireAt.IsZero() {
		return fmt.Errorf("%w: expire_at is required", contract.ErrInvalidRequest)
	}
	return nil
}

func (c *Consumer) handleFailure(ctx context.Context, msg *contract.DispatchMessage, err error) error {
	maxRetry := c.MaxRetry
	if maxRetry <= 0 {
		maxRetry = DefaultMaxRetry
	}
	if msg.RetryCount < maxRetry {
		if retryErr := c.publish(ctx, msg, true); retryErr != nil {
			return retryErr
		}
		if c.LogError != nil {
			c.LogError(ctx, "send message failed and moved to retry", err)
		}
		return nil
	}
	return c.saveRecord(ctx, msg, StatusFailed, err.Error())
}

func (c *Consumer) publish(ctx context.Context, msg *contract.DispatchMessage, incrementRetry bool) error {
	if c.RetryPublisher == nil {
		return fmt.Errorf("%w: retry publisher is required", contract.ErrTemporaryFailure)
	}

	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	retryDelay := c.RetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Minute
	}

	retry := *msg
	if incrementRetry {
		retry.RetryCount++
		retry.ExpectedSendAt = now().Add(retryDelay)
	}

	return c.RetryPublisher.Publish(ctx, &retry)
}

func (c *Consumer) saveRecord(ctx context.Context, msg *contract.DispatchMessage, status Status, errorMessage string) error {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	return c.Recorder.Save(ctx, &SendRecord{
		MessageID:      msg.MessageID,
		IdempotencyKey: msg.IdempotencyKey,
		TenantID:       msg.TenantID,
		MessageType:    msg.MessageType,
		Source:         msg.Source,
		Status:         status,
		RetryCount:     msg.RetryCount,
		ErrorMessage:   errorMessage,
		CreatedAt:      msg.CreatedAt,
		ExpectedSendAt: msg.ExpectedSendAt,
		ExpireAt:       msg.ExpireAt,
		UpdatedAt:      now(),
	})
}
