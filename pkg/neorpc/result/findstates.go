package result

type FindStates struct {
	Results    []KeyValue    `json:"results"`
	FirstProof *ProofWithKey `json:"firstProof,omitzero"`
	LastProof  *ProofWithKey `json:"lastProof,omitzero"`
	Truncated  bool          `json:"truncated"`
}

type KeyValue struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}
