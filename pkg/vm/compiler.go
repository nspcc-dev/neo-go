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
	"strings"
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

	i       int
	varList []Variable
	vars    map[string]Variable
}

// NewCompiler returns a new compiler ready to compile smartcontracts.
func NewCompiler() *Compiler {
	return &Compiler{
		OutputExt: outputExt,
		sb:        &ScriptBuilder{new(bytes.Buffer)},
		vars:      map[string]Variable{},
		varList:   []Variable{},
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

func (c *Compiler) newVariable(k token.Token, i, val string) Variable {
	v := Variable{
		kind:  k,
		ident: i,
		value: val,
		pos:   c.i,
	}
	c.vars[v.ident] = v
	c.i++
	return v
}

func (c *Compiler) initialize(n OpCode) {
	// Get the (n) localVars which is basicly the number of args passed in Main
	// and the number of local Vars in the function body.
	c.sb.emitPush(n)
	c.sb.emitPush(OpNewArray)
	c.sb.emitPush(OpToAltStack)
}

func (c *Compiler) teardown() {
	c.sb.emitPush(OpNOP)
	c.sb.emitPush(OpFromAltStack)
	c.sb.emitPush(OpDrop)
	c.sb.emitPush(OpRET)
}

// Push a variable on to the stack.
func (c *Compiler) storeLocal(v Variable) {
	if v.kind == token.INT {
		val, _ := strconv.Atoi(v.value)
		c.sb.emitPushInt(int64(val))
	}
	if v.kind == token.STRING {
		val := strings.Replace(v.value, `"`, "", 2)
		c.sb.emitPushString(val)
	}

	c.sb.emitPush(OpFromAltStack)
	c.sb.emitPush(OpDup)
	c.sb.emitPush(OpToAltStack)

	pos := int64(v.pos)

	c.sb.emitPushInt(pos)
	c.sb.emitPushInt(2)
	c.sb.emitPush(OpRoll)
	c.sb.emitPush(OpSetItem)
}

func (c *Compiler) loadLocal(ident string) {
	val, ok := c.vars[ident]
	if !ok {
		c.reportError(fmt.Sprintf("local variable %s not found", ident))
	}

	pos := int64(val.pos)

	c.sb.emitPush(OpFromAltStack)
	c.sb.emitPush(OpDup)
	c.sb.emitPush(OpToAltStack)

	// push it's index on the stack
	c.sb.emitPushInt(pos)
	c.sb.emitPush(OpPickItem)
}

// TODO: instead of passing the stmt in to this, put the lhs and rhs in.
// so we can reuse this.
func (c *Compiler) processAssignStmt(stmt *ast.AssignStmt) {
	lhs := stmt.Lhs[0].(*ast.Ident)
	switch t := stmt.Rhs[0].(type) {
	case *ast.BasicLit:
		c.storeLocal(c.newVariable(t.Kind, lhs.Name, t.Value))
	case *ast.CompositeLit:
		switch t.Type.(type) {
		case *ast.StructType:
			c.reportError("assigning struct literals not yet implemented")
		case *ast.ArrayType:
			// for _, expr := range t.Elts {
			// 	v := expr.(*ast.BasicLit)
			// 	c.storeLocal(c.newVariable(v.Kind, lhs.Name, v.Value))
			// }
		}
	case *ast.Ident:
		c.loadLocal(t.Name)
	case *ast.FuncLit:
		c.reportError("assigning function literals not yet implemented")
	default:
		fmt.Println(reflect.TypeOf(t))
	}
}

// Compile will compile from r into an avm format.
func (c *Compiler) Compile(r io.Reader) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", r, 0)
	if err != nil {
		return err
	}

	c.initialize(OpPush2) // initialize the compiler with n local stack vars.
	ast.Walk(c, f)        // walk through and process the AST
	c.teardown()          // done compiling

	fmt.Println(c.vars)

	return nil
}

// TODO: More detailed report (lineno, ...)
func (c *Compiler) reportError(msg string) {
	fmt.Printf("COMPILER ERROR :: %s\n", msg)
	os.Exit(1)
}

// A Variable can represent any variable in the program.
type Variable struct {
	ident string
	kind  token.Token
	value string
	pos   int
}
