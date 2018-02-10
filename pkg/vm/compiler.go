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

	i    int
	vars map[string]*Variable
}

// NewCompiler returns a new compiler ready to compile smartcontracts.
func NewCompiler() *Compiler {
	return &Compiler{
		OutputExt: outputExt,
		sb:        &ScriptBuilder{new(bytes.Buffer)},
		vars:      map[string]*Variable{},
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

func (c *Compiler) registerVar(v *Variable) {
	// if oldVar, ok := c.vars[v.ident]; ok {
	// 	if oldVar.kind != v.kind {
	// 		c.reportError(fmt.Sprintf("types mismatch %s and %s", oldVar.kind, v.kind))
	// 	}
	// 	oldVar.value = v.value
	// 	return
	// }
	c.vars[v.ident] = v
	v.pos = c.i
	c.i++
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

// Push a variable on the stack.
func (c *Compiler) storeLocal(v *Variable) {
	c.registerVar(v)

	if v.kind == token.INT {
		c.sb.emitPushInt(int64(v.value.(int)))
	}
	if v.kind == token.STRING {
		val := strings.Replace(v.value.(string), `"`, "", 2)
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

func (c *Compiler) processAssignStmt(stmt *ast.AssignStmt) {
	lhs := stmt.Lhs[0].(*ast.Ident)

	switch t := stmt.Rhs[0].(type) {
	// basic literals (1, "some string")
	case *ast.BasicLit:
		c.storeLocal(newVariable(lhs.Name, t.Kind, t.Value))
	// compose literals ([]int{1, 2, 3}, inline structs, ..)
	case *ast.CompositeLit:
		c.processComposeLit(t)
	// identifiers (x, y, foo, bar)
	case *ast.Ident:
		c.loadLocal(t.Name)
	// binary expressions (x + 2, 10 - 1)
	case *ast.BinaryExpr:
		// first resolve the bin expr.
		val := c.resolveBinaryExpr(t)
		// fmt.Println(lhs.Name)
		// fmt.Println(val.ident)
		// val.ident = lhs.Name
		// store the resuls as a local var.
		c.storeLocal(val)
	// inline function literals (x := func() {})
	case *ast.FuncLit:
		c.reportError("assigning function literals not yet implemented")
	default:
		fmt.Println(reflect.TypeOf(t))
	}
}

func (c *Compiler) resolveBinaryExpr(expr *ast.BinaryExpr) *Variable {
	var (
		lhs    = expr.X
		rhs    = expr.Y
		lhsVar *Variable
		rhsVar *Variable
	)

	switch t := lhs.(type) {
	case *ast.Ident:
		val, ok := c.vars[t.Name]
		if !ok {
			c.reportError(fmt.Sprintf("could not resolve %s", t.Name))
		}
		lhsVar = newVariable("", val.kind, val.value)
	case *ast.BasicLit:
		lhsVar = newVariable("", t.Kind, t.Value)
	// package ast will handle presedence for us :)
	// if the lhs is binary expr it needs to be resolved first.
	case *ast.BinaryExpr:
		lhsVar = c.resolveBinaryExpr(t)
	}

	switch t := rhs.(type) {
	case *ast.BinaryExpr:
		rhsVar = c.resolveBinaryExpr(t)
	case *ast.BasicLit:
		rhsVar = newVariable("", t.Kind, t.Value)
	}

	if rhsVar.kind != lhsVar.kind {
		c.reportError(fmt.Sprintf("types mismatch %s and %s", rhsVar.kind, lhsVar.kind))
	}

	// When we resolved just handle the operator.
	if expr.Op == token.ADD {
		lhsVar.add(rhsVar)
	}
	if expr.Op == token.MUL {
		lhsVar.mul(rhsVar)
	}
	if expr.Op == token.SUB {
		lhsVar.sub(rhsVar)
	}
	if expr.Op == token.QUO {
		lhsVar.div(rhsVar)
	}

	return lhsVar
}

func (c *Compiler) processRHS(node ast.Node) {
	switch t := node.(type) {
	case *ast.CompositeLit:
		fmt.Println("compose lit")
	case *ast.BasicLit:
		fmt.Println("just assign this")
	case *ast.Ident:
		fmt.Println("an identifier maybe not known")
	case *ast.BinaryExpr:
		fmt.Println("binary expression")
	case *ast.FuncLit:
	default:
		fmt.Println(reflect.TypeOf(t))
	}
}

func (c *Compiler) processComposeLit(node *ast.CompositeLit) {
	switch t := node.Type.(type) {
	case *ast.StructType:
		fmt.Println("composing a struct inline")
	case *ast.ArrayType:
		fmt.Println("composing an array")
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

	return nil
}

// TODO: More detailed report (lineno, ...)
func (c *Compiler) reportError(msg string) {
	fmt.Printf("COMPILER ERROR :: %s\n", msg)
	os.Exit(1)
}

// DumpOpcode dumps the current buffer, formatted with index, hex and opcode.
// Usefull for debugging smartcontracts.
func (c *Compiler) DumpOpcode() {
	c.sb.dumpOpcode()
}

// A Variable can represent any variable in the program.
type Variable struct {
	ident string
	kind  token.Token
	value interface{}
	pos   int
}

func newVariable(ident string, kind token.Token, val string) *Variable {
	// The AST will always give us strings as the value type. Therefor we will convert
	// it here assign it to the underlying interface.
	v := &Variable{
		ident: ident,
		kind:  kind,
	}

	if kind == token.STRING {
		v.value = val
	}
	if kind == token.INT {
		v.value, _ = strconv.Atoi(val)
	}

	return v
}

func (v *Variable) add(other *Variable) {
	if v.kind == token.INT {
		v.value = v.value.(int) + other.value.(int)
	}
}

func (v *Variable) mul(other *Variable) {
	if v.kind == token.INT {
		v.value = v.value.(int) * other.value.(int)
	}
}

func (v *Variable) sub(other *Variable) {
	if v.kind == token.INT {
		v.value = v.value.(int) - other.value.(int)
	}
}

func (v *Variable) div(other *Variable) {
	if v.kind == token.INT {
		v.value = v.value.(int) / other.value.(int)
	}
}
