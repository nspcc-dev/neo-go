package integration

import (
	"math/rand"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/stretchr/testify/require"
)

// Generator goal is to generate tx for testing purposes.
type Generator struct {}

// getTransactions returns Invocation txes.
func getTransactions(t require.TestingT, numberOfTX int) []*transaction.Transaction{
	var data []*transaction.Transaction

	wif, err := getWif()
	require.NoError(t, err)

	for n := 0; n < numberOfTX; n++ {
		tx := getTX(t, wif)
		require.NoError(t, rpc.SignTx(tx, wif))
		data = append(data, tx)
	}
	return data
}

// getWif returns Wif.
func getWif() (*keys.WIF, error) {
	var (
		wifEncoded = "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o"
		version    = byte(0x00)
	)
	return keys.WIFDecode(wifEncoded, version)
}

// getTX returns Invocation transaction with some random attributes in order to have different hashes.
func getTX(t require.TestingT, wif *keys.WIF) *transaction.Transaction {
	fromAddress := wif.PrivateKey.Address()
	fromAddressHash, err := crypto.Uint160DecodeAddress(fromAddress)
	require.NoError(t, err)

	tx := &transaction.Transaction{
		Type:    transaction.InvocationType,
		Version: 1,
		Data: &transaction.InvocationTX{
			Script:  []byte{0x51},
			Gas:     1,
			Version: 1,
		},
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
		Trimmed:    false,
	}
	tx.Attributes = append(tx.Attributes,
		transaction.Attribute{
			Usage: transaction.Description,
			Data:  []byte(randString(16)),
		})
	tx.Attributes = append(tx.Attributes,
		transaction.Attribute{
			Usage: transaction.Script,
			Data:  fromAddressHash.BytesBE(),
		})
	return tx
}

// String returns a random string with the n as its length.
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(Int(32, 126))
	}

	return string(b)
}

// Int returns a random integer in [min,max).
func Int(min, max int) int {
	return min + rand.Intn(max-min)
}

// init is required for random initialization otherwise rand will use the same seed each time.
func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
