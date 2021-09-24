package subscriptions

import (
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

// NotaryRequestEvent represents P2PNotaryRequest event either added or removed
// from notary payload pool.
type NotaryRequestEvent struct {
	Type          mempoolevent.Type         `json:"type"`
	NotaryRequest *payload.P2PNotaryRequest `json:"notaryrequest"`
}
