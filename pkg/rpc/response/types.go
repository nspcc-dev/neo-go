package response

import (
	"encoding/json"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/rpc/request"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// InvokeScript stores response for the invoke script call.
type InvokeScript struct {
	HeaderAndError
	Result *InvokeResult `json:"result,omitempty"`
}

// InvokeResult represents the outcome of a script that is
// executed by the NEO VM.
type InvokeResult struct {
	State       vm.State `json:"state"`
	GasConsumed string   `json:"gas_consumed"`
	Script      string   `json:"script"`
	Stack       []request.StackParam
}

// AccountState holds the getaccountstate response.
type AccountState struct {
	Header
	Result *Account `json:"result"`
}

// Unspent represents server response to the `getunspents` command.
type Unspent struct {
	HeaderAndError
	Result *result.Unspents `json:"result,omitempty"`
}

// Account represents details about a NEO account.
type Account struct {
	Version    int    `json:"version"`
	ScriptHash string `json:"script_hash"`
	Frozen     bool
	// TODO: need to check this field out.
	Votes    []interface{}
	Balances []*Balance
}

// Balance represents details about a NEO account balance.
type Balance struct {
	Asset string `json:"asset"`
	Value string `json:"value"`
}

// Header is a generic JSON-RPC 2.0 response header (ID and JSON-RPC version).
type Header struct {
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
}

// HeaderAndError adds an Error (that can be empty) to the Header, it's used
// to construct type-specific responses.
type HeaderAndError struct {
	Header
	Error *Error `json:"error,omitempty"`
}

// Raw represents a standard raw JSON-RPC 2.0
// response: http://www.jsonrpc.org/specification#response_object.
type Raw struct {
	HeaderAndError
	Result interface{} `json:"result,omitempty"`
}

// SendToAddress stores response for the sendtoaddress call.
type SendToAddress struct {
	HeaderAndError
	Result *Tx
}

// GetTxOut represents result of `gettxout` RPC call.
type GetTxOut struct {
	HeaderAndError
	Result *result.TransactionOutput
}

// GetRawTx represents verbose output of `getrawtransaction` RPC call.
type GetRawTx struct {
	HeaderAndError
	Result *RawTx `json:"result"`
}

// RawTx stores transaction with blockchain metadata to be sent as a response.
type RawTx struct {
	Tx
	BlockHash     string `json:"blockhash"`
	Confirmations uint   `json:"confirmations"`
	BlockTime     uint   `json:"blocktime"`
}

// Tx stores transaction to be sent as a response.
type Tx struct {
	TxID       string                  `json:"txid"`
	Size       int                     `json:"size"`
	Type       string                  `json:"type"` // todo: convert to TransactionType
	Version    int                     `json:"version"`
	Attributes []transaction.Attribute `json:"attributes"`
	Vins       []Vin                   `json:"vin"`
	Vouts      []Vout                  `json:"vout"`
	SysFee     int                     `json:"sys_fee"`
	NetFee     int                     `json:"net_fee"`
	Scripts    []transaction.Witness   `json:"scripts"`
}

// Vin represents JSON-serializable tx input.
type Vin struct {
	TxID string `json:"txid"`
	Vout int    `json:"vout"`
}

// Vout represents JSON-serializable tx output.
type Vout struct {
	N       int    `json:"n"`
	Asset   string `json:"asset"`
	Value   int    `json:"value"`
	Address string `json:"address"`
}
