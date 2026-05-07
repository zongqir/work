package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/apache/pulsar-client-go/pulsar"
	"work/notification/code/contract"
)

type PulsarPublisher struct {
	client   pulsar.Client
	topic    string
	mu       sync.Mutex
	producer pulsar.Producer
}

func NewPulsarPublisher(client pulsar.Client, topic string) (*PulsarPublisher, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: pulsar client is required", contract.ErrInvalidRequest)
	}
	if topic == "" {
		return nil, fmt.Errorf("%w: topic is required", contract.ErrInvalidRequest)
	}

	return &PulsarPublisher{
		client: client,
		topic:  topic,
	}, nil
}

func (p *PulsarPublisher) Publish(ctx context.Context, msg *contract.DispatchMessage) error {
	if p == nil {
		return fmt.Errorf("%w: pulsar publisher is required", contract.ErrInvalidRequest)
	}
	if msg == nil {
		return fmt.Errorf("%w: dispatch message is nil", contract.ErrInvalidRequest)
	}

	producer, err := p.getProducer()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = producer.Send(ctx, &pulsar.ProducerMessage{
		Payload: payload,
		Key:     msg.TenantID,
	})
	return err
}

func (p *PulsarPublisher) Close() {
	if p == nil {
		return
	}

	p.mu.Lock()
	producer := p.producer
	p.producer = nil
	p.mu.Unlock()

	if producer != nil {
		producer.Close()
	}
}

func (p *PulsarPublisher) getProducer() (pulsar.Producer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.producer != nil {
		return p.producer, nil
	}

	producer, err := p.client.CreateProducer(pulsar.ProducerOptions{
		Topic: p.topic,
	})
	if err != nil {
		return nil, err
	}
	p.producer = producer
	return p.producer, nil
}
