package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"notes/notifysdk"
)

func main() {
	publisher := &demoPublisher{}

	client := notifysdk.New(&notifysdk.MQTransport{
		Topic:     "notification.events",
		Publisher: publisher,
	}, notifysdk.WithOutboxTxStore(&demoOutboxStore{}))

	result, err := client.Send(context.Background(), notifysdk.Command{
		BizType:       "trade",
		EventCode:     "order_paid",
		TemplateCode:  "tpl_order_paid",
		Receivers:     []notifysdk.Receiver{{Type: "wechat", Value: "openid-123"}},
		Payload:       OrderPaidPayload{OrderID: "202604080001", UserID: "u-1", AmountFen: 19900},
		IdempotentKey: "trade:order_paid:202604080001",
		TraceID:       "trace-001",
		Priority:      10,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("accepted=%v mode=%s\n", result.Accepted, result.DeliveryMode)

	reliableResult, err := client.EnqueueInTx(context.Background(), demoTx{}, notifysdk.Command{
		BizType:       "trade",
		EventCode:     "order_paid",
		TemplateCode:  "tpl_order_paid",
		Receivers:     []notifysdk.Receiver{{Type: "wechat", Value: "openid-123"}},
		Payload:       OrderPaidPayload{OrderID: "202604080001", UserID: "u-1", AmountFen: 19900},
		IdempotentKey: "trade:order_paid:202604080001",
		TraceID:       "trace-001",
		Priority:      10,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("queued=%v mode=%s\n", reliableResult.Queued, reliableResult.DeliveryMode)
}

type OrderPaidPayload struct {
	OrderID   string `json:"orderId"`
	UserID    string `json:"userId"`
	AmountFen int64  `json:"amountFen"`
}

func (p OrderPaidPayload) Validate() error {
	if p.OrderID == "" {
		return errors.New("orderId is required")
	}
	if p.UserID == "" {
		return errors.New("userId is required")
	}
	if p.AmountFen <= 0 {
		return errors.New("amountFen must be greater than 0")
	}
	return nil
}

type demoPublisher struct{}

func (demoPublisher) Publish(_ context.Context, topic string, key string, body []byte, headers map[string]string) error {
	fmt.Printf("topic=%s key=%s headers=%v body=%s\n", topic, key, headers, string(body))
	return nil
}

type demoOutboxStore struct{}

func (demoOutboxStore) SaveInTx(_ context.Context, _ notifysdk.Tx, msg notifysdk.OutboxMessage) error {
	fmt.Printf("outbox saved id=%s status=%s next_attempt_at=%s\n", msg.ID, msg.Status, msg.NextAttemptAt.Format("2006-01-02 15:04:05"))
	return nil
}

type demoTx struct{}

func (demoTx) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return demoResult(1), nil
}

type demoResult int64

func (r demoResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r demoResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
