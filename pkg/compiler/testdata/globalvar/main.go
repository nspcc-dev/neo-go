package globalvar

// Unused shouldn't produce any initialization code if it's not used anywhere.
var Unused = 3

// Default is initialized by default value.
var Default int

// A initialized by function call, thus the initialization code should always be emitted.
var A = f()

func f() int {
	return 5
}
