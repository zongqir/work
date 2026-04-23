package notifysdk

import (
	"fmt"
	"strings"
)

type Validator interface {
	Validate(Command) error
}

type DefaultValidator struct{}

func (DefaultValidator) Validate(cmd Command) error {
	if strings.TrimSpace(cmd.BizType) == "" {
		return fmt.Errorf("%w: bizType is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(cmd.EventCode) == "" {
		return fmt.Errorf("%w: eventCode is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(cmd.TemplateCode) == "" {
		return fmt.Errorf("%w: templateCode is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(cmd.IdempotentKey) == "" {
		return fmt.Errorf("%w: idempotentKey is required", ErrInvalidArgument)
	}
	if cmd.Payload == nil {
		return fmt.Errorf("%w: payload is required", ErrInvalidArgument)
	}
	if len(cmd.Receivers) == 0 {
		return fmt.Errorf("%w: at least one receiver is required", ErrInvalidArgument)
	}
	for i, receiver := range cmd.Receivers {
		if strings.TrimSpace(receiver.Type) == "" {
			return fmt.Errorf("%w: receivers[%d].type is required", ErrInvalidArgument, i)
		}
		if strings.TrimSpace(receiver.Value) == "" {
			return fmt.Errorf("%w: receivers[%d].value is required", ErrInvalidArgument, i)
		}
	}
	if err := cmd.Payload.Validate(); err != nil {
		return fmt.Errorf("%w: payload validation failed: %v", ErrInvalidArgument, err)
	}

	return nil
}
