package notifysdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPTransport struct {
	Endpoint string
	Client   *http.Client
	Headers  map[string]string
}

func (t *HTTPTransport) Name() Mode {
	return ModeHTTP
}

func (t *HTTPTransport) Send(ctx context.Context, envelope Envelope) (Result, error) {
	if strings.TrimSpace(t.Endpoint) == "" {
		return Result{}, ErrTransportUnavailable
	}

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return Result{}, wrapError(ErrMarshalPayload, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, wrapError(ErrTransportUnavailable, err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range envelope.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return Result{}, wrapError(ErrTransportTimeout, err)
		}
		return Result{}, wrapError(ErrTransportUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("%w: notification http status=%d", ErrTransportUnavailable, resp.StatusCode)
	}

	var payload struct {
		Accepted  *bool  `json:"accepted"`
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil && !errors.Is(err, context.Canceled) {
		return Result{
			Accepted:     true,
			DeliveryMode: ModeHTTP,
		}, nil
	}

	accepted := true
	if payload.Accepted != nil {
		accepted = *payload.Accepted
	}

	return Result{
		Accepted:     accepted,
		RequestID:    payload.RequestID,
		DeliveryMode: ModeHTTP,
	}, nil
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
