package publisher

import (
	"testing"
	"time"

	"work/notification/code/contract"
)

func TestBuildProducerMessageUsesExpectedSendAtAsDeliverAt(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	expectedSendAt := now.Add(2 * time.Minute)

	producerMessage, err := buildProducerMessage(&contract.DispatchMessage{
		IdempotencyKey: "k",
		TenantID:       "t_1",
		MessageType:    "sample_both",
		ExpectedSendAt: expectedSendAt,
	}, now)
	if err != nil {
		t.Fatalf("buildProducerMessage failed: %v", err)
	}
	if !producerMessage.DeliverAt.Equal(expectedSendAt) {
		t.Fatalf("expected deliver_at=%v, got %v", expectedSendAt, producerMessage.DeliverAt)
	}
}

func TestBuildProducerMessageDoesNotDelayDueMessage(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)

	producerMessage, err := buildProducerMessage(&contract.DispatchMessage{
		IdempotencyKey: "k",
		TenantID:       "t_1",
		MessageType:    "sample_both",
		ExpectedSendAt: now,
	}, now)
	if err != nil {
		t.Fatalf("buildProducerMessage failed: %v", err)
	}
	if !producerMessage.DeliverAt.IsZero() {
		t.Fatalf("expected zero deliver_at, got %v", producerMessage.DeliverAt)
	}
}
