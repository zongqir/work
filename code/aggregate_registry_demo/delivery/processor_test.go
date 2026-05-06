package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"notes/code/aggregate_registry_demo/contract"
)

type stubSender struct {
	err  error
	sent *contract.DispatchMessage
}

func (s *stubSender) Send(_ context.Context, msg *contract.DispatchMessage) error {
	s.sent = msg
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
		Sender:   sender,
		Recorder: recorder,
		Now: func() time.Time {
			return now
		},
	}

	err := p.Process(context.Background(), newMessage(now))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if sender.sent == nil {
		t.Fatal("expected sender to be called")
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
		Sender:   sender,
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
	if sender.sent == nil {
		t.Fatal("expected sender to be called")
	}
}

func TestProcessRetryOnSendFailure(t *testing.T) {
	now := time.Date(2026, 4, 29, 13, 0, 0, 0, time.UTC)
	publisher := &stubPublisher{}
	recorder := &stubRecorder{}
	p := &Processor{
		Sender: &stubSender{
			err: errors.New("send failed"),
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
		Sender:    &stubSender{},
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
		Sender:   &stubSender{},
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
		Sender: &stubSender{
			err: errors.New("send failed"),
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
		Sender: &stubSender{
			err: errors.New("send failed"),
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
