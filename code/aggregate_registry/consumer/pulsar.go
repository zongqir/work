package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"notes/code/aggregate_registry/contract"
)

type Processor interface {
	Process(ctx context.Context, msg *contract.DispatchMessage) error
}

type PulsarConsumer struct {
	consumer  pulsar.Consumer
	processor Processor
	LogError  func(ctx context.Context, msg string, err error)
}

func NewPulsarConsumer(client pulsar.Client, topic, subscription string, processor Processor) (*PulsarConsumer, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: pulsar client is required", contract.ErrInvalidRequest)
	}
	if topic == "" {
		return nil, fmt.Errorf("%w: topic is required", contract.ErrInvalidRequest)
	}
	if subscription == "" {
		return nil, fmt.Errorf("%w: subscription is required", contract.ErrInvalidRequest)
	}
	if processor == nil {
		return nil, fmt.Errorf("%w: processor is required", contract.ErrInvalidRequest)
	}

	rawConsumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscription,
		Type:             pulsar.Shared,
	})
	if err != nil {
		return nil, err
	}

	return &PulsarConsumer{
		consumer:  rawConsumer,
		processor: processor,
	}, nil
}

func (c *PulsarConsumer) Run(ctx context.Context) error {
	if c == nil || c.consumer == nil {
		return fmt.Errorf("%w: pulsar consumer is required", contract.ErrInvalidRequest)
	}
	if c.processor == nil {
		return fmt.Errorf("%w: processor is required", contract.ErrInvalidRequest)
	}

	for {
		message, err := c.consumer.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if err := c.handleMessage(ctx, message); err != nil {
			return err
		}
	}
}

func (c *PulsarConsumer) Close() {
	if c == nil || c.consumer == nil {
		return
	}
	c.consumer.Close()
}

func (c *PulsarConsumer) handleMessage(ctx context.Context, message pulsar.Message) error {
	var dispatchMessage contract.DispatchMessage
	if err := json.Unmarshal(message.Payload(), &dispatchMessage); err != nil {
		if c.LogError != nil {
			c.LogError(ctx, "decode dispatch message failed", err)
		}
		return c.consumer.Ack(message)
	}

	err := c.processor.Process(ctx, &dispatchMessage)
	if err == nil {
		return c.consumer.Ack(message)
	}
	if errors.Is(err, contract.ErrInvalidRequest) {
		if c.LogError != nil {
			c.LogError(ctx, "drop invalid dispatch message", err)
		}
		return c.consumer.Ack(message)
	}

	if c.LogError != nil {
		c.LogError(ctx, "process dispatch message failed", err)
	}
	c.consumer.Nack(message)
	return nil
}
