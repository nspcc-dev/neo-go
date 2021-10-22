package neogointernal

// Opcode0 emits opcode without arguments.
func Opcode0(op string) interface{} {
	return nil
}

// Opcode1 emits opcode with 1 argument.
func Opcode1(op string, arg interface{}) interface{} {
	return nil
}

// Opcode2 emits opcode with 2 arguments.
func Opcode2(op string, arg1, arg2 interface{}) interface{} {
	return nil
}

// Opcode3 emits opcode with 3 arguments.
func Opcode3(op string, arg1, arg2, arg3 interface{}) interface{} {
	return nil
}
