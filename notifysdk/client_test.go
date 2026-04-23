package notifysdk

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestClientSendMQ(t *testing.T) {
	publisher := &stubPublisher{}
	client := New(&MQTransport{
		Topic:     "notification.events",
		Publisher: publisher,
	})

	result, err := client.Send(context.Background(), Command{
		BizType:      "trade",
		EventCode:    "order_paid",
		TemplateCode: "tpl_order_paid",
		Receivers:    []Receiver{{Type: "wechat", Value: "openid-1"}},
		Payload: OrderPaidPayload{
			OrderID: "o-1",
			UserID:  "u-1",
		},
		IdempotentKey: "trade:order_paid:o-1",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !result.Accepted || result.DeliveryMode != ModeMQ {
		t.Fatalf("unexpected result: %+v", result)
	}
	if publisher.topic != "notification.events" {
		t.Fatalf("unexpected topic: %s", publisher.topic)
	}

	var envelope Envelope
	if err := json.Unmarshal(publisher.body, &envelope); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if envelope.IdempotentKey != "trade:order_paid:o-1" {
		t.Fatalf("unexpected idempotent key: %s", envelope.IdempotentKey)
	}
}

func TestOutboxDispatchOnce(t *testing.T) {
	store := NewMemoryOutboxStore()
	client := New(&OutboxTransport{
		Store: store,
		Now: func() time.Time {
			return time.Unix(100, 0)
		},
	})

	_, err := client.Send(context.Background(), Command{
		BizType:      "trade",
		EventCode:    "refund_created",
		TemplateCode: "tpl_refund_created",
		Receivers:    []Receiver{{Type: "email", Value: "user@example.com"}},
		Payload: RefundCreatedPayload{
			RefundID: "r-1",
		},
		IdempotentKey: "trade:refund_created:r-1",
	})
	if err != nil {
		t.Fatalf("enqueue error = %v", err)
	}

	publisher := &stubPublisher{}
	dispatcher := &Dispatcher{
		Store: store,
		Relay: &MQTransport{
			Topic:     "notification.events",
			Publisher: publisher,
		},
		RetryPolicy: FixedDelays{Delays: []time.Duration{time.Minute}},
		Now: func() time.Time {
			return time.Unix(100, 0)
		},
	}

	report, err := dispatcher.DispatchOnce(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if report.Sent != 1 || report.Processed != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestClientSendRejectsInvalidPayload(t *testing.T) {
	metrics := &stubMetrics{}
	publisher := &stubPublisher{}
	client := New(&MQTransport{
		Topic:     "notification.events",
		Publisher: publisher,
	}, WithMetrics(metrics))

	_, err := client.Send(context.Background(), Command{
		BizType:      "trade",
		EventCode:    "order_paid",
		TemplateCode: "tpl_order_paid",
		Receivers:    []Receiver{{Type: "wechat", Value: "openid-1"}},
		Payload: OrderPaidPayload{
			OrderID: "",
			UserID:  "u-1",
		},
		IdempotentKey: "trade:order_paid:o-1",
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected invalid argument error, got %v", err)
	}
	if metrics.errorType != "invalid_argument" {
		t.Fatalf("unexpected error type: %s", metrics.errorType)
	}
	if metrics.sendResult != "fail" {
		t.Fatalf("unexpected send result: %s", metrics.sendResult)
	}
}

func TestClientEnqueueInTx(t *testing.T) {
	store := &stubOutboxTxStore{}
	client := New(&MQTransport{
		Topic:     "notification.events",
		Publisher: &stubPublisher{},
	}, WithOutboxTxStore(store), WithNow(func() time.Time {
		return time.Unix(100, 0)
	}))

	result, err := client.EnqueueInTx(context.Background(), stubTx{}, Command{
		BizType:      "trade",
		EventCode:    "order_paid",
		TemplateCode: "tpl_order_paid",
		Receivers:    []Receiver{{Type: "wechat", Value: "openid-1"}},
		Payload: OrderPaidPayload{
			OrderID: "o-1",
			UserID:  "u-1",
		},
		IdempotentKey: "trade:order_paid:o-1",
	})
	if err != nil {
		t.Fatalf("EnqueueInTx() error = %v", err)
	}
	if !result.Queued || result.DeliveryMode != ModeOutbox {
		t.Fatalf("unexpected result: %+v", result)
	}
	if store.saved.ID != "trade:order_paid:o-1" {
		t.Fatalf("unexpected saved message id: %s", store.saved.ID)
	}
	if store.saved.Status != OutboxPending {
		t.Fatalf("unexpected saved status: %s", store.saved.Status)
	}
}

func TestClientEnqueueInTxRequiresStore(t *testing.T) {
	client := New(&MQTransport{
		Topic:     "notification.events",
		Publisher: &stubPublisher{},
	})

	_, err := client.EnqueueInTx(context.Background(), stubTx{}, Command{
		BizType:      "trade",
		EventCode:    "order_paid",
		TemplateCode: "tpl_order_paid",
		Receivers:    []Receiver{{Type: "wechat", Value: "openid-1"}},
		Payload: OrderPaidPayload{
			OrderID: "o-1",
			UserID:  "u-1",
		},
		IdempotentKey: "trade:order_paid:o-1",
	})
	if !errors.Is(err, ErrOutboxNotConfigured) {
		t.Fatalf("expected outbox store error, got %v", err)
	}
}

type stubPublisher struct {
	topic string
	key   string
	body  []byte
}

func (s *stubPublisher) Publish(_ context.Context, topic string, key string, body []byte, _ map[string]string) error {
	s.topic = topic
	s.key = key
	s.body = append([]byte(nil), body...)
	return nil
}

type stubMetrics struct {
	sendMode   Mode
	sendResult string
	errorType  string
	dispatch   []string
	outbox     map[OutboxStatus]int
}

func (s *stubMetrics) RecordSend(mode Mode, _, _ string, result string, _ time.Duration) {
	s.sendMode = mode
	s.sendResult = result
}

func (s *stubMetrics) RecordError(_ Mode, errorType string) {
	s.errorType = errorType
}

func (s *stubMetrics) RecordDispatch(result string) {
	s.dispatch = append(s.dispatch, result)
}

func (s *stubMetrics) SetOutboxSize(status OutboxStatus, count int) {
	if s.outbox == nil {
		s.outbox = make(map[OutboxStatus]int)
	}
	s.outbox[status] = count
}

type stubOutboxTxStore struct {
	saved OutboxMessage
}

func (s *stubOutboxTxStore) SaveInTx(_ context.Context, _ Tx, msg OutboxMessage) error {
	s.saved = msg
	return nil
}

type stubTx struct{}

func (stubTx) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return stubResult(0), nil
}

type stubResult int64

func (r stubResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r stubResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

type OrderPaidPayload struct {
	OrderID string `json:"orderId"`
	UserID  string `json:"userId"`
}

func (p OrderPaidPayload) Validate() error {
	if p.OrderID == "" {
		return errors.New("orderId is required")
	}
	if p.UserID == "" {
		return errors.New("userId is required")
	}
	return nil
}

type RefundCreatedPayload struct {
	RefundID string `json:"refundId"`
}

func (p RefundCreatedPayload) Validate() error {
	if p.RefundID == "" {
		return errors.New("refundId is required")
	}
	return nil
}
