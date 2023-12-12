package rpcevent

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

type (
	testComparator struct {
		id     neorpc.EventID
		filter any
	}
	testContainer struct {
		id  neorpc.EventID
		pld any
	}
)

func (c testComparator) EventID() neorpc.EventID {
	return c.id
}
func (c testComparator) Filter() any {
	return c.filter
}
func (c testContainer) EventID() neorpc.EventID {
	return c.id
}
func (c testContainer) EventPayload() any {
	return c.pld
}

func TestMatches(t *testing.T) {
	primary := byte(1)
	badPrimary := byte(2)
	index := uint32(5)
	badHigherIndex := uint32(6)
	badLowerIndex := index - 1
	sender := util.Uint160{1, 2, 3}
	signer := util.Uint160{4, 5, 6}
	contract := util.Uint160{7, 8, 9}
	notaryType := mempoolevent.TransactionAdded
	badUint160 := util.Uint160{9, 9, 9}
	cnt := util.Uint256{1, 2, 3}
	badUint256 := util.Uint256{9, 9, 9}
	name := "ntf name"
	badName := "bad name"
	badType := mempoolevent.TransactionRemoved
	bContainer := testContainer{
		id: neorpc.BlockEventID,
		pld: &block.Block{
			Header: block.Header{PrimaryIndex: byte(primary), Index: index},
		},
	}
	headerContainer := testContainer{
		id:  neorpc.HeaderOfAddedBlockEventID,
		pld: &block.Header{PrimaryIndex: byte(primary), Index: index},
	}
	st := vmstate.Halt
	goodState := st.String()
	badState := "FAULT"
	txContainer := testContainer{
		id:  neorpc.TransactionEventID,
		pld: &transaction.Transaction{Signers: []transaction.Signer{{Account: sender}, {Account: signer}}},
	}
	ntfContainer := testContainer{
		id:  neorpc.NotificationEventID,
		pld: &state.ContainedNotificationEvent{NotificationEvent: state.NotificationEvent{ScriptHash: contract, Name: name}},
	}
	exContainer := testContainer{
		id:  neorpc.ExecutionEventID,
		pld: &state.AppExecResult{Container: cnt, Execution: state.Execution{VMState: st}},
	}
	ntrContainer := testContainer{
		id: neorpc.NotaryRequestEventID,
		pld: &result.NotaryRequestEvent{
			Type: notaryType,
			NotaryRequest: &payload.P2PNotaryRequest{
				MainTransaction:     &transaction.Transaction{Signers: []transaction.Signer{{Account: signer}}},
				FallbackTransaction: &transaction.Transaction{Signers: []transaction.Signer{{Account: util.Uint160{}}, {Account: sender}}},
			},
		},
	}
	missedContainer := testContainer{
		id: neorpc.MissedEventID,
	}
	var testCases = []struct {
		name       string
		comparator testComparator
		container  testContainer
		expected   bool
	}{
		{
			name:       "ID mismatch",
			comparator: testComparator{id: neorpc.TransactionEventID},
			container:  bContainer,
			expected:   false,
		},
		{
			name:       "missed event",
			comparator: testComparator{id: neorpc.BlockEventID},
			container:  missedContainer,
			expected:   false,
		},
		{
			name:       "block, no filter",
			comparator: testComparator{id: neorpc.BlockEventID},
			container:  bContainer,
			expected:   true,
		},
		{
			name: "block, primary mismatch",
			comparator: testComparator{
				id:     neorpc.BlockEventID,
				filter: neorpc.BlockFilter{Primary: &badPrimary},
			},
			container: bContainer,
			expected:  false,
		},
		{
			name: "block, since mismatch",
			comparator: testComparator{
				id:     neorpc.BlockEventID,
				filter: neorpc.BlockFilter{Since: &badHigherIndex},
			},
			container: bContainer,
			expected:  false,
		},
		{
			name: "block, till mismatch",
			comparator: testComparator{
				id:     neorpc.BlockEventID,
				filter: neorpc.BlockFilter{Till: &badLowerIndex},
			},
			container: bContainer,
			expected:  false,
		},
		{
			name: "block, filter match",
			comparator: testComparator{
				id:     neorpc.BlockEventID,
				filter: neorpc.BlockFilter{Primary: &primary, Since: &index, Till: &index},
			},
			container: bContainer,
			expected:  true,
		},
		{
			name:       "header, no filter",
			comparator: testComparator{id: neorpc.HeaderOfAddedBlockEventID},
			container:  headerContainer,
			expected:   true,
		},
		{
			name: "header, primary mismatch",
			comparator: testComparator{
				id:     neorpc.HeaderOfAddedBlockEventID,
				filter: neorpc.BlockFilter{Primary: &badPrimary},
			},
			container: headerContainer,
			expected:  false,
		},
		{
			name: "header, since mismatch",
			comparator: testComparator{
				id:     neorpc.HeaderOfAddedBlockEventID,
				filter: neorpc.BlockFilter{Since: &badHigherIndex},
			},
			container: headerContainer,
			expected:  false,
		},
		{
			name: "header, till mismatch",
			comparator: testComparator{
				id:     neorpc.HeaderOfAddedBlockEventID,
				filter: neorpc.BlockFilter{Till: &badLowerIndex},
			},
			container: headerContainer,
			expected:  false,
		},
		{
			name: "header, filter match",
			comparator: testComparator{
				id:     neorpc.HeaderOfAddedBlockEventID,
				filter: neorpc.BlockFilter{Primary: &primary, Since: &index, Till: &index},
			},
			container: headerContainer,
			expected:  true,
		},
		{
			name:       "transaction, no filter",
			comparator: testComparator{id: neorpc.TransactionEventID},
			container:  txContainer,
			expected:   true,
		},
		{
			name: "transaction, sender mismatch",
			comparator: testComparator{
				id:     neorpc.TransactionEventID,
				filter: neorpc.TxFilter{Sender: &badUint160},
			},
			container: txContainer,
			expected:  false,
		},
		{
			name: "transaction, signer mismatch",
			comparator: testComparator{
				id:     neorpc.TransactionEventID,
				filter: neorpc.TxFilter{Signer: &badUint160},
			},
			container: txContainer,
			expected:  false,
		},
		{
			name: "transaction, filter match",
			comparator: testComparator{
				id:     neorpc.TransactionEventID,
				filter: neorpc.TxFilter{Sender: &sender, Signer: &signer},
			},
			container: txContainer,
			expected:  true,
		},
		{
			name:       "notification, no filter",
			comparator: testComparator{id: neorpc.NotificationEventID},
			container:  ntfContainer,
			expected:   true,
		},
		{
			name: "notification, contract mismatch",
			comparator: testComparator{
				id:     neorpc.NotificationEventID,
				filter: neorpc.NotificationFilter{Contract: &badUint160},
			},
			container: ntfContainer,
			expected:  false,
		},
		{
			name: "notification, name mismatch",
			comparator: testComparator{
				id:     neorpc.NotificationEventID,
				filter: neorpc.NotificationFilter{Name: &badName},
			},
			container: ntfContainer,
			expected:  false,
		},
		{
			name: "notification, filter match",
			comparator: testComparator{
				id:     neorpc.NotificationEventID,
				filter: neorpc.NotificationFilter{Name: &name, Contract: &contract},
			},
			container: ntfContainer,
			expected:  true,
		},
		{
			name:       "execution, no filter",
			comparator: testComparator{id: neorpc.ExecutionEventID},
			container:  exContainer,
			expected:   true,
		},
		{
			name: "execution, state mismatch",
			comparator: testComparator{
				id:     neorpc.ExecutionEventID,
				filter: neorpc.ExecutionFilter{State: &badState},
			},
			container: exContainer,
			expected:  false,
		},
		{
			name: "execution, container mismatch",
			comparator: testComparator{
				id:     neorpc.ExecutionEventID,
				filter: neorpc.ExecutionFilter{Container: &badUint256},
			},
			container: exContainer,
			expected:  false,
		},
		{
			name: "execution, filter mismatch",
			comparator: testComparator{
				id:     neorpc.ExecutionEventID,
				filter: neorpc.ExecutionFilter{State: &goodState, Container: &cnt},
			},
			container: exContainer,
			expected:  true,
		},
		{
			name:       "notary request, no filter",
			comparator: testComparator{id: neorpc.NotaryRequestEventID},
			container:  ntrContainer,
			expected:   true,
		},
		{
			name: "notary request, sender mismatch",
			comparator: testComparator{
				id:     neorpc.NotaryRequestEventID,
				filter: neorpc.NotaryRequestFilter{Sender: &badUint160},
			},
			container: ntrContainer,
			expected:  false,
		},
		{
			name: "notary request, signer mismatch",
			comparator: testComparator{
				id:     neorpc.NotaryRequestEventID,
				filter: neorpc.NotaryRequestFilter{Signer: &badUint160},
			},
			container: ntrContainer,
			expected:  false,
		},
		{
			name: "notary request, type mismatch",
			comparator: testComparator{
				id:     neorpc.NotaryRequestEventID,
				filter: neorpc.NotaryRequestFilter{Type: &badType},
			},
			container: ntrContainer,
			expected:  false,
		},
		{
			name: "notary request, filter match",
			comparator: testComparator{
				id:     neorpc.NotaryRequestEventID,
				filter: neorpc.NotaryRequestFilter{Sender: &sender, Signer: &signer, Type: &notaryType},
			},
			container: ntrContainer,
			expected:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, Matches(tc.comparator, tc.container))
		})
	}
}
