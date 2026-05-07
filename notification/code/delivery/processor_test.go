package delivery

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"work/notification/code/config"
	"work/notification/code/contract"
	"work/notification/code/render"
)

type stubSender struct {
	err     error
	msg     *contract.DispatchMessage
	cfg     *render.ChannelPolicy
	channel *render.RenderedChannelMessage
}

func (s *stubSender) Send(_ context.Context, msg *contract.DispatchMessage, cfg render.ChannelPolicy, channel render.RenderedChannelMessage) error {
	s.msg = msg
	s.cfg = &cfg
	s.channel = &channel
	return s.err
}

type stubPublisher struct {
	msg *contract.DispatchMessage
}

func (p *stubPublisher) Publish(_ context.Context, msg *contract.DispatchMessage) error {
	p.msg = msg
	return nil
}

type stubRecorder struct {
	record *SendRecord
}

func (r *stubRecorder) Save(_ context.Context, record *SendRecord) error {
	r.record = record
	return nil
}

func TestProcessSuccess(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	sender := &stubSender{}
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": sender,
		},
		Recorder: recorder,
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.msg == nil {
		t.Fatal("expected sender to be called")
	}
	if sender.channel == nil || sender.channel.Channel != "sms" {
		t.Fatal("expected sms channel to be rendered")
	}
	if sender.cfg == nil || len(sender.cfg.Audience.Recipients) != 1 || sender.cfg.Audience.Recipients[0] != "13111223344" {
		t.Fatalf("unexpected sender cfg: %+v", sender.cfg)
	}
	if recorder.record == nil {
		t.Fatal("expected record to be saved")
	}
	if recorder.record.IdempotencyKey != "realtime:t_1:xdr_risk_digest:biz-1" {
		t.Fatalf("unexpected idempotency_key: %s", recorder.record.IdempotencyKey)
	}
	if recorder.record.Status != StatusSuccess {
		t.Fatalf("expected success, got %s", recorder.record.Status)
	}
}

func TestProcessSuccessWithoutCreatedAt(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	sender := &stubSender{}
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": sender,
		},
		Recorder: recorder,
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.CreatedAt = time.Time{}

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.msg == nil {
		t.Fatal("expected sender to be called")
	}
}

func TestProcessRetryOnSendFailure(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	publisher := &stubPublisher{}
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{
				err: errors.New("send failed"),
			},
		},
		Publisher:  publisher,
		Recorder:   recorder,
		RetryDelay: 2 * time.Minute,
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if publisher.msg == nil {
		t.Fatal("expected retry message to be published")
	}
	if publisher.msg.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", publisher.msg.RetryCount)
	}
	if !publisher.msg.ExpectedSendAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("unexpected expected_send_at: %v", publisher.msg.ExpectedSendAt)
	}
	if recorder.record != nil {
		t.Fatal("did not expect final record on retry")
	}
}

func TestProcessBeforeExpectedSendAt(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	publisher := &stubPublisher{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{},
		},
		Publisher: publisher,
		Recorder:  &stubRecorder{},
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.ExpectedSendAt = now.Add(2 * time.Minute)

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if publisher.msg == nil {
		t.Fatal("expected message to be re-published")
	}
	if publisher.msg.RetryCount != 0 {
		t.Fatalf("expected retry_count=0, got %d", publisher.msg.RetryCount)
	}
	if !publisher.msg.ExpectedSendAt.Equal(msg.ExpectedSendAt) {
		t.Fatalf("unexpected expected_send_at: %v", publisher.msg.ExpectedSendAt)
	}
}

func TestProcessExpired(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 31, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{},
		},
		Recorder: recorder,
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC))
	msg.ExpireAt = time.Date(2026, 4, 29, 13, 30, 0, 0, time.UTC)

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if recorder.record == nil {
		t.Fatal("expected expired record")
	}
	if recorder.record.Status != StatusExpired {
		t.Fatalf("expected expired, got %s", recorder.record.Status)
	}
}

func TestProcessFinalFailure(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{
				err: errors.New("send failed"),
			},
		},
		Recorder: recorder,
		MaxRetry: 3,
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.RetryCount = 3

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if recorder.record == nil {
		t.Fatal("expected failed record")
	}
	if recorder.record.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", recorder.record.Status)
	}
}

func TestProcessThirdRetryStillPublishes(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	publisher := &stubPublisher{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{
				err: errors.New("send failed"),
			},
		},
		Publisher: publisher,
		Recorder:  &stubRecorder{},
		MaxRetry:  DefaultMaxRetry,
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.RetryCount = 2

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if publisher.msg == nil {
		t.Fatal("expected retry message to be published")
	}
	if publisher.msg.RetryCount != 3 {
		t.Fatalf("expected retry_count=3, got %d", publisher.msg.RetryCount)
	}
}

func newMessage(createdAt time.Time) *contract.DispatchMessage {
	return &contract.DispatchMessage{
		IdempotencyKey: "realtime:t_1:xdr_risk_digest:biz-1",
		TenantID:       "t_1",
		MessageType:    "xdr_risk_digest",
		Source:         contract.DispatchSourceRealtime,
		RetryCount:     0,
		CreatedAt:      createdAt,
		ExpectedSendAt: createdAt,
		ExpireAt:       createdAt.Add(30 * time.Minute),
		BizVars: contract.TemplateVars{
			"k": "v",
		},
	}
}

func smsConfig() *config.MessageConfig {
	return &config.MessageConfig{
		Channels: []render.ChannelPolicy{
			{
				Channel: "sms",
				Template: render.TemplateConfig{
					TemplateKey: "commonTemplate",
				},
				Audience: render.AudienceConfig{
					Recipients: []string{"13111223344"},
				},
				Delivery: render.DeliveryConfig{
					Platform: "ali",
				},
			},
		},
	}
}
