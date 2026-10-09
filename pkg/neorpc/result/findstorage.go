package result

// FindStorage represents the result of `findstorage` RPC handler.
type FindStorage struct {
	Results []KeyValue `json:"results"`
	// Next is an exclusive cursor for the next page. It holds the last returned
	// key (or empty if there are no results). Pass it as `start` to the next
	// call to `findstorage*` if Truncated is set.
	Next      []byte `json:"next"`
	Truncated bool   `json:"truncated"`
}
