package compiler_test

import (
	"math/big"
	"testing"
)

func TestAddressOfLiteral(t *testing.T) {
	src := `package foo
	type Foo struct { A int	}
	func Main() int {
		f := &Foo{}
		setA(f, 3)
		return f.A
	}
	func setA(s *Foo, a int) { s.A = a }`
	eval(t, src, big.NewInt(3))
}

func TestPointerDereference(t *testing.T) {
	src := `package foo
	type Foo struct { A int	}
	func Main() int {
		f := &Foo{A: 4}
		setA(*f, 3)
		return f.A
	}
	func setA(s Foo, a int) { s.A = a }`
	eval(t, src, big.NewInt(4))
}

func TestStructArgCopy(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo                                                                                                                                                                                                                                                                                                                                                                  
		type Foo struct { A int }                                                                                                                                                                                                                                                                                                                                                            
		func Main() int {                                                                                                                                                                                                                                                                                                                                                                    
			f := Foo{A: 4}                                                                                                                                                                                                                                                                                                                                                               
			setA(f, 3)                                                                                                                                                                                                                                                                                                                                                                   
			return f.A                                                                                                                                                                                                                                                                                                                                                                   
		}                                                                                                                                                                                                                                                                                                                                                                                    
		func setA(s Foo, a int) { s.A = a }`
		eval(t, src, big.NewInt(4))
	})
	t.Run("StructField", func(t *testing.T) {
		src := `package foo
		type Bar struct { A int }
		type Foo struct { B Bar }                                                                                                                                                                                                                                                                                                                                                            
		func Main() int {                                                                                                                                                                                                                                                                                                                                                                    
			f := Foo{B: Bar{A: 4}}                                                                                                                                                                                                                                                                                                                                                               
			setA(f, 3)                                                                                                                                                                                                                                                                                                                                                                   
			return f.B.A                                                                                                                                                                                                                                                                                                                                                                   
		}                                                                                                                                                                                                                                                                                                                                                                                    
		func setA(s Foo, a int) { s.B.A = a }`
		eval(t, src, big.NewInt(4))
	})
	t.Run("StructPointerField", func(t *testing.T) {
		src := `package foo
		type Bar struct { A int }
		type Foo struct { B *Bar }                                                                                                                                                                                                                                                                                                                                                            
		func Main() int {                                                                                                                                                                                                                                                                                                                                                                    
			f := Foo{B: &Bar{A: 4}}                                                                                                                                                                                                                                                                                                                                                               
			setA(f, 3)                                                                                                                                                                                                                                                                                                                                                                   
			return f.B.A                                                                                                                                                                                                                                                                                                                                                                   
		}                                                                                                                                                                                                                                                                                                                                                                                    
		func setA(s Foo, a int) { s.B.A = a }`
		eval(t, src, big.NewInt(3))
	})
}
