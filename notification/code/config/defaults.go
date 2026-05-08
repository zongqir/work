package config

import (
	"encoding/json"
	"fmt"
	"os"

	"work/notification/code/contract"
	"work/notification/code/model"
)

func LoadDefaultMessageConfigsFromFile(path string) ([]model.MessageConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: default_config_path is required", contract.ErrInvalidRequest)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []model.MessageConfig
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("%w: parse default message configs: %v", contract.ErrUnsupportedConfig, err)
	}

	return items, nil
}
