package notifysdk

import (
	"context"
	"time"
)

type Mode string

const (
	ModeHTTP   Mode = "http"
	ModeMQ     Mode = "mq"
	ModeOutbox Mode = "outbox"
)

type Result struct {
	Accepted     bool   `json:"accepted"`
	RequestID    string `json:"requestId,omitempty"`
	DeliveryMode Mode   `json:"deliveryMode"`
	Queued       bool   `json:"queued,omitempty"`
}

type Transport interface {
	Name() Mode
	Send(context.Context, Envelope) (Result, error)
}

type Client struct {
	transport Transport
	validator Validator
	metrics   Metrics
	outboxTx  OutboxTxStore
	now       func() time.Time
}

type Option func(*Client)

func WithValidator(v Validator) Option {
	return func(c *Client) {
		if v != nil {
			c.validator = v
		}
	}
}

func WithMetrics(m Metrics) Option {
	return func(c *Client) {
		if m != nil {
			c.metrics = m
		}
	}
}

func WithOutboxTxStore(store OutboxTxStore) Option {
	return func(c *Client) {
		if store != nil {
			c.outboxTx = store
		}
	}
}

func WithNow(now func() time.Time) Option {
	return func(c *Client) {
		if now != nil {
			c.now = now
		}
	}
}

func New(transport Transport, opts ...Option) *Client {
	client := &Client{
		transport: transport,
		validator: DefaultValidator{},
		metrics:   NoopMetrics{},
		now:       time.Now,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) Send(ctx context.Context, cmd Command) (Result, error) {
	start := time.Now()
	mode := c.transport.Name()

	if err := c.validator.Validate(cmd); err != nil {
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	envelope, err := cmd.Envelope()
	if err != nil {
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	result, err := c.transport.Send(ctx, envelope)
	if err != nil {
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	sendResult := "success"
	if !result.Accepted {
		sendResult = "rejected"
	}
	c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, sendResult, time.Since(start))

	return result, nil
}

func (c *Client) EnqueueInTx(ctx context.Context, tx Tx, cmd Command) (Result, error) {
	start := time.Now()
	mode := ModeOutbox

	if c.outboxTx == nil {
		err := ErrOutboxNotConfigured
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	if err := c.validator.Validate(cmd); err != nil {
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	envelope, err := cmd.Envelope()
	if err != nil {
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	message := OutboxMessage{
		ID:            envelope.IdempotentKey,
		Envelope:      envelope,
		Status:        OutboxPending,
		RetryCount:    0,
		NextAttemptAt: c.now(),
		CreatedAt:     c.now(),
		UpdatedAt:     c.now(),
	}

	if err := c.outboxTx.SaveInTx(ctx, tx, message); err != nil {
		err = wrapError(ErrEnqueueFailed, err)
		c.metrics.RecordError(mode, classifyError(err))
		c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "fail", time.Since(start))
		return Result{}, err
	}

	c.metrics.RecordSend(mode, cmd.BizType, cmd.EventCode, "success", time.Since(start))
	return Result{
		Accepted:     true,
		RequestID:    message.ID,
		DeliveryMode: ModeOutbox,
		Queued:       true,
	}, nil
}
