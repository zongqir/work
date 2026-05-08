package model

type MessageCapability struct {
	MessageType            string                  `json:"message_type"`
	RealtimeSupported      bool                    `json:"realtime_supported"`
	AggregateSupported     bool                    `json:"aggregate_supported"`
	AggregatePeriodMinutes []int                   `json:"aggregate_period_minutes,omitempty"`
	Channels               []ChannelCapability     `json:"channels"`
	FilterFields           []FilterFieldCapability `json:"filter_fields,omitempty"`
}

type ChannelCapability struct {
	Channel string `json:"channel"`
	Label   string `json:"label,omitempty"`
}

type FilterFieldCapability struct {
	Name     string   `json:"name"`
	Label    string   `json:"label,omitempty"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}

type MessageConfigView struct {
	Capability MessageCapability `json:"capability"`
	Config     MessageConfig     `json:"config"`
}
