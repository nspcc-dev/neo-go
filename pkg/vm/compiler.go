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

// A VarType represents an arbitrary variable type.
type VarType int

// Enum with supported types.
const (
	ILLEGAL VarType = iota
	STRING
	INT
	INTARRAY
	STRINGARRAY
	STRUCT
	FUNC
)

func varTypeFromString(s string) VarType {
	switch strings.ToLower(s) {
	case "int":
		return INT
	case "string":
		return STRING
	default:
		return ILLEGAL
	}
}

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

// Register a new variable. Compiler will keep track of its position
// and type.
func (c *Compiler) registerVar(v *Variable) {
	c.vars[v.name] = v
	v.pos = c.i
	c.i++
}

// Push a variable on the stack.
func (c *Compiler) pushVar(v *Variable) {
	c.registerVar(v)

	switch v.kind {
	case INT:
		c.sb.emitPushInt(int64(v.value.(int)))
	case STRING:
		val := strings.Replace(v.value.(string), `"`, "", 2)
		c.sb.emitPushString(val)
	case INTARRAY:
	case STRINGARRAY:
	}
}

// Store as local variable
func (c *Compiler) storeLocal(v *Variable) {
	c.sb.emitPush(OpFromAltStack)
	c.sb.emitPush(OpDup)
	c.sb.emitPush(OpToAltStack)

	pos := int64(v.pos)
	if pos < 0 {
		c.reportError(fmt.Sprintf("invalid position %d to store local variable", pos))
	}

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
	if pos < 0 {
		c.reportError(fmt.Sprintf("invalid position %d to store local variable", pos))
	}

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
		kind := varTypeFromString(t.Kind.String())
		val := newVariable(kind, lhs.Name, t.Value)
		c.pushVar(val)
		c.storeLocal(val)
	// compose literals ([]int{1, 2, 3}, inline structs, ..)
	case *ast.CompositeLit:
		val := c.processComposeLit(t)
		val.name = lhs.Name
		c.pushVar(val)
	// identifiers (x, y, foo, bar)
	case *ast.Ident:
		// c.loadLocal(t.Name)
	// binary expressions (x + 2, 10 - 1)
	case *ast.BinaryExpr:
		val := c.resolveBinaryExpr(t)
		val.name = lhs.Name
		c.pushVar(val)
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
		lhsVar = &Variable{kind: val.kind, value: val.value}
	case *ast.BasicLit:
		kind := varTypeFromString(t.Kind.String())
		lhsVar = newVariable(kind, "", t.Value)
	// package AST will handle presedence for us. If the LHS is a binary expr
	// it needs to be resolved first.
	case *ast.BinaryExpr:
		lhsVar = c.resolveBinaryExpr(t)
	}

	switch t := rhs.(type) {
	case *ast.BinaryExpr:
		rhsVar = c.resolveBinaryExpr(t)
	case *ast.BasicLit:
		kind := varTypeFromString(t.Kind.String())
		rhsVar = newVariable(kind, "", t.Value)
	}

	if rhsVar.kind != lhsVar.kind {
		c.reportError(fmt.Sprintf("types mismatch %s and %s", rhsVar.kind, lhsVar.kind))
	}

	// When done resolving, process the binary operator.
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

func (c *Compiler) processComposeLit(node *ast.CompositeLit) *Variable {
	// switch t := node.Type.(type) {
	// case *ast.StructType:
	// 	fmt.Println("composing a struct inline")
	// case *ast.ArrayType:
	// 	kind := varTypeFromString(t.Elt.(*ast.Ident).Name)
	// 	return &Variable{kind: kind, value: node.Elts}
	// }
	return nil
}

func arrayTypeToken(t *ast.ArrayType) token.Token {
	kind := t.Elt.(*ast.Ident).Name
	switch kind {
	case "int":
		return token.INT
	case "string":
		return token.STRING
	}
	return token.ILLEGAL
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
	// name of the variable (x, y, ..)
	name string
	// type of the variable
	kind VarType
	// actual value
	value interface{}
	// position saved in the program. This is used for storing and retrieving local
	// variables on the VM.
	pos int
}

func newVariable(kind VarType, name, val string) *Variable {
	// The AST package will always give us strings as the value type.
	// hence we will convert it to a VarType and assign it to the underlying interface.
	v := &Variable{
		name: name,
		kind: kind,
		pos:  -1,
	}

	if kind == STRING {
		v.value = val
	}
	if kind == INT {
		v.value, _ = strconv.Atoi(val)
	}

	return v
}

func (v *Variable) add(other *Variable) {
	if v.kind == INT {
		v.value = v.value.(int) + other.value.(int)
	}
}

func (v *Variable) mul(other *Variable) {
	if v.kind == INT {
		v.value = v.value.(int) * other.value.(int)
	}
}

func (v *Variable) sub(other *Variable) {
	if v.kind == INT {
		v.value = v.value.(int) - other.value.(int)
	}
}

func (v *Variable) div(other *Variable) {
	if v.kind == INT {
		v.value = v.value.(int) / other.value.(int)
	}
}
