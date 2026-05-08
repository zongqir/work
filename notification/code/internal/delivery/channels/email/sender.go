package email

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"work/notification/code/internal/delivery/channels/common"
	"work/notification/code/internal/render"
	"work/notification/code/pkg/notification/contract"
)

const requestPath = "/xdr/NOTIFICATION/INNER/api/v1/mail/notification"

type Sender struct {
	httpClient *http.Client
	baseURL    string
}

type request struct {
	Subject   string   `json:"subject"`
	Text      string   `json:"text"`
	To        []string `json:"to"`
	Cc        []string `json:"cc"`
	Bcc       []string `json:"bcc"`
	Attaches  []any    `json:"attaches"`
	Inlines   []any    `json:"inlines"`
	MessageID string   `json:"messageId"`
}

func NewSender(httpClient *http.Client, baseURL string) *Sender {
	return &Sender{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

func (s *Sender) Send(ctx context.Context, msg *contract.DispatchMessage, cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) error {
	req, err := buildRequest(msg, cfg, rendered)
	if err != nil {
		return err
	}
	return common.PostJSON(ctx, s.httpClient, s.requestURL(requestPath), req, nil)
}

func (s *Sender) requestURL(path string) string {
	if s.baseURL == "" {
		return path
	}
	return s.baseURL + "/" + strings.TrimLeft(path, "/")
}

func buildRequest(msg *contract.DispatchMessage, cfg render.ChannelPolicy, rendered render.RenderedChannelMessage) (*request, error) {
	if msg == nil {
		return nil, fmt.Errorf("%w: dispatch message is required", contract.ErrInvalidRequest)
	}
	if rendered.Email == nil {
		return nil, fmt.Errorf("%w: rendered email is required", contract.ErrInvalidRequest)
	}
	if len(cfg.Audience.To) == 0 {
		return nil, fmt.Errorf("%w: email audience.to is required", contract.ErrInvalidRequest)
	}

	return &request{
		Subject:   rendered.Email.Subject,
		Text:      rendered.Email.Body,
		To:        append([]string(nil), cfg.Audience.To...),
		Cc:        append([]string{}, cfg.Audience.Cc...),
		Bcc:       append([]string{}, cfg.Audience.Bcc...),
		Attaches:  []any{},
		Inlines:   []any{},
		MessageID: msg.IdempotencyKey,
	}, nil
}
