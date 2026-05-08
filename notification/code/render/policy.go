package render

import "work/notification/code/model"

type EffectivePolicy struct {
	TenantID    string        `json:"tenant_id"`
	MessageType string        `json:"message_type"`
	Channel     ChannelPolicy `json:"channel"`
}

type ChannelPolicy = model.ChannelPolicy
type AudienceConfig = model.AudienceConfig
type DeliveryConfig = model.DeliveryConfig
