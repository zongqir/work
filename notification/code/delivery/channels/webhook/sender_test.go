package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"work/notification/code/render"
)

func TestSenderSend(t *testing.T) {
	var capturedBody string
	var capturedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}
		capturedBody = string(data)
		capturedHeader = r.Header.Get("X-Test")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.Client())
	err := sender.Send(context.Background(), nil, render.ChannelPolicy{
		Delivery: render.DeliveryConfig{
			URL: server.URL,
			Headers: map[string]string{
				"X-Test": "1",
			},
		},
	}, render.RenderedChannelMessage{
		Channel: "webhook",
		Webhook: &render.RenderedWebhook{
			Content: "hello webhook",
		},
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if capturedBody != "hello webhook" {
		t.Fatalf("unexpected body: %s", capturedBody)
	}
	if capturedHeader != "1" {
		t.Fatalf("unexpected header: %s", capturedHeader)
	}
}
