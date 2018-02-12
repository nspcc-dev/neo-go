package compiler

// import "strconv"

// // A Variable can represent any variable in the program.
// type Variable struct {
// 	// name of the variable (x, y, ..)
// 	name string
// 	// type of the variable
// 	kind VarType
// 	// actual value
// 	value interface{}
// 	// position saved in the program. This is used for storing and retrieving local
// 	// variables on the VM.
// 	pos int
// }

// // The AST package will always give us strings as the value type.
// // hence we will convert it to a VarType and assign it to the underlying interface.
// func newVariable(kind VarType, name, val string) *Variable {
// 	v := &Variable{
// 		name: name,
// 		kind: kind,
// 		pos:  -1,
// 	}

// 	if kind == STRING {
// 		v.value = val
// 	}
// 	if kind == INT {
// 		v.value, _ = strconv.Atoi(val)
// 	}
// 	if kind == BOOL {
// 		v.value, _ = strconv.ParseBool(val)
// 	}

// 	return v
// }

// func (v *Variable) add(other *Variable) {
// 	if v.kind == INT {
// 		v.value = v.value.(int) + other.value.(int)
// 	}
// }

// func (v *Variable) mul(other *Variable) {
// 	if v.kind == INT {
// 		v.value = v.value.(int) * other.value.(int)
// 	}
// }

// func (v *Variable) sub(other *Variable) {
// 	if v.kind == INT {
// 		v.value = v.value.(int) - other.value.(int)
// 	}
// }

// func (v *Variable) div(other *Variable) {
// 	if v.kind == INT {
// 		v.value = v.value.(int) / other.value.(int)
// 	}
// }

// func (v *Variable) lt(other *Variable) bool {
// 	if v.kind == INT {
// 		return v.value.(int) < other.value.(int)
// 	}
// 	return false
// }

// func (v *Variable) gt(other *Variable) bool {
// 	if v.kind == INT {
// 		return v.value.(int) > other.value.(int)
// 	}
// 	return false
// }
