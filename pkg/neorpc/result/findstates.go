package result

type FindStates struct {
	Results    []KeyValue    `json:"results"`
	FirstProof *ProofWithKey `json:"firstProof,omitempty"`
	LastProof  *ProofWithKey `json:"lastProof,omitempty"`
	Truncated  bool          `json:"truncated"`
}

type KeyValue struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}
