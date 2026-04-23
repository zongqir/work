package notifysdk

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidArgument      = errors.New("invalid notification command")
	ErrMarshalPayload       = errors.New("marshal payload")
	ErrTransportUnavailable = errors.New("transport unavailable")
	ErrTransportTimeout     = errors.New("transport timeout")
	ErrEnqueueFailed        = errors.New("enqueue failed")
	ErrOutboxNotConfigured  = errors.New("outbox tx store not configured")
	ErrOutboxNotFound       = errors.New("outbox message not found")
)

func wrapError(base error, err error) error {
	if err == nil {
		return base
	}

	return fmt.Errorf("%w: %v", base, err)
}
