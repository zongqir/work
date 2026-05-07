package sms

import (
	"context"
	"fmt"
	"net/http"

	"work/notification/code/contract"
	"work/notification/code/delivery/channels/common"
	"work/notification/code/render"
)

const requestPath = "/xdr/NOTIFICATION/INNER/api/v1/sms/notification"

type Sender struct {
	client *common.Client
}

type request struct {
	TemplateKey    string               `json:"templateKey"`
	Recipients     []string             `json:"recipients"`
	TemplateParams []templateParamEntry `json:"templateParams"`
	Platform       string               `json:"platform"`
}

type templateParamEntry struct {
	ParamName  string `json:"paramName"`
	ParamValue string `json:"paramValue"`
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
	if rendered.SMS == nil {
		return nil, fmt.Errorf("%w: rendered sms is required", contract.ErrInvalidRequest)
	}
	if cfg.TemplateKey == "" {
		return nil, fmt.Errorf("%w: sms template key is required", contract.ErrInvalidRequest)
	}
	if len(cfg.Audience.Recipients) == 0 {
		return nil, fmt.Errorf("%w: sms audience.recipients is required", contract.ErrInvalidRequest)
	}
	if cfg.Delivery.Platform == "" {
		return nil, fmt.Errorf("%w: sms delivery.platform is required", contract.ErrInvalidRequest)
	}

	params := make([]templateParamEntry, 0, len(rendered.SMS.TemplateParams))
	for _, item := range rendered.SMS.TemplateParams {
		params = append(params, templateParamEntry{
			ParamName:  item.ParamName,
			ParamValue: item.ParamValue,
		})
	}

	return &request{
		TemplateKey:    cfg.TemplateKey,
		Recipients:     append([]string(nil), cfg.Audience.Recipients...),
		TemplateParams: params,
		Platform:       cfg.Delivery.Platform,
	}, nil
}
