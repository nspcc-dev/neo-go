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

type VarContext struct {
	name  string
	kind  types.Type
	value interface{}
	pos   int
}

func newVarContext(tinfo types.TypeAndValue) *VarContext {
	ctx := &VarContext{
		kind: tinfo.Type,
		pos:  -1,
	}

	t := tinfo.Type.(*types.Basic)
	switch t.Kind() {
	case types.Int64, types.Int, types.UntypedInt:
		ctx.value, _ = constant.Int64Val(tinfo.Value)
	case types.String:
		ctx.value = constant.StringVal(tinfo.Value)
	default:
		log.Fatal(t)
	}
	return ctx
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
		sb:        &ScriptBuilder{new(bytes.Buffer)},
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

func (c *Compiler) convertVar(ctx *VarContext, storeLocal bool) {
	switch ctx.kind.(*types.Basic).Kind() {
	case types.Int:
		c.sb.emitPushInt(ctx.value.(int64))
	case types.String:
		c.sb.emitPushString(ctx.value.(string))
	}
	if storeLocal {
		c.storeLocal(ctx)
	}
}

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

func (c *Compiler) storeLocal(ctx *VarContext) {
	c.sb.emitPush(vm.OpFromAltStack)
	c.sb.emitPush(vm.OpDup)
	c.sb.emitPush(vm.OpToAltStack)

	pos := int64(ctx.pos)

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
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
	}
	c.typeInfo = info

	_, err = conf.Check("", fset, []*ast.File{f}, info)
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
	ctx := newFuncContext(decl.Name.Name)
	c.funcs[ctx.name] = ctx

	c.sb.emitPush(vm.OpPush2)
	c.sb.emitPush(vm.OpNewArray)
	c.sb.emitPush(vm.OpToAltStack)

	for _, stmt := range decl.Body.List {
		c.convertStmt(ctx, stmt)
	}
}

func (c *Compiler) convertStmt(funcCtx *FuncContext, stmt ast.Stmt) {
	switch t := stmt.(type) {
	case *ast.AssignStmt:
		if t.Tok == token.DEFINE {
			for i := 0; i < len(t.Lhs); i++ {
				lhs := t.Lhs[i].(*ast.Ident)
				ctx := funcCtx.varContextFromExpr(t.Rhs[i], c.typeInfo)
				ctx.name = lhs.Name
				funcCtx.putContext(ctx, true)
				c.convertVar(ctx, true)
			}
		}
		// TODO: handle assigns
	// The Opcode needs to be compatible with other platforms. Therefore multiple returns will not be
	// supported.
	case *ast.IfStmt:
		switch t := t.Cond.(type) {
		case *ast.BinaryExpr:
			lhs := funcCtx.varContextFromExpr(t.X, c.typeInfo)
			rhs := funcCtx.varContextFromExpr(t.Y, c.typeInfo)

			// if the LHS is registered load local
			if funcCtx.isRegistered(lhs) {
				c.loadLocal(lhs)
			}
			// write the RHS
			if funcCtx.isRegistered(rhs) {
				c.loadLocal(rhs)
			} else {
				c.sb.emitPushInt(rhs.value.(int64))
			}

			switch t.Op {
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
	case *ast.ReturnStmt:
		if len(t.Results) > 1 {
			log.Fatal("multiple returns not supported.")
		}

		c.sb.emitPush(vm.OpJMP)
		c.sb.emitPush(vm.OpCode(0x03))
		c.sb.emitPush(vm.OpPush0)

		result := t.Results[0]
		switch t := result.(type) {
		case *ast.Ident:
			varCtx := funcCtx.getContext(t.Name)
			c.loadLocal(varCtx)
		case *ast.BasicLit:
			tinfo := c.typeInfo.Types[t]
			ctx := newVarContext(tinfo)
			c.convertVar(ctx, false)
		case *ast.BinaryExpr:
		}

		// Write the actual return.
		c.sb.emitPush(vm.OpNOP)
		c.sb.emitPush(vm.OpFromAltStack)
		c.sb.emitPush(vm.OpDrop)
		c.sb.emitPush(vm.OpRET)
	}
}

func resolveBinaryExpr(ctx *FuncContext, expr *ast.BinaryExpr, tinfo *types.Info) *VarContext {
	lhs := ctx.varContextFromExpr(expr.X, tinfo)
	rhs := ctx.varContextFromExpr(expr.Y, tinfo)

	// Handle the binary operator assuming that the type is int64.
	switch expr.Op {
	case token.ADD:
		lhs.value = lhs.value.(int64) + rhs.value.(int64)
	case token.SUB:
		lhs.value = lhs.value.(int64) - rhs.value.(int64)
	case token.QUO:
		lhs.value = lhs.value.(int64) / rhs.value.(int64)
	case token.MUL:
		lhs.value = lhs.value.(int64) * rhs.value.(int64)
	}

	return lhs
}

// DumpOpcode dumps the current buffer, formatted with index, hex and opcode.
func (c *Compiler) DumpOpcode() {
	c.sb.dumpOpcode()
}
