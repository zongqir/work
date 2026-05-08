package capability

import (
	"encoding/json"
	"time"
)

type MessageCapability struct {
	MessageType            string              `json:"message_type"`
	RealtimeSupported      bool                `json:"realtime_supported"`
	AggregateSupported     bool                `json:"aggregate_supported"`
	AggregatePeriodMinutes []int               `json:"aggregate_period_minutes,omitempty"`
	Channels               []ChannelCapability `json:"channels"`
}

type ChannelCapability struct {
	Channel string `json:"channel"`
	Label   string `json:"label,omitempty"`
}

type MessageConfig struct {
	TenantID               string          `json:"tenant_id,omitempty"`
	MessageType            string          `json:"message_type"`
	RealtimeEnabled        bool            `json:"realtime_enabled"`
	AggregateEnabled       bool            `json:"aggregate_enabled"`
	AggregatePeriodMinutes int             `json:"aggregate_period_minutes"`
	Filter                 json.RawMessage `json:"filter,omitempty"`
	Channel                ChannelPolicy   `json:"channel"`
	UpdatedBy              string          `json:"updated_by,omitempty"`
	UpdatedAt              time.Time       `json:"updated_at,omitempty"`
}

type ChannelPolicy struct {
	Channel      string         `json:"channel"`
	TemplateCode string         `json:"template_code,omitempty"`
	TemplateKey  string         `json:"template_key,omitempty"`
	Audience     AudienceConfig `json:"audience,omitempty"`
	Delivery     DeliveryConfig `json:"delivery,omitempty"`
}

type AudienceConfig struct {
	To         []string `json:"to,omitempty"`
	Cc         []string `json:"cc,omitempty"`
	Bcc        []string `json:"bcc,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Phone      []string `json:"phone,omitempty"`
}

type DeliveryConfig struct {
	Platform string            `json:"platform,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	AgentID  string            `json:"agent_id,omitempty"`
	URL      string            `json:"url,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

type MessageConfigView struct {
	Capability MessageCapability `json:"capability"`
	Config     MessageConfig     `json:"config"`
}
