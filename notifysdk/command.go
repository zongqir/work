package notifysdk

import (
	"encoding/json"
)

// Payload is the business-specific message body contract. Each notification
// payload should keep its own validation rules close to the struct definition.
type Payload interface {
	Validate() error
}

// Command is the business-facing request model. Callers should populate the
// stable envelope fields and keep business-specific parameters inside Payload.
type Command struct {
	BizType       string            `json:"bizType"`
	EventCode     string            `json:"eventCode"`
	TemplateCode  string            `json:"templateCode"`
	Receivers     []Receiver        `json:"receivers"`
	Payload       Payload           `json:"payload"`
	IdempotentKey string            `json:"idempotentKey"`
	TraceID       string            `json:"traceId,omitempty"`
	Priority      int               `json:"priority,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Meta          map[string]string `json:"meta,omitempty"`
}

type Receiver struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Envelope is the transport-ready representation shared by HTTP, MQ and Outbox.
type Envelope struct {
	BizType       string            `json:"bizType"`
	EventCode     string            `json:"eventCode"`
	TemplateCode  string            `json:"templateCode"`
	Receivers     []Receiver        `json:"receivers"`
	Payload       json.RawMessage   `json:"payload"`
	IdempotentKey string            `json:"idempotentKey"`
	TraceID       string            `json:"traceId,omitempty"`
	Priority      int               `json:"priority,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Meta          map[string]string `json:"meta,omitempty"`
}

func (c Command) Envelope() (Envelope, error) {
	payload, err := json.Marshal(c.Payload)
	if err != nil {
		return Envelope{}, wrapError(ErrMarshalPayload, err)
	}

	return Envelope{
		BizType:       c.BizType,
		EventCode:     c.EventCode,
		TemplateCode:  c.TemplateCode,
		Receivers:     c.Receivers,
		Payload:       payload,
		IdempotentKey: c.IdempotentKey,
		TraceID:       c.TraceID,
		Priority:      c.Priority,
		Headers:       cloneMap(c.Headers),
		Meta:          cloneMap(c.Meta),
	}, nil
}

func cloneMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}

	return dst
}
