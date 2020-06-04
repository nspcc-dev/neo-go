package result

// StateHeight is a result of getstateheight RPC.
type StateHeight struct {
	BlockHeight uint32 `json:"blockHeight"`
	StateHeight uint32 `json:"stateHeight"`
}
