package nested2

// Unused shouldn't produce any code if unused.
var Unused = 21

// Argument is an argument used from external package to call nested1.f.
var Argument = 22

// A has the same name as nested1.A.
var A = 23

// B should produce call to f and be DROPped if unused.
var B = f()

// Unique has unique name.
var Unique = 24

func f() int {
	return 25
}
