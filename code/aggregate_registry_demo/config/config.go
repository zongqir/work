package config

import "encoding/json"

type MessageConfig struct {
	RealtimeEnabled        bool            `json:"realtime_enabled"`
	AggregateEnabled       bool            `json:"aggregate_enabled"`
	Filter                 json.RawMessage `json:"filter"`
	AggregatePeriodMinutes int             `json:"aggregate_period_minutes"`
}
