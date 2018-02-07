package vm

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
)

var raw = `
package NEP5 

func Main() {
	tokenName := "some token name"
	arr := []int{1, 2}
	x := 1
}
`

const (
	maxStackBytes = 75
	outputExt     = ".avm"
)

// Compiler holds the output buffer of the compiled source.
type Compiler struct {
	buf *bytes.Buffer
}

// writes an opcode into the buffer.
func (c *Compiler) writeOpCode(op OpCode) error {
	return nil
}

func (c *Compiler) processString(str string) error {
	return nil
}

// Visit implements the ? interface.
func (c *Compiler) Visit(node ast.Node) ast.Visitor {
	switch t := node.(type) {
	case *ast.AssignStmt:
		for _, expr := range t.Rhs {
			switch tt := expr.(type) {
			case *ast.BasicLit:
				if tt.Kind == token.STRING {
				}
			case *ast.ArrayType:
				//fmt.Println(tt)
			case *ast.SliceExpr:
				//fmt.Println(tt)
			case *ast.CompositeLit:
				// fmt.Println(tt)
			default:
				fmt.Println(reflect.TypeOf(tt))
			}
		}
	case *ast.FuncDecl:
		//fmt.Println(t)
	}

	return c
}

// CompileSource will compile the given source file into an avm format. Ready
// for deploying on the NEO blockchain.
func CompileSource(src string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", raw, parser.DeclarationErrors)
	if err != nil {
		return err
	}

	c := &Compiler{new(bytes.Buffer)}

	ast.Walk(c, f)

	fmt.Println(c.buf.Bytes())

	return nil
}
