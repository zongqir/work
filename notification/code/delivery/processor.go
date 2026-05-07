package delivery

import (
	"context"
	"fmt"
	"time"

	"work/notification/code/config"
	"work/notification/code/contract"
	"work/notification/code/render"
)

type Sender interface {
	Send(ctx context.Context, msg *contract.DispatchMessage, channel render.RenderedChannelMessage) error
}

type Publisher interface {
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

type Processor struct {
	LoadConfig   func(ctx context.Context, tenantID, messageType string) (*config.MessageConfig, error)
	TemplateRoot string
	Sender       Sender
	Publisher    Publisher
	Recorder     Recorder
	RetryDelay   time.Duration
	MaxRetry     int
	Now          func() time.Time
}

func (p *Processor) Process(ctx context.Context, msg *contract.DispatchMessage) error {
	if p == nil || p.Sender == nil {
		return fmt.Errorf("%w: sender is required", contract.ErrInvalidRequest)
	}
	if p.LoadConfig == nil {
		return fmt.Errorf("%w: load_config is required", contract.ErrInvalidRequest)
	}
	if p.TemplateRoot == "" {
		return fmt.Errorf("%w: template_root is required", contract.ErrInvalidRequest)
	}
	if p.Recorder == nil {
		return fmt.Errorf("%w: recorder is required", contract.ErrInvalidRequest)
	}
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", contract.ErrInvalidRequest)
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
	if msg.ExpectedSendAt.IsZero() {
		return fmt.Errorf("%w: expected_send_at is required", contract.ErrInvalidRequest)
	}
	if msg.ExpireAt.IsZero() {
		return fmt.Errorf("%w: expire_at is required", contract.ErrInvalidRequest)
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	current := now()

	if current.Before(msg.ExpectedSendAt) {
		if p.Publisher == nil {
			return fmt.Errorf("%w: publisher is required", contract.ErrTemporaryFailure)
		}
		pending := *msg
		return p.Publisher.Publish(ctx, &pending)
	}
	if current.After(msg.ExpireAt) {
		record := *msg
		return p.Recorder.Save(ctx, &SendRecord{
			DispatchMessage: record,
			Status:          StatusExpired,
			UpdatedAt:       current,
		})
	}

	cfg, err := p.LoadConfig(ctx, msg.TenantID, msg.MessageType)
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.Channels) == 0 {
		return fmt.Errorf("%w: channels are required", contract.ErrUnsupportedConfig)
	}
	policy := &render.EffectivePolicy{
		TenantID:    msg.TenantID,
		MessageType: msg.MessageType,
		Channels:    cfg.Channels,
	}
	renderedMessages, err := render.RenderDispatch(msg, policy, p.TemplateRoot)
	if err != nil {
		return err
	}
	for _, channel := range renderedMessages {
		if err := p.Sender.Send(ctx, msg, channel); err != nil {
			maxRetry := p.MaxRetry
			if maxRetry <= 0 {
				maxRetry = DefaultMaxRetry
			}
			if msg.RetryCount < maxRetry {
				if p.Publisher == nil {
					return fmt.Errorf("%w: publisher is required", contract.ErrTemporaryFailure)
				}
				retryDelay := p.RetryDelay
				if retryDelay <= 0 {
					retryDelay = time.Minute
				}
				retry := *msg
				retry.RetryCount++
				retry.ExpectedSendAt = current.Add(retryDelay)
				return p.Publisher.Publish(ctx, &retry)
			}
			record := *msg
			return p.Recorder.Save(ctx, &SendRecord{
				DispatchMessage: record,
				Status:          StatusFailed,
				ErrorMessage:    err.Error(),
				UpdatedAt:       current,
			})
		}
	}

	record := *msg
	return p.Recorder.Save(ctx, &SendRecord{
		DispatchMessage: record,
		Status:          StatusSuccess,
		UpdatedAt:       current,
	})
}
