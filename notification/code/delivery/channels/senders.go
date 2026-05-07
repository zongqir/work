package channels

import (
	"net/http"

	"work/notification/code/delivery"
	"work/notification/code/delivery/channels/email"
	"work/notification/code/delivery/channels/sms"
	"work/notification/code/delivery/channels/webhook"
	"work/notification/code/delivery/channels/wecom"
)

func NewSenders(httpClient *http.Client, baseURL string) map[string]delivery.ChannelSender {
	return map[string]delivery.ChannelSender{
		"email":   email.NewSender(httpClient, baseURL),
		"sms":     sms.NewSender(httpClient, baseURL),
		"wecom":   wecom.NewSender(httpClient, baseURL),
		"webhook": webhook.NewSender(httpClient),
	}
}
