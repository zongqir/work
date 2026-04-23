package notifysdk

import (
	"context"
	"database/sql"
)

// Tx is the minimal transaction contract needed by the outbox store. *sql.Tx
// satisfies this interface directly.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
