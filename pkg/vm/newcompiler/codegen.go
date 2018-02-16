package newcompiler

import (
	"bytes"
	"encoding/binary"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
)

type codegen struct {
	prog *bytes.Buffer

	typeInfo *types.Info // Type information

	funcs map[string]*funcScope

	fctx *funcScope // current function being generated

	l []int // Label table for recording jump destinations.
}

// newLabel creates a new label to jump to
func (c *codegen) newLabel() (l int) {
	l = len(c.l)
	c.l = append(c.l, -1)
	return
}

func (c *codegen) setLabel(l int) {
	c.l[l] = c.pc() + 1
}

// pc return the program offset off the last instruction.
func (c *codegen) pc() int {
	return c.prog.Len() - 1
}

func (c *codegen) emitLoadConst(t types.TypeAndValue) {
	switch typ := t.Type.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.Int:
			val, _ := constant.Int64Val(t.Value)
			emitInt(c.prog, val)
		case types.String:
			val := constant.StringVal(t.Value)
			emitString(c.prog, val)
		case types.Bool, types.UntypedBool:
			val := constant.BoolVal(t.Value)
			emitBool(c.prog, val)
		}
	default:
		log.Fatalf("compiler don't know how to convert this constant: %v", t)
	}
}

func (c *codegen) emitLoadLocal(name string) {
	pos := c.fctx.loadVar(name)
	if pos < 0 {
		log.Fatalf("cannot load local variable with position: %d", pos)
	}

	emitOpcode(c.prog, Ofromaltstack)
	emitOpcode(c.prog, Odup)
	emitOpcode(c.prog, Otoaltstack)

	emitInt(c.prog, int64(pos))
	emitOpcode(c.prog, Opickitem)
}

func (c *codegen) emitStoreLocal(pos int) {
	emitOpcode(c.prog, Ofromaltstack)
	emitOpcode(c.prog, Odup)
	emitOpcode(c.prog, Otoaltstack)

	if pos < 0 {
		log.Fatalf("invalid position to store local: %d", pos)
	}

	emitInt(c.prog, int64(pos))
	emitInt(c.prog, 2)
	emitOpcode(c.prog, Oroll)
	emitOpcode(c.prog, Osetitem)
}

func (c *codegen) convertFuncDecl(decl *ast.FuncDecl) {
	f := c.newFunc(decl)

	emitInt(c.prog, f.stackSize())
	emitOpcode(c.prog, Onewarray)
	emitOpcode(c.prog, Otoaltstack)

	for _, stmt := range decl.Body.List {
		c.convertStmt(stmt)
	}
}

func (c *codegen) convertStmt(stmt ast.Stmt) {
	switch n := stmt.(type) {

	case *ast.BlockStmt:
		for _, stmt := range n.List {
			c.convertStmt(stmt)
		}

	case *ast.AssignStmt:
		for i := 0; i < len(n.Lhs); i++ {
			lhs := n.Lhs[i].(*ast.Ident)
			l := c.newLocal(lhs.Name)
			switch rhs := n.Rhs[i].(type) {
			case *ast.BasicLit:
				c.emitLoadConst(c.getTypeInfo(rhs))
			case *ast.Ident:
				c.emitLoadLocal(rhs.Name)
			default:
				c.convertExpr(rhs)
			}
			c.emitStoreLocal(l)
		}

	case *ast.ReturnStmt:
		if len(n.Results) > 1 {
			log.Fatal("multiple returns not supported.")
		}

		emitOpcode(c.prog, Ojmp)
		emitOpcode(c.prog, Opcode(0x03))
		emitOpcode(c.prog, Opush0)

		if len(n.Results) > 0 {
			c.convertExpr(n.Results[0])
		}

		emitOpcode(c.prog, Onop)
		emitOpcode(c.prog, Ofromaltstack)
		emitOpcode(c.prog, Odrop)
		emitOpcode(c.prog, Oret)

	case *ast.IfStmt:
		lEnd := c.newLabel()
		lElse := c.newLabel()
		if n.Cond != nil {
			c.convertExpr(n.Cond)
			emitJmp(c.prog, Ojmpifnot, int16(lElse))
		}

		c.convertStmt(n.Body)

		c.setLabel(lElse)
		if n.Else != nil {
			c.convertStmt(n.Else)
		}
		c.setLabel(lEnd)
	}
}

func (c *codegen) convertExpr(expr ast.Expr) {
	switch n := expr.(type) {
	case *ast.BasicLit:
		c.emitLoadConst(c.getTypeInfo(n))

	case *ast.Ident:
		c.emitLoadLocal(n.Name)

	case *ast.BinaryExpr:
		switch n.Op {

		case token.LAND:
			c.convertExpr(n.X)
			emitJmp(c.prog, Ojmpifnot, int16(0))
			c.convertExpr(n.Y)

		case token.LOR:
			c.convertExpr(n.X)
			emitJmp(c.prog, Ojmpif, int16(0))
			c.convertExpr(n.Y)

		default:
			// The AST package will try to resolve all basic literals for us.
			// If the typeinfo.Value is not nil we know that the expr is resolved
			// and needs no further action. e.g. x := 2 + 2 + 2 will be resolved to 6.
			if tinfo := c.getTypeInfo(n); tinfo.Value != nil {
				c.emitLoadConst(tinfo)
				return
			}

			c.convertExpr(n.X)
			c.convertExpr(n.Y)

			switch n.Op {
			case token.ADD:
				emitOpcode(c.prog, Oadd)
			case token.SUB:
				emitOpcode(c.prog, Osub)
			case token.MUL:
				emitOpcode(c.prog, Omul)
			case token.QUO:
				emitOpcode(c.prog, Odiv)
			case token.LSS:
				emitOpcode(c.prog, Olt)
			case token.LEQ:
				emitOpcode(c.prog, Olte)
			case token.GTR:
				emitOpcode(c.prog, Ogt)
			case token.GEQ:
				emitOpcode(c.prog, Ogte)
			}
		}
	}
}

func (c *codegen) newFunc(decl *ast.FuncDecl) *funcScope {
	fctx := newFuncScope(decl, c.pc())
	c.funcs[fctx.name] = fctx
	c.fctx = fctx
	return fctx
}

func (c *codegen) newLocal(name string) int {
	return c.fctx.newVar(name)
}

func (c *codegen) getTypeInfo(expr ast.Expr) types.TypeAndValue {
	return c.typeInfo.Types[expr]
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

// Compile is the function that compiles the program to bytecode.
func Compile(r io.Reader) (*bytes.Buffer, error) {
	c := &codegen{
		prog:  new(bytes.Buffer),
		l:     []int{},
		funcs: map[string]*funcScope{},
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", r, 0)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			c.convertFuncDecl(n)
		}
	}

	c.writeJumps()

	return c.prog, nil
}

func (c *codegen) writeJumps() {
	b := c.prog.Bytes()
	for i, op := range b {
		switch Opcode(op) {
		case Ojmpifnot:
			prevOp := Opcode(b[i-1])
			// Make sure this is a real jump by checking the previous opcode.
			switch prevOp {
			case Olte, Olt, Ogte, Ogt:
				offset := i + 1
				index := binary.LittleEndian.Uint16(b[offset : offset+2])
				newLabel := uint16(c.l[index] - i)
				binary.LittleEndian.PutUint16(b[offset:offset+2], newLabel)
			}
		}
	}
}
