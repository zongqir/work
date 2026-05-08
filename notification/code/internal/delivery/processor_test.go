package delivery

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"work/notification/code/config"
	"work/notification/code/internal/render"
	"work/notification/code/pkg/notification/contract"
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
		TemplateRoot: filepath.Join("..", "..", "templates"),
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
	if recorder.record.IdempotencyKey != "realtime:t_1:sample_both:biz-1" {
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
		TemplateRoot: filepath.Join("..", "..", "templates"),
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

func TestProcessUsesSameChannelForRealtimeSource(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	sender := &stubSender{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return sharedSMSConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": sender,
		},
		Recorder: &stubRecorder{},
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.cfg == nil {
		t.Fatal("expected sender config")
	}
	if sender.cfg.TemplateKey != "commonTemplateShared" {
		t.Fatalf("unexpected template key: %+v", sender.cfg)
	}
	if len(sender.cfg.Audience.Recipients) != 1 || sender.cfg.Audience.Recipients[0] != "13111111111" {
		t.Fatalf("unexpected audience: %+v", sender.cfg.Audience)
	}
}

func TestProcessUsesSameChannelForAggregateSource(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	sender := &stubSender{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return sharedSMSConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": sender,
		},
		Recorder: &stubRecorder{},
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.Source = contract.DispatchSourceAggregate
	msg.IdempotencyKey = "aggregate:t_1:sample_both:202604291300"

	err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.cfg == nil {
		t.Fatal("expected sender config")
	}
	if sender.cfg.TemplateKey != "commonTemplateShared" {
		t.Fatalf("unexpected template key: %+v", sender.cfg)
	}
	if len(sender.cfg.Audience.Recipients) != 1 || sender.cfg.Audience.Recipients[0] != "13111111111" {
		t.Fatalf("unexpected audience: %+v", sender.cfg.Audience)
	}
}

func TestProcessLoadsSystemVarsForRendering(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	sender := &stubSender{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		LoadSystemVars: func(_ context.Context, msg *contract.DispatchMessage) (contract.TemplateVars, error) {
			if msg.TenantID != "t_1" {
				t.Fatalf("unexpected tenant id: %s", msg.TenantID)
			}
			return contract.TemplateVars{
				"tenant_name": "租户A",
			}, nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": sender,
		},
		Recorder: &stubRecorder{},
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.channel == nil || sender.channel.SMS == nil {
		t.Fatal("expected sms to be rendered")
	}
	if paramValue(sender.channel.SMS.TemplateParams, "tenant_name") != "租户A" {
		t.Fatalf("expected tenant_name system var, got %+v", sender.channel.SMS.TemplateParams)
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
		TemplateRoot: filepath.Join("..", "..", "templates"),
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
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{},
		},
		Recorder: &stubRecorder{},
		Now: func() time.Time {
			return now
		},
	}

	msg := newMessage(now)
	msg.ExpectedSendAt = now.Add(2 * time.Minute)

	err := p.Process(context.Background(), msg)
	var delayErr *contract.DelayError
	if !errors.As(err, &delayErr) {
		t.Fatalf("expected DelayError, got %v", err)
	}
	if delayErr.Delay != 2*time.Minute {
		t.Fatalf("unexpected delay: %v", delayErr.Delay)
	}
}

func TestProcessExpired(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 31, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
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

func TestProcessRecordsFailureOnUnsupportedConfig(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return &config.MessageConfig{}, nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{},
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
	if recorder.record == nil {
		t.Fatal("expected failed record")
	}
	if recorder.record.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", recorder.record.Status)
	}
	if recorder.record.ErrorMessage == "" {
		t.Fatal("expected failure error message")
	}
}

func TestProcessRecordsFailureOnUnsupportedSender(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"email": &stubSender{},
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
	if recorder.record == nil {
		t.Fatal("expected failed record")
	}
	if recorder.record.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", recorder.record.Status)
	}
}

func TestProcessRecordsFailureOnPermanentSenderError(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	publisher := &stubPublisher{}
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
		Senders: map[string]ChannelSender{
			"sms": &stubSender{
				err: contract.ErrInvalidRequest,
			},
		},
		Publisher: publisher,
		Recorder:  recorder,
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if publisher.msg != nil {
		t.Fatal("did not expect retry message")
	}
	if recorder.record == nil {
		t.Fatal("expected failed record")
	}
	if recorder.record.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", recorder.record.Status)
	}
}

func TestProcessFinalFailure(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	recorder := &stubRecorder{}
	p := &Processor{
		LoadConfig: func(context.Context, string, string) (*config.MessageConfig, error) {
			return smsConfig(), nil
		},
		TemplateRoot: filepath.Join("..", "..", "templates"),
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
		TemplateRoot: filepath.Join("..", "..", "templates"),
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
		IdempotencyKey: "realtime:t_1:sample_both:biz-1",
		TenantID:       "t_1",
		MessageType:    "sample_both",
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

func paramValue(params []render.RenderedParam, name string) string {
	for _, param := range params {
		if param.ParamName == name {
			return param.ParamValue
		}
	}
	return ""
}

func smsConfig() *config.MessageConfig {
	return &config.MessageConfig{
		Channel: render.ChannelPolicy{
			Channel:     "sms",
			TemplateKey: "commonTemplate",
			Audience: render.AudienceConfig{
				Recipients: []string{"13111223344"},
			},
			Delivery: render.DeliveryConfig{
				Platform: "ali",
			},
		},
	}
}

func sharedSMSConfig() *config.MessageConfig {
	return &config.MessageConfig{
		Channel: render.ChannelPolicy{
			Channel:     "sms",
			TemplateKey: "commonTemplateShared",
			Audience: render.AudienceConfig{
				Recipients: []string{"13111111111"},
			},
			Delivery: render.DeliveryConfig{
				Platform: "ali",
			},
		},
	}
}
