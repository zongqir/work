package messageconfig

import (
	"context"
	"fmt"

	"work/notification/code/capability"
	"work/notification/code/internal/config"
	"work/notification/code/contract"
	"work/notification/code/model"
)

type Loader struct {
	ConfigLoader *config.MessageConfigLoader
}

func (l *Loader) LoadView(ctx context.Context, tenantID, messageType string) (*model.MessageConfigView, error) {
	if l == nil || l.ConfigLoader == nil {
		return nil, fmt.Errorf("%w: config_loader is required", contract.ErrInvalidRequest)
	}

	cfg, err := l.ConfigLoader.LoadRecord(ctx, tenantID, messageType)
	if err != nil {
		return nil, err
	}
	capItem, err := capability.Get(messageType)
	if err != nil {
		return nil, err
	}
	return &model.MessageConfigView{
		Capability: *capItem,
		Config:     *cfg,
	}, nil
}

func Validate(item *model.MessageConfig) error {
	return capability.ValidateConfig(item)
}
