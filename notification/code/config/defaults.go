package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"work/notification/code/contract"
)

func LoadDefaultMessageConfigsFromFile(path string) (DefaultMessageConfigLoader, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: default_config_path is required", contract.ErrInvalidRequest)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	byMessageType := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &byMessageType); err != nil {
		return nil, fmt.Errorf("%w: parse default message configs: %v", contract.ErrUnsupportedConfig, err)
	}

	return func(_ context.Context, messageType string) (json.RawMessage, error) {
		raw := byMessageType[messageType]
		if len(raw) == 0 {
			return nil, fmt.Errorf("%w: default config not found: %s", contract.ErrUnsupportedConfig, messageType)
		}
		return raw, nil
	}, nil
}
