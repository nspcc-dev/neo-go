package vm

import "encoding/json"

// StackOutput holds information about the stack, used for pretty printing
// the stack.
type stackItem struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func stackToArray(s *Stack) []stackItem {
	items := make([]stackItem, 0, s.Len())
	s.Iter(func(e *Element) {
		items = append(items, stackItem{
			Value: e.value,
			Type:  e.value.String(),
		})
	})
	return items
}

func buildStackOutput(s *Stack) string {
	b, _ := json.MarshalIndent(stackToArray(s), "", "    ")
	return string(b)
}
