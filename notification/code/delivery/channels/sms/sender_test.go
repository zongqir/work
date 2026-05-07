package sms

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
		TemplateKey: "commonTemplate",
		Audience: render.AudienceConfig{
			Recipients: []string{"13111223344"},
		},
		Delivery: render.DeliveryConfig{
			Platform: "ali",
		},
	}, render.RenderedChannelMessage{
		Channel: "sms",
		SMS: &render.RenderedSMS{
			TemplateParams: []render.RenderedParam{
				{ParamName: "code", ParamValue: "123456"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if captured.TemplateKey != "commonTemplate" {
		t.Fatalf("unexpected template key: %s", captured.TemplateKey)
	}
	if len(captured.Recipients) != 1 || captured.Recipients[0] != "13111223344" {
		t.Fatalf("unexpected recipients: %+v", captured.Recipients)
	}
	if len(captured.TemplateParams) != 1 || captured.TemplateParams[0].ParamName != "code" || captured.TemplateParams[0].ParamValue != "123456" {
		t.Fatalf("unexpected template params: %+v", captured.TemplateParams)
	}
	if captured.Platform != "ali" {
		t.Fatalf("unexpected platform: %s", captured.Platform)
	}
}
