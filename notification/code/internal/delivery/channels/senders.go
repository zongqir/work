package channels

import (
	"net/http"

	"work/notification/code/internal/delivery"
	"work/notification/code/internal/delivery/channels/email"
	"work/notification/code/internal/delivery/channels/sms"
	"work/notification/code/internal/delivery/channels/webhook"
	"work/notification/code/internal/delivery/channels/wecom"
)

func NewSenders(httpClient *http.Client, baseURL string) map[string]delivery.ChannelSender {
	return map[string]delivery.ChannelSender{
		"email":   email.NewSender(httpClient, baseURL),
		"sms":     sms.NewSender(httpClient, baseURL),
		"wecom":   wecom.NewSender(httpClient, baseURL),
		"webhook": webhook.NewSender(httpClient),
	}
}
