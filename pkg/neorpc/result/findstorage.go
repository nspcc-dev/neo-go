package result

// FindStorage represents the result of `findstorage` RPC handler.
type FindStorage struct {
	Results []KeyValue `json:"results"`
	// Next contains the index of the next subsequent element of the contract storage
	// that can be retrieved during the next iteration.
	Next      int  `json:"next"`
	Truncated bool `json:"truncated"`
}
