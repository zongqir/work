package wecom

import (
	"context"
	"fmt"
	"net/http"

	"work/notification/code/contract"
	"work/notification/code/delivery/channels/common"
	"work/notification/code/render"
)

const requestPath = "/xdr/NOTIFICATION/INNER/api/v1/weCom/notification"

type Sender struct {
	client *common.Client
}

type request struct {
	Secret  string   `json:"secret"`
	AgentID string   `json:"agentId"`
	Phone   []string `json:"phone"`
	Text    string   `json:"text"`
}

func NewSender(httpClient *http.Client, baseURL string) *Sender {
	return &Sender{
		client: common.NewClient(httpClient, baseURL),
	}
}

func (s *Sender) Send(ctx context.Context, msg *contract.DispatchMessage, cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) error {
	_ = msg
	req, err := buildRequest(cfg, rendered)
	if err != nil {
		return err
	}
	return s.client.PostJSON(ctx, requestPath, req, nil)
}

func buildRequest(cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) (*request, error) {
	if rendered.WeCom == nil {
		return nil, fmt.Errorf("%w: rendered wecom message is required", contract.ErrInvalidRequest)
	}
	if cfg.Delivery.Secret == "" {
		return nil, fmt.Errorf("%w: wecom delivery.secret is required", contract.ErrInvalidRequest)
	}
	if cfg.Delivery.AgentID == "" {
		return nil, fmt.Errorf("%w: wecom delivery.agent_id is required", contract.ErrInvalidRequest)
	}
	if len(cfg.Audience.Phone) == 0 {
		return nil, fmt.Errorf("%w: wecom audience.phone is required", contract.ErrInvalidRequest)
	}

	return &request{
		Secret:  cfg.Delivery.Secret,
		AgentID: cfg.Delivery.AgentID,
		Phone:   append([]string(nil), cfg.Audience.Phone...),
		Text:    rendered.WeCom.Text,
	}, nil
}
