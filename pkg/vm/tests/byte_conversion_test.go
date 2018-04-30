package vm_test

import "testing"

func TestStringToByteConversion(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		b := []byte("foo")
		return b
	}
	`
	eval(t, src, []byte("foo"))
}

func TestStringToByteAppend(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		b := []byte("foo")
		c := []byte("bar")
		e := append(b, c...)
		return e
	}
	`
	eval(t, src, []byte("foobar"))
}

func TestByteConversionInFunctionCall(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		b := []byte("foo")
		return handle(b)
	}

	func handle(b []byte) []byte {
		return b
	}
	`
	eval(t, src, []byte("foo"))
}

func TestByteConversionDirectlyInFunctionCall(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		return handle([]byte("foo"))
	}

	func handle(b []byte) []byte {
		return b
	}
	`
	eval(t, src, []byte("foo"))
}
