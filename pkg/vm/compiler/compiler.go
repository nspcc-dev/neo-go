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
	"reflect"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

const (
	outputExt = ".avm"
	// Identifier off the entry point function.
	mainIdent = "Main"
)

// CallContext represents more details on function calls in the program.
// It stores the position off where the call happend along with the
// function it called. The compiler will store all function calls, so
// it can update them later.
type CallContext struct {
	pos      int
	funcName string
}

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
	OutputExt string

	// scriptBuilder is responsible for all opcode writes.
	sb *ScriptBuilder

	// map with all function contexts across the program.
	funcs map[string]*FuncContext

	// list of function calls
	funcCalls []CallContext

	// struct with info about decls, types, uses, ..
	typeInfo *types.Info
}

// New returns a new compiler ready to compile smartcontracts.
func New() *Compiler {
	return &Compiler{
		OutputExt: outputExt,
		sb:        &ScriptBuilder{buf: new(bytes.Buffer)},
		funcs:     map[string]*FuncContext{},
		funcCalls: []CallContext{},
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
	switch t := ctx.tinfo.Type.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Int:
			val, _ := constant.Int64Val(ctx.tinfo.Value)
			c.sb.emitPushInt(val)
		case types.String:
			val := constant.StringVal(ctx.tinfo.Value)
			c.sb.emitPushString(val)
		case types.Bool, types.UntypedBool:
			val := constant.BoolVal(ctx.tinfo.Value)
			c.sb.emitPushBool(val)
		}
	default:
		log.Fatalf("compiler don't know how to handle this => %v", ctx)
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
		log.Fatalf("want to load local %v but got invalid position => %d <=", ctx, pos)
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
		log.Fatalf("want to store local %v but got invalid positionl => %d", vctx, pos)
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

	// Typechecker
	_, err = conf.Check("", fset, []*ast.File{f}, typeInfo)
	if err != nil {
		log.Fatal(err)
	}

	var main *ast.FuncDecl
	ast.Inspect(f, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.FuncDecl:
			if t.Name.Name == mainIdent {
				main = t
				return false
			}
		}
		return true
	})
	if main == nil {
		log.Fatal("could not find func main. did you forgot to declare it?")
	}

	c.resolveFuncDecls(f)
	c.convertFuncDecl(main)

	// Start building all declarations
	for _, decl := range f.Decls {
		switch t := decl.(type) {
		case *ast.GenDecl:
		case *ast.FuncDecl:
			if t.Name.Name != mainIdent {
				c.convertFuncDecl(t)
			}
		}
	}

	// update all local function calls.
	c.updateFuncCalls()

	return nil
}

// updateFuncCalls will update all local function calls that occured the program.
func (c *Compiler) updateFuncCalls() {
	for _, ctx := range c.funcCalls {
		fun, ok := c.funcs[ctx.funcName]
		if !ok {
			log.Fatalf("could not resolve function %s", ctx.funcName)
		}
		// pos is the position of the call op, we need to add 1 to get the
		// start of the label.
		// for calculating the correct offset we need to subtract the target label
		// with the position the call occured.
		offset := fun.label - int16(ctx.pos)
		c.sb.updatePushCall(ctx.pos+1, offset)
	}
}

func (c *Compiler) resolveFuncDecls(f *ast.File) {
	for _, decl := range f.Decls {
		switch t := decl.(type) {
		case *ast.GenDecl:
		case *ast.FuncDecl:
			if t.Name.Name != mainIdent {
				c.funcs[t.Name.Name] = newFuncContext(t, 0)
			}
		}
	}
}

func (c *Compiler) convertFuncDecl(decl *ast.FuncDecl) {
	ctx := newFuncContext(decl, c.currentPos())
	c.funcs[ctx.name] = ctx

	// We need to write the the total stack size of the function first.
	// That size is the number of arguments + body operations  that will be
	// pushed on the stack
	c.sb.emitPushInt(ctx.numStackOps())
	c.sb.emitPush(vm.OpNewArray)
	c.sb.emitPush(vm.OpToAltStack)

	// Load the arguments into scope.
	for _, arg := range decl.Type.Params.List {
		name := arg.Names[0].Name
		ctx.args[name] = true
		vctx := ctx.newConst(name, c.getTypeInfo(arg.Type), true)
		c.storeLocal(vctx)
	}

	for _, stmt := range decl.Body.List {
		c.convertStmt(ctx, stmt)
	}
}

func (c *Compiler) convertStmt(fctx *FuncContext, stmt ast.Stmt) {
	switch t := stmt.(type) {
	case *ast.AssignStmt:
		for i := 0; i < len(t.Lhs); i++ {
			lhs := t.Lhs[i].(*ast.Ident)

			switch rhs := t.Rhs[i].(type) {
			case *ast.BasicLit:
				vctx := fctx.newConst(lhs.Name, c.getTypeInfo(t.Rhs[i]), true)
				c.loadConst(vctx, true)

			case *ast.CompositeLit:
				// Write constants in reverse order on the stack.
				n := len(rhs.Elts)
				for i := n - 1; i >= 0; i-- {
					vctx := fctx.newConst("", c.getTypeInfo(rhs.Elts[i]), false)
					c.loadConst(vctx, false)
				}

				c.sb.emitPushInt(int64(n))
				c.sb.emitPush(vm.OpPack)

				vctx := fctx.newConst(lhs.Name, c.getTypeInfo(rhs), true)
				c.storeLocal(vctx)

			case *ast.Ident:
				if isIdentBool(rhs) {
					vctx := fctx.newConst(lhs.Name, makeBoolFromIdent(rhs, c.typeInfo), true)
					c.loadConst(vctx, true)
					continue
				}

				knownCtx := fctx.getContext(rhs.Name)
				c.loadLocal(knownCtx)
				newCtx := fctx.newConst(lhs.Name, c.getTypeInfo(rhs), true)
				c.storeLocal(newCtx)

			default:
				c.convertExpr(fctx, t.Rhs[i])
				vctx := fctx.newConst(lhs.Name, c.getTypeInfo(t.Rhs[i]), true)
				c.storeLocal(vctx)
			}
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

	// TODO: this needs a rewrite ASAP.
	case *ast.IfStmt:
		c.convertExpr(fctx, t.Cond)

		binExpr, ok := t.Cond.(*ast.BinaryExpr)
		if ok && binExpr.Op != token.LAND && binExpr.Op != token.LOR {
			// use a placeholder for the label.
			c.sb.emitJump(vm.OpJMPIFNOT, int16(0))
			// track our offset to update later subtract sizeOf int16.
			offset := int(c.currentPos()) - 2

			defer func(offset int) {
				jumpTo := c.currentPos() + 1 - int16(offset)
				c.sb.updateJmpLabel(jumpTo, offset)
			}(offset)
		}

		labelBeforeBlock := c.currentPos()
		// Process the block.
		for _, stmt := range t.Body.List {
			c.convertStmt(fctx, stmt)
		}

		// if there are any labels we need to update.
		if len(fctx.jumpLabels) > 0 {
			for _, label := range fctx.jumpLabels {
				var pos int16
				if label.op == vm.OpJMPIF {
					pos = labelBeforeBlock + 1
				} else {
					pos = c.currentPos() + 1
				}
				jumpTo := pos - int16(label.offset)
				c.sb.updateJmpLabel(jumpTo, label.offset)
			}
			fctx.jumpLabels = []jumpLabel{}
		}

	default:
		log.Fatalf("compiler has not implemented this statement => %v", reflect.TypeOf(t))
	}
}

func (c *Compiler) convertExpr(fctx *FuncContext, expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.BasicLit:
		vctx := fctx.newConst("", c.getTypeInfo(t), false)
		c.loadConst(vctx, false)

	case *ast.Ident:
		if isIdentBool(t) {
			vctx := fctx.newConst(t.Name, makeBoolFromIdent(t, c.typeInfo), false)
			c.loadConst(vctx, false)
			return
		}
		if fctx.isArgument(t.Name) {
			vctx := fctx.getContext(t.Name)
			c.loadLocal(vctx)
			return
		}
		vctx := fctx.getContext(t.Name)
		c.loadLocal(vctx)

	case *ast.CallExpr:
		fun := t.Fun.(*ast.Ident)
		fctx, ok := c.funcs[fun.Name]
		if !ok {
			log.Fatalf("could not resolve func %s", fun.Name)
		}

		// handle the passed arguments.
		for _, arg := range t.Args {
			vctx := fctx.newConst("", c.getTypeInfo(arg), false)
			c.loadLocal(vctx)
		}

		// c# compiler adds a NOP (0x61) before every function call. Dont think its relevant
		// and we could easily removed it, but to be consistent with the original compiler I
		// will put them in. ^^
		c.sb.emitPush(vm.OpNOP)

		c.funcCalls = append(c.funcCalls, CallContext{int(c.currentPos()), fun.Name})
		c.sb.emitPushCall(0) // placeholder, update later.

	case *ast.BinaryExpr:
		if t.Op == token.LAND || t.Op == token.LOR {
			c.convertExpr(fctx, t.X)

			opJMP := vm.OpJMPIFNOT
			if t.Op == token.LOR {
				opJMP = vm.OpJMPIF
			}

			if e, ok := t.X.(*ast.BinaryExpr); ok && e.Op != token.LAND && e.Op != token.LOR {
				c.sb.emitJump(opJMP, int16(0))
				fctx.addJump(opJMP, int(c.currentPos())-2)
			}

			c.convertExpr(fctx, t.Y)
			c.sb.emitJump(vm.OpJMPIFNOT, int16(0))
			fctx.addJump(vm.OpJMPIFNOT, int(c.currentPos())-2)
			c.convertToken(t.Op)
			return
		}

		// The AST package resolves all basic literals for us. If the typeinfo.Value is not nil
		// we know that the bin expr is resolved and needs no further action.
		// e.g. x := 2 + 2 + 2 will be resolved to 6.
		if tinfo := c.getTypeInfo(t); tinfo.Value != nil {
			vctx := fctx.newConst("", tinfo, false)
			c.loadConst(vctx, false)
			return
		}

		c.convertExpr(fctx, t.X)
		c.convertExpr(fctx, t.Y)
		c.convertToken(t.Op)

	default:
		log.Fatalf("compiler has not implemented this expr => %v", reflect.TypeOf(t))
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

// Buffer returns the buffer of the builder as a io.Reader.
func (c *Compiler) Buffer() *bytes.Buffer {
	return c.sb.buf
}

// DumpOpcode dumps the current buffer, formatted with index, hex and opcode.
func (c *Compiler) DumpOpcode() {
	c.sb.dumpOpcode()
}

func makeBoolFromIdent(ident *ast.Ident, tinfo *types.Info) types.TypeAndValue {
	var b bool
	if ident.Name == "true" {
		b = true
	} else if ident.Name == "false" {
		b = false
	} else {
		log.Fatalf("givent identifier cannot be converted to a boolean => %s", ident.Name)
	}

	return types.TypeAndValue{
		Type:  tinfo.ObjectOf(ident).Type(),
		Value: constant.MakeBool(b),
	}
}

func isIdentBool(ident *ast.Ident) bool {
	return ident.Name == "true" || ident.Name == "false"
}
