package config

import "encoding/json"

type MessageConfig struct {
	Enabled                bool            `json:"enabled"`
	Filter                 json.RawMessage `json:"filter"`
	AggregatePeriodMinutes int             `json:"aggregate_period_minutes"`
}
