package vm_test

import "testing"

func TestCustomType(t *testing.T) {
	src := `
		package foo

		type bar int
		type specialString string

		func Main() specialString {
			var x bar
			var str specialString
			x = 10
			str = "some short string"
			if x == 10 {
				return str
			}
			return "none"
		}
	`
	eval(t, src, []byte("some short string"))
}
