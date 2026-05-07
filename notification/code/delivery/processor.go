package delivery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"work/notification/code/config"
	"work/notification/code/contract"
	"work/notification/code/render"
)

type ChannelSender interface {
	Send(ctx context.Context, msg *contract.DispatchMessage, cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) error
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
	Senders      map[string]ChannelSender
	Publisher    Publisher
	Recorder     Recorder
	RetryDelay   time.Duration
	MaxRetry     int
	Now          func() time.Time
}

func (p *Processor) Process(ctx context.Context, msg *contract.DispatchMessage) error {
	if p == nil || len(p.Senders) == 0 {
		return fmt.Errorf("%w: senders are required", contract.ErrInvalidRequest)
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
		return &contract.DelayError{
			Err:   contract.ErrTemporaryFailure,
			Delay: msg.ExpectedSendAt.Sub(current),
		}
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
		if errors.Is(err, contract.ErrUnsupportedConfig) {
			return p.saveFailure(ctx, msg, current, err)
		}
		return err
	}
	channelCfg, ok := cfg.ChannelForSource(msg.Source)
	if !ok {
		return p.saveFailure(ctx, msg, current, fmt.Errorf("%w: channel is required", contract.ErrUnsupportedConfig))
	}
	policy := &render.EffectivePolicy{
		TenantID:    msg.TenantID,
		MessageType: msg.MessageType,
		Channel:     channelCfg,
	}
	renderedMessages, err := render.Render(render.RenderInput{
		TenantID:    msg.TenantID,
		MessageType: msg.MessageType,
		WindowStart: msg.WindowStart,
		WindowEnd:   msg.WindowEnd,
		BizVars:     msg.BizVars,
	}, policy, p.TemplateRoot)
	if err != nil {
		return p.saveFailure(ctx, msg, current, err)
	}
	for _, channel := range renderedMessages {
		sender, ok := p.Senders[channel.Channel]
		if !ok {
			return p.saveFailure(ctx, msg, current, fmt.Errorf("%w: unsupported channel sender: %s", contract.ErrUnsupportedConfig, channel.Channel))
		}
		err = sender.Send(ctx, msg, channelCfg, channel)
		if err != nil {
			if errors.Is(err, contract.ErrInvalidRequest) || errors.Is(err, contract.ErrUnsupportedConfig) {
				return p.saveFailure(ctx, msg, current, err)
			}
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
			return p.saveFailure(ctx, msg, current, err)
		}
	}

	record := *msg
	return p.Recorder.Save(ctx, &SendRecord{
		DispatchMessage: record,
		Status:          StatusSuccess,
		UpdatedAt:       current,
	})
}

func (p *Processor) saveFailure(ctx context.Context, msg *contract.DispatchMessage, current time.Time, err error) error {
	record := *msg
	return p.Recorder.Save(ctx, &SendRecord{
		DispatchMessage: record,
		Status:          StatusFailed,
		ErrorMessage:    err.Error(),
		UpdatedAt:       current,
	})
}
