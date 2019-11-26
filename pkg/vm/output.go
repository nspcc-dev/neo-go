package vm

import "encoding/json"

// StackOutput holds information about the stack, used for pretty printing
// the stack.
type stackItem struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func appendToItems(items *[]stackItem, val StackItem, seen map[StackItem]bool) {
	if arr, ok := val.Value().([]StackItem); ok {
		if seen[val] {
			return
		}
		seen[val] = true
		intItems := make([]stackItem, 0, len(arr))
		for _, v := range arr {
			appendToItems(&intItems, v, seen)
		}
		*items = append(*items, stackItem{
			Value: intItems,
			Type:  val.String(),
		})

	} else {
		*items = append(*items, stackItem{
			Value: val,
			Type:  val.String(),
		})
	}
}

func stackToArray(s *Stack) []stackItem {
	items := make([]stackItem, 0, s.Len())
	seen := make(map[StackItem]bool)
	s.IterBack(func(e *Element) {
		appendToItems(&items, e.value, seen)
	})
	return items
}

func buildStackOutput(s *Stack) string {
	b, _ := json.MarshalIndent(stackToArray(s), "", "    ")
	return string(b)
}
