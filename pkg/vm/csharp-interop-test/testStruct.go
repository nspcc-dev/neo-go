package csharpinterop

// VMUnitTest is a struct for capturing the fields in the json files
type VMUnitTest struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Tests    []struct {
		Name   string `json:"name"`
		Script string `json:"script"`
		Steps  []struct {
			Actions []string `json:"actions"`
			Result  struct {
				State           string `json:"state"`
				InvocationStack []struct {
					ScriptHash         string `json:"scriptHash"`
					InstructionPointer int    `json:"instructionPointer"`
					NextInstruction    string `json:"nextInstruction"`
					EvaluationStack    []struct {
						Type  string `json:"type"`
						Value string `json:"value"`
					} `json:"evaluationStack"`
				} `json:"invocationStack"`
			} `json:"result"`
		} `json:"steps"`
	} `json:"tests"`
}
