package vm

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"reflect"
	"strconv"
)

const (
	outputExt = ".avm"
)

// Compiler holds the output buffer of the compiled source.
type Compiler struct {
	// Output extension of the file. Default .avm.
	OutputExt  string
	sb         *ScriptBuilder
	curLineNum int
	si         int
	vars       map[string]Variable
}

// NewCompiler returns a new compiler ready to compile smartcontracts.
func NewCompiler() *Compiler {
	return &Compiler{
		OutputExt: outputExt,
		sb:        &ScriptBuilder{new(bytes.Buffer)},
		vars:      map[string]Variable{},
	}
}

// CompileSource will compile the source file into an avm format.
func (c *Compiler) CompileSource(src string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	return c.Compile(file)
}

// Visit implements the ast.Visitor interface.
func (c *Compiler) Visit(node ast.Node) ast.Visitor {
	switch t := node.(type) {
	case *ast.AssignStmt:
		c.processAssignStmt(t)
	case *ast.FuncDecl:
	case *ast.ReturnStmt:
	}
	return c
}

func typeof(v interface{}) {
	fmt.Println(reflect.TypeOf(v))
}

type Variable struct {
	kind  token.Token
	value string
}

func (c *Compiler) pushVar(kind token.Token, name, value string) {
	c.vars[name] = Variable{kind, value}
	if kind == token.INT {
		val, _ := strconv.Atoi(value)
		c.sb.emitPushInt(int64(val))
	}
}

func (c *Compiler) processAssignStmt(stmt *ast.AssignStmt) {
	lhs := stmt.Lhs[0].(*ast.Ident)
	switch t := stmt.Rhs[0].(type) {
	case *ast.BasicLit:
		c.pushVar(t.Kind, lhs.Name, t.Value)
	case *ast.CompositeLit:
		switch t.Type.(type) {
		case *ast.ArrayType:
			for _, expr := range t.Elts {
				v := expr.(*ast.BasicLit)
				c.pushVar(v.Kind, lhs.Name, v.Value)
			}
		}

	case *ast.Ident:
		switch tt := t.Obj.Decl.(type) {
		case *ast.BasicLit:
			fmt.Println("its a basit literal")
		default:
			typeof(tt)
		}
	default:
		typeof(t)

		fmt.Println("push to stack")
	}
}

// Compile will compile from r into an avm format.
func (c *Compiler) Compile(r io.Reader) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", r, 0)
	if err != nil {
		return err
	}

	ast.Walk(c, file)
	fmt.Println(c.vars)

	return nil
}
