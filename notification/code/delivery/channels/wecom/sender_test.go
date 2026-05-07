package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	err := sender.Send(context.Background(), nil, render.ChannelPolicy{
		Audience: render.AudienceConfig{
			Phone: []string{"13111223344"},
		},
		Delivery: render.DeliveryConfig{
			Secret:  "secret-value",
			AgentID: "1000108",
		},
	}, render.RenderedChannelMessage{
		Channel: "wecom",
		WeCom: &render.RenderedWeCom{
			Text: "测试消息",
		},
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if captured.Secret != "secret-value" || captured.AgentID != "1000108" {
		t.Fatalf("unexpected credentials: %+v", captured)
	}
	if len(captured.Phone) != 1 || captured.Phone[0] != "13111223344" {
		t.Fatalf("unexpected phone: %+v", captured.Phone)
	}
	if captured.Text != "测试消息" {
		t.Fatalf("unexpected text: %s", captured.Text)
	}
}
