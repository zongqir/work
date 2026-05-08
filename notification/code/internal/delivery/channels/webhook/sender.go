package webhook

import (
	"context"
	"fmt"
	"net/http"

	"work/notification/code/internal/delivery/channels/common"
	"work/notification/code/internal/render"
	"work/notification/code/pkg/notification/contract"
)

type Sender struct {
	httpClient *http.Client
}

func NewSender(httpClient *http.Client) *Sender {
	return &Sender{
		httpClient: httpClient,
	}
}

func (s *Sender) Send(ctx context.Context, msg *contract.DispatchMessage, cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) error {
	_ = msg
	if rendered.Webhook == nil {
		return fmt.Errorf("%w: rendered webhook content is required", contract.ErrInvalidRequest)
	}
	if cfg.Delivery.URL == "" {
		return fmt.Errorf("%w: webhook delivery.url is required", contract.ErrInvalidRequest)
	}
	return common.Post(
		ctx,
		s.httpClient,
		cfg.Delivery.URL,
		"text/plain; charset=utf-8",
		[]byte(rendered.Webhook.Content),
		cfg.Delivery.Headers,
	)
}
