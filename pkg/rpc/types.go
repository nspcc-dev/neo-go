package rpc

type InvokeScriptResponse struct {
	responseHeader
	Error  *Error        `json:"error,omitempty"`
	Result *InvokeResult `json:"result,omitempty"`
}

// InvokeResult represents the outcome of a script that is
// executed by the NEO VM.
type InvokeResult struct {
	State       string `json:"state"`
	GasConsumed string `json:"gas_consumed"`
	Script      string `json:"script"`
	Stack       []*StackParam
}

// StackParam respresent a stack parameter.
type StackParam struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// AccountStateResponse holds the getaccountstate response.
type AccountStateResponse struct {
	responseHeader
	Result *Account `json:"result"`
}

// Account respresents details about a NEO account.
type Account struct {
	Version    int    `json:"version"`
	ScriptHash string `json:"script_hash"`
	Frozen     bool
	// TODO: need to check this field out.
	Votes    []interface{}
	Balances []*Balance
}

// Balance respresents details about a NEO account balance.
type Balance struct {
	Asset string `json:"asset"`
	Value string `json:"value"`
}

type params struct {
	values []interface{}
}

func newParams(vals ...interface{}) params {
	p := params{}
	p.values = make([]interface{}, len(vals))
	for i := 0; i < len(p.values); i++ {
		p.values[i] = vals[i]
	}
	return p
}

type request struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type responseHeader struct {
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
}

type response struct {
	responseHeader
	Error  *Error      `json:"error"`
	Result interface{} `json:"result"`
}
