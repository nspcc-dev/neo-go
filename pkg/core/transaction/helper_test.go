package transaction

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	// tx from C# privnet 0x25426643feed564cd3e57f346d6c68692f5622b3063da11c5572d99ee1a5b49a
	rawInvocationTX = "ANgkvBnA2KcAAAAAACCqRAAAAAAA6AMAAAHe7nnBifMAmLC6ai65CzqSWKbH/wEAXwsDAEDZ3YhNCgAMFIDOx7b1tW9QV49zfxYtOrFNRmUNDBTe7nnBifMAmLC6ai65CzqSWKbH/xTAHwwIdHJhbnNmZXIMFM924ovQBixKR47jVWEBExnzz6TSQWJ9W1I5AcYMQNafQPvPYQuqk3yCFwz8+18XCjnr8F8Rqx8e5IoQIkxjG9TjuvZm1KKGDn2UbFJnMey/FPLqezK8nbbJw2Eg10kMQKXrVyD3fs38e6Mqwsy7bAkxLsLnhvMnerbYLOqWW/DdinzT1RKAoOz5b7dAPusj5IWzQ6EifSCigJRTp//XdaMMQOv1d15PkIZM7wIvQmKDNxNy5yzQYFyoezx9Og7rM+64J9LtaHp3LFIKKNPgDhL7sFR1bd2w7vzbyR7V+Pyg3GaTEwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEF7zmyl"
)

func decodeTransaction(rawTX string, t *testing.T) *Transaction {
	b, err1 := base64.StdEncoding.DecodeString(rawTX)
	assert.Nil(t, err1)
	tx, err := NewTransactionFromBytes(b)
	assert.NoError(t, err)
	return tx
}
