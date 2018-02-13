package compiler

import (
	"bytes"
	"fmt"
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
	ctx := newFuncContext(decl.Name.Name)
	c.funcs[ctx.name] = ctx

	c.sb.emitPush(vm.OpPush2)
	c.sb.emitPush(vm.OpNewArray)
	c.sb.emitPush(vm.OpToAltStack)

	for _, stmt := range decl.Body.List {
		c.convertStmt(ctx, stmt)
	}
}

func (c *Compiler) convertStmt(fctx *FuncContext, stmt ast.Stmt) {
	switch t := stmt.(type) {
	case *ast.AssignStmt:
		for i := 0; i < len(t.Lhs); i++ {
			lhs := t.Lhs[i].(*ast.Ident)
			c.convertExpr(fctx, t.Rhs[i], lhs.Name)
			fctx.registerContext(lhs.Name)
			c.storeLocal(fctx.currentCtx)
		}

	case *ast.ReturnStmt:

	}
}

func (c *Compiler) convertExpr(fctx *FuncContext, expr ast.Expr, ident string) {
	switch t := expr.(type) {
	case *ast.BasicLit:
		fctx.currentCtx = newVarContext("", c.getTypeInfo(expr))
		c.loadConst(fctx.currentCtx, false)

	case *ast.Ident:
		fctx.getContext(ident)
		fmt.Println("load local")

	case *ast.BinaryExpr:
		if tinfo := c.getTypeInfo(t); tinfo.Value != nil {
			fmt.Printf("load const bin %v\n", tinfo)
			//fmt.Println(t.Y)
			return
		}

		//fmt.Printf("converting %v\n", reflect.TypeOf(t))
		c.convertExpr(fctx, t.X)
		c.convertExpr(fctx, t.Y)
		fmt.Printf("op %v\n", t.Op)
	}
}

// func (c *Compiler) convertStmt(fun *FuncContext, stmt ast.Stmt) {
// 	switch t := stmt.(type) {
// 	case *ast.AssignStmt:
// 		for i := 0; i < len(t.Lhs); i++ {
// 			lhs := t.Lhs[i].(*ast.Ident)
// 			c.convertExpr(fun, t.Rhs[i], lhs)
// 		}
// 	case *ast.IfStmt:
// 		switch t.Cond.(type) {
// 		case *ast.BinaryExpr:
// 			fmt.Println("binexpr")
// 			// 	lhs := funcCtx.varContextFromExpr(t.X, c.typeInfo)
// 			// 	rhs := funcCtx.varContextFromExpr(t.Y, c.typeInfo)

// 			// 	// lhs
// 			// 	if funcCtx.isRegistered(lhs) {
// 			// 		c.loadLocal(lhs)
// 			// 	} else {
// 			// 		c.sb.emitPushInt(lhs.value.(int64))
// 			// 	}

// 			// 	// rhs
// 			// 	if funcCtx.isRegistered(rhs) {
// 			// 		c.loadLocal(rhs)
// 			// 	} else {
// 			// 		c.sb.emitPushInt(rhs.value.(int64))
// 			// 	}

// 			// 	switch t.Op {
// 			// 	case token.LSS:
// 			// 		c.sb.emitPush(vm.OpLT)
// 			// 	case token.LEQ:
// 			// 		c.sb.emitPush(vm.OpLTE)
// 			// 	case token.GTR:
// 			// 		c.sb.emitPush(vm.OpGT)
// 			// 	case token.GEQ:
// 			// 		c.sb.emitPush(vm.OpGTE)
// 			// 	}
// 			// }

// 			// // We need to know where to jump if this stmt is false.
// 			// // We write the jump with a label placeholder and update it
// 			// // after we converted the block of the statement.
// 			// c.sb.emitJump(vm.OpJMPIFNOT, int16(0))
// 			// jumpOffset := int(c.currentPos()) - 2

// 			// // convert the if block
// 			// for _, stmt := range t.Body.List {
// 			// 	c.convertStmt(funcCtx, stmt)
// 			// }

// 			// // now update the jump label
// 			// jumpTo := c.currentPos() + 1 - int16(jumpOffset)
// 			// c.sb.updateJmpLabel(jumpTo, jumpOffset)
// 		}
// 	case *ast.ReturnStmt:
// 		// Due to the design of the orginal VM, multiple return are not supported.
// 		if len(t.Results) > 1 {
// 			log.Fatal("multiple returns not supported.")
// 		}

// 		c.sb.emitPush(vm.OpJMP)
// 		c.sb.emitPush(vm.OpCode(0x03))
// 		c.sb.emitPush(vm.OpPush0)

// 		c.convertExpr(fun, t.Results[0], nil)

// 		c.sb.emitPush(vm.OpNOP)
// 		c.sb.emitPush(vm.OpFromAltStack)
// 		c.sb.emitPush(vm.OpDrop)
// 		c.sb.emitPush(vm.OpRET)
// 	}
// }

// func (c *Compiler) convertExpr(fctx *FuncContext, expr ast.Expr, lhs *ast.Ident) {
// 	switch t := expr.(type) {
// 	case *ast.BasicLit:
// 		vctx := newVarContext(c.getTypeInfo(expr))
// 		if lhs != nil {
// 			vctx.name = lhs.Name
// 			fctx.registerContext(vctx, true)
// 			c.convertVar(vctx, true)
// 			return
// 		}
// 		c.convertVar(vctx, false)
// 	case *ast.Ident:
// 		knownCtx := fctx.getContext(t.Name)
// 		c.loadLocal(knownCtx)
// 		if lhs != nil {
// 			vctx := newVarContext(c.getTypeInfo(t))
// 			vctx.name = lhs.Name
// 			fctx.registerContext(vctx, true)
// 			c.storeLocal(vctx)
// 		}
// 	case *ast.BinaryExpr:
// 		ast.Inspect(t, func(n ast.Node) bool {
// 			if n == nil {
// 				return false
// 			}
// 			fmt.Println(reflect.TypeOf(n))
// 			return true
// 		})
// 		// if tinfo := c.getTypeInfo(t); tinfo.Value != nil {
// 		// 	vctx := newVarContext(tinfo)
// 		// 	if lhs != nil {
// 		// 		vctx.name = lhs.Name
// 		// 		fctx.registerContext(vctx, true)
// 		// 		c.convertVar(vctx, true)
// 		// 		return
// 		// 	}
// 		// 	c.convertVar(vctx, false)
// 		// 	return
// 		// }

// 		// vctx := newVarContext(c.getTypeInfo(t.X))
// 		// if lhs != nil {
// 		// 	vctx.name = lhs.Name
// 		// 	fctx.registerContext(vctx, true)
// 		// }

// 		// // Lets do this all in one pass lads.
// 		// var pushOP = false
// 		// switch t.X.(type) {
// 		// case *ast.BasicLit, *ast.Ident, *ast.BinaryExpr:
// 		// 	switch t.Y.(type) {
// 		// 	case *ast.BasicLit, *ast.Ident, *ast.BinaryExpr:
// 		// 		pushOP = true
// 		// 	}
// 		// }

// 		// c.convertExpr(fctx, t.X, nil)
// 		// c.convertExpr(fctx, t.Y, nil)

// 		// if pushOP {
// 		// 	c.convertToken(t.Op)
// 		// 	c.storeLocal(vctx)
// 		// }
// 	}
// }

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
	}
}

func (c *Compiler) convertBinExpr(expr *ast.BinaryExpr) {
	fmt.Println(c.getTypeInfo(expr.X))
	fmt.Println(c.getTypeInfo(expr.Y))
}

func (c *Compiler) getTypeInfo(expr ast.Expr) types.TypeAndValue {
	return c.typeInfo.Types[expr]
}

func (c *Compiler) currentPos() int16 {
	return int16(c.sb.buf.Len())
}

// DumpOpcode dumps the current buffer, formatted with index, hex and opcode.
func (c *Compiler) DumpOpcode() {
	c.sb.dumpOpcode()
}
