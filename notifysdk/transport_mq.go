package notifysdk

import (
	"context"
	"encoding/json"
)

// MQPublisher is the adapter point for the team's existing MQ send library.
type MQPublisher interface {
	Publish(ctx context.Context, topic string, key string, body []byte, headers map[string]string) error
}

type MQTransport struct {
	Topic     string
	Publisher MQPublisher
}

func (t *MQTransport) Name() Mode {
	return ModeMQ
}

func (t *MQTransport) Send(ctx context.Context, envelope Envelope) (Result, error) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return Result{}, wrapError(ErrMarshalPayload, err)
	}

	if err := t.Publisher.Publish(ctx, t.Topic, envelope.IdempotentKey, body, envelope.Headers); err != nil {
		return Result{}, wrapError(ErrTransportUnavailable, err)
	}

	return Result{
		Accepted:     true,
		DeliveryMode: ModeMQ,
	}, nil
}
