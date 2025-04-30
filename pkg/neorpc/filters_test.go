package neorpc

import (
	"testing"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestBlockFilterCopy(t *testing.T) {
	var bf, tf *BlockFilter

	require.Nil(t, bf.Copy())

	bf = new(BlockFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.Primary = new(byte)
	*bf.Primary = 42

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Primary = 100
	require.NotEqual(t, bf, tf)

	bf.Since = new(uint32)
	*bf.Since = 42

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Since = 100500
	require.NotEqual(t, bf, tf)

	bf.Till = new(uint32)
	*bf.Till = 42

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Till = 100500
	require.NotEqual(t, bf, tf)
}

func TestTxFilterCopy(t *testing.T) {
	var bf, tf *TxFilter

	require.Nil(t, bf.Copy())

	bf = new(TxFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.Sender = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Sender = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Signer = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Signer = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)
}

func TestNotificationFilterCopy(t *testing.T) {
	var bf, tf *NotificationFilter

	require.Nil(t, bf.Copy())

	bf = new(NotificationFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.Contract = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Contract = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Name = new(string)
	*bf.Name = "ololo"

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Name = "azaza"
	require.NotEqual(t, bf, tf)

	var err error
	bf.Parameters, err = smartcontract.NewParametersFromValues(1, "2", []byte{3})
	require.NoError(t, err)

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	bf.Parameters[0], bf.Parameters[1] = bf.Parameters[1], bf.Parameters[0]
	require.NotEqual(t, bf, tf)
}

func TestExecutionFilterCopy(t *testing.T) {
	var bf, tf *ExecutionFilter

	require.Nil(t, bf.Copy())

	bf = new(ExecutionFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.State = new(string)
	*bf.State = "ololo"

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.State = "azaza"
	require.NotEqual(t, bf, tf)

	bf.Container = &util.Uint256{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Container = util.Uint256{3, 2, 1}
	require.NotEqual(t, bf, tf)
}

func TestNotaryRequestFilterCopy(t *testing.T) {
	var bf, tf *NotaryRequestFilter

	require.Nil(t, bf.Copy())

	bf = new(NotaryRequestFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.Sender = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Sender = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Signer = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Signer = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Type = new(mempoolevent.Type)
	*bf.Type = mempoolevent.TransactionAdded

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Type = mempoolevent.TransactionRemoved
	require.NotEqual(t, bf, tf)
}

func TestMempoolTransactionFilterCopy(t *testing.T) {
	var bf, tf *MempoolTransactionFilter

	require.Nil(t, bf.Copy())

	bf = new(MempoolTransactionFilter)
	tf = bf.Copy()
	require.Equal(t, bf, tf)

	bf.Sender = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Sender = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Signer = &util.Uint160{1, 2, 3}

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Signer = util.Uint160{3, 2, 1}
	require.NotEqual(t, bf, tf)

	bf.Type = new(mempoolevent.Type)
	*bf.Type = mempoolevent.TransactionAdded

	tf = bf.Copy()
	require.Equal(t, bf, tf)
	*bf.Type = mempoolevent.TransactionRemoved
	require.NotEqual(t, bf, tf)
}

func TestMempoolTransactionFilterIsValid(t *testing.T) {
	filter := MempoolTransactionFilter{}
	require.NoError(t, filter.IsValid())

	filter.Sender = &util.Uint160{1, 2, 3}
	require.NoError(t, filter.IsValid())

	filter.Signer = &util.Uint160{4, 5, 6}
	require.NoError(t, filter.IsValid())

	mempoolType := mempoolevent.TransactionAdded
	filter.Type = &mempoolType
	require.NoError(t, filter.IsValid())

	mempoolType = mempoolevent.TransactionRemoved
	require.NoError(t, filter.IsValid())
}

func TestMempoolTransactionFilterJSON(t *testing.T) {
	// Test unmarshaling and marshaling of MempoolTransactionFilter
	jsonStr := `{
		"sender": "0x0102030000000000000000000000000000000000",
		"signer": "0x0405060000000000000000000000000000000000",
		"type": 1
	}`

	var filter MempoolTransactionFilter
	err := json.Unmarshal([]byte(jsonStr), &filter)
	require.NoError(t, err)

	// Verify the unmarshaled values
	require.NotNil(t, filter.Sender)
	require.Equal(t, util.Uint160{1, 2, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, *filter.Sender)
	require.NotNil(t, filter.Signer)
	require.Equal(t, util.Uint160{4, 5, 6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, *filter.Signer)
	require.NotNil(t, filter.Type)
	require.Equal(t, mempoolevent.TransactionAdded, *filter.Type)

	// Marshal back to JSON and verify
	data, err := json.Marshal(filter)
	require.NoError(t, err)
	
	// Unmarshal again to verify the round-trip
	var filter2 MempoolTransactionFilter
	err = json.Unmarshal(data, &filter2)
	require.NoError(t, err)
	require.Equal(t, filter, filter2)

	// Test with empty filter
	emptyFilter := MempoolTransactionFilter{}
	emptyJSON, err := json.Marshal(emptyFilter)
	require.NoError(t, err)
	require.Equal(t, "{}", string(emptyJSON))
}
