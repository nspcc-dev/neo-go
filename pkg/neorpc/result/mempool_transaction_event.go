package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
)

// MempoolTransactionEvent represents a transaction event either added to
// or removed from the mempool.
type MempoolTransactionEvent struct {
	Type        mempoolevent.Type       `json:"type"`
	Transaction *transaction.Transaction `json:"transaction"`
}