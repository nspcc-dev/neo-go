package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
)

// MempoolEvent represents a transaction event either added to
// or removed from the mempool.
type MempoolEvent struct {
	Type mempoolevent.Type        `json:"type"`
	Tx   *transaction.Transaction `json:"transaction"`
}
