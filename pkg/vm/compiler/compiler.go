package compiler

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

const (
	outputExt = ".avm"
)

// A VarContext holds the info about the context of a variable in the program.
type VarContext struct {
	name  string
	tinfo types.TypeAndValue
	pos   int
}

func newVarContext(name string, tinfo types.TypeAndValue) *VarContext {
	return &VarContext{
		name:  name,
		pos:   -1,
		tinfo: tinfo,
	}
}

// Compiler holds the output buffer of the compiled source.
type Compiler struct {
	// Output extension of the file. Default .avm.
	OutputExt  string
	sb         *ScriptBuilder
	curLineNum int

	funcs    map[string]*FuncContext
	typeInfo *types.Info

	i int
}

// New returns a new compiler ready to compile smartcontracts.
func New() *Compiler {
	return &Compiler{
		OutputExt: outputExt,
		sb:        &ScriptBuilder{buf: new(bytes.Buffer)},
		funcs:     map[string]*FuncContext{},
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

// LoadConst load a constant, if storeLocal is true it will store it on the position
// of the VarContext.
func (c *Compiler) loadConst(ctx *VarContext, storeLocal bool) {
	switch ctx.tinfo.Type.(*types.Basic).Kind() {
	case types.Int:
		val, _ := constant.Int64Val(ctx.tinfo.Value)
		c.sb.emitPushInt(val)
	case types.String:
		val := constant.StringVal(ctx.tinfo.Value)
		c.sb.emitPushString(val)
	}

	if storeLocal {
		c.storeLocal(ctx)
	}
}

// Load a local variable. The position of the VarContext is used to retrieve from
// that position.
func (c *Compiler) loadLocal(ctx *VarContext) {
	pos := int64(ctx.pos)
	if pos < 0 {
		log.Fatalf("invalid position to load local %d", pos)
	}

	c.sb.emitPush(vm.OpFromAltStack)
	c.sb.emitPush(vm.OpDup)
	c.sb.emitPush(vm.OpToAltStack)

	// push it's index on the stack
	c.sb.emitPushInt(pos)
	c.sb.emitPush(vm.OpPickItem)
}

// Store a local variable on the stack. The position of the VarContext is used
// to store at that position.
func (c *Compiler) storeLocal(vctx *VarContext) {
	c.sb.emitPush(vm.OpFromAltStack)
	c.sb.emitPush(vm.OpDup)
	c.sb.emitPush(vm.OpToAltStack)

	pos := int64(vctx.pos)
	if pos < 0 {
		log.Fatalf("invalid position to store local: %d", pos)
	}

	c.sb.emitPushInt(pos)
	c.sb.emitPushInt(2)
	c.sb.emitPush(vm.OpRoll)
	c.sb.emitPush(vm.OpSetItem)
}

// Compile will compile from r into an avm format.
func (c *Compiler) Compile(r io.Reader) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", r, 0)
	if err != nil {
		return err
	}

	conf := types.Config{Importer: importer.Default()}
	typeInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	c.typeInfo = typeInfo

	_, err = conf.Check("", fset, []*ast.File{f}, typeInfo)
	if err != nil {
		log.Fatal(err)
	}

	for _, decl := range f.Decls {
		switch t := decl.(type) {
		case *ast.GenDecl:
		case *ast.FuncDecl:
			c.convertFuncDecl(t)
		}
	}

	return nil
}

func (c *Compiler) convertFuncDecl(decl *ast.FuncDecl) {
	ctx := newFuncContext(decl.Name.Name, c.i)
	c.funcs[ctx.name] = ctx

	c.sb.emitPush(vm.OpPush2)
	c.sb.emitPush(vm.OpNewArray)
	c.sb.emitPush(vm.OpToAltStack)

	for _, stmt := range decl.Body.List {
		c.convertStmt(ctx, stmt)
	}

	c.i++
}

func (c *Compiler) convertStmt(fctx *FuncContext, stmt ast.Stmt) {
	switch t := stmt.(type) {
	case *ast.AssignStmt:
		for i := 0; i < len(t.Lhs); i++ {
			lhs := t.Lhs[i].(*ast.Ident)

			switch rhs := t.Rhs[i].(type) {
			case *ast.BasicLit:
				vctx := newVarContext(lhs.Name, c.getTypeInfo(t.Rhs[i]))
				fctx.registerContext(vctx, true)
				c.loadConst(vctx, true)
				continue

			case *ast.Ident:
				knownCtx := fctx.getContext(rhs.Name)
				c.loadLocal(knownCtx)
				newCtx := newVarContext(lhs.Name, c.getTypeInfo(rhs))
				fctx.registerContext(newCtx, true)
				c.storeLocal(newCtx)
				continue
			}

			vctx := newVarContext(lhs.Name, c.getTypeInfo(t.Rhs[i]))
			fctx.registerContext(vctx, true)
			c.convertExpr(fctx, t.Rhs[i])
			c.storeLocal(vctx)
		}

	//Due to the design of the orginal VM, multiple return are not supported.
	case *ast.ReturnStmt:
		if len(t.Results) > 1 {
			log.Fatal("multiple returns not supported.")
		}

		c.sb.emitPush(vm.OpJMP)
		c.sb.emitPush(vm.OpCode(0x03))
		c.sb.emitPush(vm.OpPush0)

		c.convertExpr(fctx, t.Results[0])

		c.sb.emitPush(vm.OpNOP)
		c.sb.emitPush(vm.OpFromAltStack)
		c.sb.emitPush(vm.OpDrop)
		c.sb.emitPush(vm.OpRET)

	case *ast.IfStmt:
		c.convertExpr(fctx, t.Cond)

		// use a placeholder for the label.
		c.sb.emitJump(vm.OpJMPIFNOT, int16(0))
		// track our offset to update later subtract sizeOf int16.
		jumpOffset := int(c.currentPos()) - 2

		// Process the block.
		for _, stmt := range t.Body.List {
			c.convertStmt(fctx, stmt)
		}

		jumpTo := c.currentPos() + 1 - int16(jumpOffset)
		c.sb.updateJmpLabel(jumpTo, jumpOffset)
	}
}

func (c *Compiler) convertExpr(fctx *FuncContext, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.BasicLit:
		vctx := newVarContext("", c.getTypeInfo(t))
		c.loadConst(vctx, false)

	case *ast.Ident:
		vctx := fctx.getContext(t.Name)
		c.loadLocal(vctx)

	case *ast.BinaryExpr:
		if tinfo := c.getTypeInfo(t); tinfo.Value != nil {
			vctx := newVarContext("", tinfo)
			c.loadConst(vctx, false)
			return
		}

		c.convertExpr(fctx, t.X)
		c.convertExpr(fctx, t.Y)
		c.convertToken(t.Op)
	}
}

func (c *Compiler) convertToken(tok token.Token) {
	switch tok {
	case token.ADD:
		c.sb.emitPush(vm.OpAdd)
	case token.SUB:
		c.sb.emitPush(vm.OpSub)
	case token.MUL:
		c.sb.emitPush(vm.OpMul)
	case token.QUO:
		c.sb.emitPush(vm.OpDiv)
	case token.LSS:
		c.sb.emitPush(vm.OpLT)
	case token.LEQ:
		c.sb.emitPush(vm.OpLTE)
	case token.GTR:
		c.sb.emitPush(vm.OpGT)
	case token.GEQ:
		c.sb.emitPush(vm.OpGTE)
	}
}

// getTypeInfo return TypeAndValue for the given expression. If it could not resolve
// the type value and type will be NIL.
func (c *Compiler) getTypeInfo(expr ast.Expr) types.TypeAndValue {
	return c.typeInfo.Types[expr]
}

// currentPos return the current position (address) of the latest opcode.
func (c *Compiler) currentPos() int16 {
	return int16(c.sb.buf.Len())
}

// DumpOpcode dumps the current buffer, formatted with index, hex and opcode.
func (c *Compiler) DumpOpcode() {
	c.sb.dumpOpcode()
}
