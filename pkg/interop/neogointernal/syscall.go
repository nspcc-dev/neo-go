package neogointernal

// Syscall0 performs syscall with 0 arguments.
func Syscall0(name string) interface{} {
	return nil
}

// Syscall0NoReturn performs syscall with 0 arguments.
func Syscall0NoReturn(name string) {
}

// Syscall1 performs syscall with 1 arguments.
func Syscall1(name string, arg interface{}) interface{} {
	return nil
}

// Syscall1NoReturn performs syscall with 1 arguments.
func Syscall1NoReturn(name string, arg interface{}) {
}

// Syscall2 performs syscall with 2 arguments.
func Syscall2(name string, arg1, arg2 interface{}) interface{} {
	return nil
}

// Syscall2NoReturn performs syscall with 2 arguments.
func Syscall2NoReturn(name string, arg1, arg2 interface{}) {
}

// Syscall3 performs syscall with 3 arguments.
func Syscall3(name string, arg1, arg2, arg3 interface{}) interface{} {
	return nil
}

// Syscall3NoReturn performs syscall with 3 arguments.
func Syscall3NoReturn(name string, arg1, arg2, arg3 interface{}) {
}

// Syscall4 performs syscall with 4 arguments.
func Syscall4(name string, arg1, arg2, arg3, arg4 interface{}) interface{} {
	return nil
}

// Syscall4NoReturn performs syscall with 4 arguments.
func Syscall4NoReturn(name string, arg1, arg2, arg3, arg4 interface{}) {
}
