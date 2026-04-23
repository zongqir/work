package notifysdk

import "errors"

func hasError(err error, target error) bool {
	return errors.Is(err, target)
}
