package delivery

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
	contract.DispatchMessage
	Status       Status    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
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

type Processor struct {
	Sender         Sender
	RetryPublisher RetryPublisher
	Recorder       Recorder
	LogError       func(ctx context.Context, msg string, err error)
	RetryDelay     time.Duration
	MaxRetry       int
	Now            func() time.Time
}

func New(options Options) *Processor {
	return &Processor{
		Sender:         options.Sender,
		RetryPublisher: options.RetryPublisher,
		Recorder:       options.Recorder,
		LogError:       options.LogError,
		RetryDelay:     options.RetryDelay,
		MaxRetry:       options.MaxRetry,
		Now:            options.Now,
	}
}

func (p *Processor) Process(ctx context.Context, msg *contract.DispatchMessage) (err error) {
	if p == nil || p.Sender == nil {
		return fmt.Errorf("%w: sender is required", contract.ErrInvalidRequest)
	}
	if p.Recorder == nil {
		return fmt.Errorf("%w: recorder is required", contract.ErrInvalidRequest)
	}
	if err := validateMessage(msg); err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = p.retryOrSaveFailure(ctx, msg, fmt.Errorf("process panic: %v", recovered))
		}
	}()

	current := time.Now()
	if p.Now != nil {
		current = p.Now()
	}

	if current.Before(msg.ExpectedSendAt) {
		return p.publish(ctx, msg, false)
	}
	if current.After(msg.ExpireAt) {
		return p.saveRecord(ctx, msg, StatusExpired, "")
	}
	if err := p.Sender.Send(ctx, msg); err != nil {
		return p.retryOrSaveFailure(ctx, msg, err)
	}

	return p.saveRecord(ctx, msg, StatusSuccess, "")
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

func (p *Processor) retryOrSaveFailure(ctx context.Context, msg *contract.DispatchMessage, err error) error {
	maxRetry := p.MaxRetry
	if maxRetry <= 0 {
		maxRetry = DefaultMaxRetry
	}
	if msg.RetryCount < maxRetry {
		if retryErr := p.publish(ctx, msg, true); retryErr != nil {
			return retryErr
		}
		if p.LogError != nil {
			p.LogError(ctx, "send message failed and moved to retry", err)
		}
		return nil
	}
	return p.saveRecord(ctx, msg, StatusFailed, err.Error())
}

func (p *Processor) publish(ctx context.Context, msg *contract.DispatchMessage, incrementRetry bool) error {
	if p.RetryPublisher == nil {
		return fmt.Errorf("%w: retry publisher is required", contract.ErrTemporaryFailure)
	}

	retryDelay := p.RetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Minute
	}

	retry := *msg
	if incrementRetry {
		now := time.Now()
		if p.Now != nil {
			now = p.Now()
		}
		retry.RetryCount++
		retry.ExpectedSendAt = now.Add(retryDelay)
	}

	return p.RetryPublisher.Publish(ctx, &retry)
}

func (p *Processor) saveRecord(ctx context.Context, msg *contract.DispatchMessage, status Status, errorMessage string) error {
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	record := *msg
	return p.Recorder.Save(ctx, &SendRecord{
		DispatchMessage: record,
		Status:          status,
		ErrorMessage:    errorMessage,
		UpdatedAt:       now,
	})
}
