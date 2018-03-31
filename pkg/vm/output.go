package vm

import "encoding/json"

// StackOutput holds information about the stack, used for pretty printing
// the stack.
type stackItem struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func buildStackOutput(s *Stack) string {
	items := make([]stackItem, s.Len())
	i := 0
	s.Iter(func(e *Element) {
		items[i] = stackItem{
			Value: e.value,
			Type:  e.value.String(),
		}
		i++
	})

	b, _ := json.MarshalIndent(items, "", "    ")
	return string(b)
}
