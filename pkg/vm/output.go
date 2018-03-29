package vm

import "encoding/json"

// StackOutput holds information about the stack, used for pretty printing
// the stack.
type stackItem struct {
	Value interface{} `json:"value"`
	Type  string      `json:"type"`
}

func buildStackOutput(vm *VM) string {
	items := make([]stackItem, vm.estack.Len())
	i := 0
	vm.estack.Iter(func(e *Element) {
		items[i] = stackItem{
			Value: e.value.Value(),
			Type:  e.value.String(),
		}
		i++
	})

	b, _ := json.MarshalIndent(items, "", "    ")
	return string(b)
}
