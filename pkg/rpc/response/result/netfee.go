package result

// NetworkFee represents a result of calculatenetworkfee RPC call.
type NetworkFee struct {
	Value int64 `json:"networkfee,string"`
}
