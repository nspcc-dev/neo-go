package rpcevent

import (
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type (
	// Comparator is an interface required from notification event filter to be able to
	// filter notifications.
	Comparator interface {
		EventID() neorpc.EventID
		Filter() neorpc.SubscriptionFilter
	}
	// Container is an interface required from notification event to be able to
	// pass filter.
	Container interface {
		EventID() neorpc.EventID
		EventPayload() any
	}
)

// Matches filters our given Container against Comparator filter.
func Matches(f Comparator, r Container) bool {
	expectedEvent := f.EventID()
	filter := f.Filter()
	if r.EventID() != expectedEvent {
		return false
	}
	if filter == nil {
		return true
	}
	switch f.EventID() {
	case neorpc.BlockEventID, neorpc.HeaderOfAddedBlockEventID:
		filt := filter.(neorpc.BlockFilter)
		var b *block.Header
		if f.EventID() == neorpc.HeaderOfAddedBlockEventID {
			b = r.EventPayload().(*block.Header)
		} else {
			b = &r.EventPayload().(*block.Block).Header
		}
		primaryOk := filt.Primary == nil || *filt.Primary == b.PrimaryIndex
		sinceOk := filt.Since == nil || *filt.Since <= b.Index
		tillOk := filt.Till == nil || b.Index <= *filt.Till
		return primaryOk && sinceOk && tillOk
	case neorpc.TransactionEventID:
		filt := filter.(neorpc.TxFilter)
		tx := r.EventPayload().(*transaction.Transaction)
		senderOK := filt.Sender == nil || tx.Sender().Equals(*filt.Sender)
		signerOK := true
		if filt.Signer != nil {
			signerOK = false
			for i := range tx.Signers {
				if tx.Signers[i].Account.Equals(*filt.Signer) {
					signerOK = true
					break
				}
			}
		}
		return senderOK && signerOK
	case neorpc.NotificationEventID:
		filt := filter.(neorpc.NotificationFilter)
		notification := r.EventPayload().(*state.ContainedNotificationEvent)
		hashOk := filt.Contract == nil || notification.ScriptHash.Equals(*filt.Contract)
		nameOk := filt.Name == nil || notification.Name == *filt.Name
		parametersOk := true
		if len(filt.Parameters) > 0 {
			stackItems := notification.Item.Value().([]stackitem.Item)
			parameters, err := filt.ParametersAsStackItems()
			if err != nil {
				return false
			}
			if len(parameters) > len(stackItems) {
				return false
			}
			for i, p := range parameters {
				if p.Type() == stackitem.AnyT && p.Value() == nil {
					continue
				}
				if !p.Equals(stackItems[i]) {
					parametersOk = false
					break
				}
			}
		}
		return hashOk && nameOk && parametersOk
	case neorpc.ExecutionEventID:
		filt := filter.(neorpc.ExecutionFilter)
		applog := r.EventPayload().(*state.AppExecResult)
		stateOK := filt.State == nil || applog.VMState.String() == *filt.State
		containerOK := filt.Container == nil || applog.Container.Equals(*filt.Container)
		return stateOK && containerOK
	case neorpc.NotaryRequestEventID:
		filt := filter.(neorpc.NotaryRequestFilter)
		req := r.EventPayload().(*result.NotaryRequestEvent)
		typeOk := filt.Type == nil || req.Type == *filt.Type
		senderOk := filt.Sender == nil || req.NotaryRequest.FallbackTransaction.Signers[1].Account == *filt.Sender
		signerOK := true
		if filt.Signer != nil {
			signerOK = false
			for _, signer := range req.NotaryRequest.MainTransaction.Signers {
				if signer.Account.Equals(*filt.Signer) {
					signerOK = true
					break
				}
			}
		}
		return senderOk && signerOK && typeOk
	case neorpc.MempoolEventID:
		filt := filter.(neorpc.MempoolEventFilter)
		memEvent := r.EventPayload().(*result.MempoolEvent)
		if filt.Type != nil && memEvent.Type != *filt.Type {
			return false
		}
		if filt.Sender != nil && !memEvent.Tx.Sender().Equals(*filt.Sender) {
			return false
		}
		if filt.Signer != nil {
			for _, signer := range memEvent.Tx.Signers {
				if signer.Account.Equals(*filt.Signer) {
					return true
				}
			}
			return false
		}
		return true
	default:
		return false
	}
}
