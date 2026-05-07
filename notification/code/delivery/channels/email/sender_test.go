package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"work/notification/code/contract"
	"work/notification/code/render"
)

func TestSenderSend(t *testing.T) {
	var captured request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != requestPath {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewSender(server.Client(), server.URL)
	err := sender.Send(context.Background(), &contract.DispatchMessage{
		IdempotencyKey: "msg-1",
	}, render.ChannelPolicy{
		Audience: render.AudienceConfig{
			To:  []string{"a@example.com"},
			Cc:  []string{"b@example.com"},
			Bcc: []string{"c@example.com"},
		},
	}, render.RenderedChannelMessage{
		Channel: "email",
		Email: &render.RenderedEmail{
			Subject: "事件通知",
			Body:    "邮件正文内容...",
		},
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if captured.Subject != "事件通知" || captured.Text != "邮件正文内容..." {
		t.Fatalf("unexpected content: %+v", captured)
	}
	if len(captured.To) != 1 || captured.To[0] != "a@example.com" {
		t.Fatalf("unexpected to: %+v", captured.To)
	}
	if len(captured.Cc) != 1 || captured.Cc[0] != "b@example.com" {
		t.Fatalf("unexpected cc: %+v", captured.Cc)
	}
	if len(captured.Bcc) != 1 || captured.Bcc[0] != "c@example.com" {
		t.Fatalf("unexpected bcc: %+v", captured.Bcc)
	}
	if captured.MessageID != "msg-1" {
		t.Fatalf("unexpected message id: %s", captured.MessageID)
	}
	if len(captured.Attaches) != 0 || len(captured.Inlines) != 0 {
		t.Fatalf("expected empty attachments and inlines: %+v", captured)
	}
}
