package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"notes/code/aggregate_registry/contract"
)

type stubProcessor struct {
	err error
	msg *contract.DispatchMessage
}

func (p *stubProcessor) Process(_ context.Context, msg *contract.DispatchMessage) error {
	p.msg = msg
	return p.err
}

type stubConsumer struct {
	acked  int
	nacked int
}

func (c *stubConsumer) Subscription() string { return "sub" }
func (c *stubConsumer) Unsubscribe() error   { return nil }
func (c *stubConsumer) UnsubscribeForce() error {
	return nil
}
func (c *stubConsumer) GetLastMessageIDs() ([]pulsar.TopicMessageID, error) {
	return nil, nil
}
func (c *stubConsumer) Receive(context.Context) (pulsar.Message, error) {
	return nil, errors.New("not implemented")
}
func (c *stubConsumer) Chan() <-chan pulsar.ConsumerMessage { return nil }
func (c *stubConsumer) Ack(pulsar.Message) error {
	c.acked++
	return nil
}
func (c *stubConsumer) AckID(pulsar.MessageID) error { return nil }
func (c *stubConsumer) AckIDList([]pulsar.MessageID) error {
	return nil
}
func (c *stubConsumer) AckWithTxn(pulsar.Message, pulsar.Transaction) error {
	return nil
}
func (c *stubConsumer) AckCumulative(pulsar.Message) error { return nil }
func (c *stubConsumer) AckIDCumulative(pulsar.MessageID) error {
	return nil
}
func (c *stubConsumer) ReconsumeLater(pulsar.Message, time.Duration) {}
func (c *stubConsumer) ReconsumeLaterWithCustomProperties(pulsar.Message, map[string]string, time.Duration) {
}
func (c *stubConsumer) Nack(pulsar.Message)     { c.nacked++ }
func (c *stubConsumer) NackID(pulsar.MessageID) {}
func (c *stubConsumer) Close()                  {}
func (c *stubConsumer) Seek(pulsar.MessageID) error {
	return nil
}
func (c *stubConsumer) SeekByTime(time.Time) error { return nil }
func (c *stubConsumer) Name() string               { return "consumer" }

type stubMessage struct {
	payload []byte
}

func (m *stubMessage) Topic() string                 { return "topic" }
func (m *stubMessage) ProducerName() string          { return "producer" }
func (m *stubMessage) Properties() map[string]string { return nil }
func (m *stubMessage) Payload() []byte               { return m.payload }
func (m *stubMessage) ID() pulsar.MessageID          { return nil }
func (m *stubMessage) PublishTime() time.Time        { return time.Time{} }
func (m *stubMessage) EventTime() time.Time          { return time.Time{} }
func (m *stubMessage) Key() string                   { return "" }
func (m *stubMessage) OrderingKey() string           { return "" }
func (m *stubMessage) RedeliveryCount() uint32       { return 0 }
func (m *stubMessage) IsReplicated() bool            { return false }
func (m *stubMessage) GetReplicatedFrom() string     { return "" }
func (m *stubMessage) GetSchemaValue(any) error      { return nil }
func (m *stubMessage) SchemaVersion() []byte         { return nil }
func (m *stubMessage) GetEncryptionContext() *pulsar.EncryptionContext {
	return nil
}
func (m *stubMessage) Index() *uint64                { return nil }
func (m *stubMessage) BrokerPublishTime() *time.Time { return nil }

func TestHandleMessageAckOnSuccess(t *testing.T) {
	rawConsumer := &stubConsumer{}
	processor := &stubProcessor{}
	c := &PulsarConsumer{
		consumer:  rawConsumer,
		processor: processor,
	}

	err := c.handleMessage(context.Background(), &stubMessage{
		payload: []byte(`{"idempotency_key":"k","tenant_id":"t","message_type":"m","expected_send_at":"2026-05-07T00:00:00Z","expire_at":"2026-05-07T01:00:00Z"}`),
	})
	if err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	if rawConsumer.acked != 1 || rawConsumer.nacked != 0 {
		t.Fatalf("expected ack=1 nack=0, got ack=%d nack=%d", rawConsumer.acked, rawConsumer.nacked)
	}
	if processor.msg == nil {
		t.Fatal("expected processor to receive message")
	}
}

func TestHandleMessageAckOnInvalidPayload(t *testing.T) {
	rawConsumer := &stubConsumer{}
	c := &PulsarConsumer{
		consumer:  rawConsumer,
		processor: &stubProcessor{},
	}

	err := c.handleMessage(context.Background(), &stubMessage{payload: []byte(`{bad`)})
	if err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	if rawConsumer.acked != 1 || rawConsumer.nacked != 0 {
		t.Fatalf("expected ack=1 nack=0, got ack=%d nack=%d", rawConsumer.acked, rawConsumer.nacked)
	}
}

func TestHandleMessageAckOnInvalidRequest(t *testing.T) {
	rawConsumer := &stubConsumer{}
	c := &PulsarConsumer{
		consumer: rawConsumer,
		processor: &stubProcessor{
			err: contract.ErrInvalidRequest,
		},
	}

	err := c.handleMessage(context.Background(), &stubMessage{
		payload: []byte(`{"idempotency_key":"k","tenant_id":"t","message_type":"m","expected_send_at":"2026-05-07T00:00:00Z","expire_at":"2026-05-07T01:00:00Z"}`),
	})
	if err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	if rawConsumer.acked != 1 || rawConsumer.nacked != 0 {
		t.Fatalf("expected ack=1 nack=0, got ack=%d nack=%d", rawConsumer.acked, rawConsumer.nacked)
	}
}

func TestHandleMessageNackOnTemporaryFailure(t *testing.T) {
	rawConsumer := &stubConsumer{}
	c := &PulsarConsumer{
		consumer: rawConsumer,
		processor: &stubProcessor{
			err: contract.ErrTemporaryFailure,
		},
	}

	err := c.handleMessage(context.Background(), &stubMessage{
		payload: []byte(`{"idempotency_key":"k","tenant_id":"t","message_type":"m","expected_send_at":"2026-05-07T00:00:00Z","expire_at":"2026-05-07T01:00:00Z"}`),
	})
	if err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	if rawConsumer.acked != 0 || rawConsumer.nacked != 1 {
		t.Fatalf("expected ack=0 nack=1, got ack=%d nack=%d", rawConsumer.acked, rawConsumer.nacked)
	}
}
