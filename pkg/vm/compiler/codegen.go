package compiler

import (
	"bytes"
	"encoding/binary"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"log"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

const mainIdent = "Main"

type codegen struct {
	prog *bytes.Buffer

	// Type information
	typeInfo *types.Info

	// a mapping of func identifiers with its scope.
	funcs map[string]*funcScope

	// current function being generated
	fctx *funcScope

	// Label table for recording jump destinations.
	l []int
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
	pos := c.fctx.loadLocal(name)
	if pos < 0 {
		log.Fatalf("cannot load local variable with position: %d", pos)
	}

	emitOpcode(c.prog, vm.Ofromaltstack)
	emitOpcode(c.prog, vm.Odup)
	emitOpcode(c.prog, vm.Otoaltstack)

	emitInt(c.prog, int64(pos))
	emitOpcode(c.prog, vm.Opickitem)
}

func (c *codegen) emitLoadStructField(sName, fName string) {
	strct := c.fctx.loadStruct(sName)
	pos := strct.loadField(fName)
	emitInt(c.prog, int64(pos))
	emitOpcode(c.prog, vm.Opickitem)
}

func (c *codegen) emitStoreLocal(pos int) {
	emitOpcode(c.prog, vm.Ofromaltstack)
	emitOpcode(c.prog, vm.Odup)
	emitOpcode(c.prog, vm.Otoaltstack)

	if pos < 0 {
		log.Fatalf("invalid position to store local: %d", pos)
	}

	emitInt(c.prog, int64(pos))
	emitInt(c.prog, 2)
	emitOpcode(c.prog, vm.Oroll)
	emitOpcode(c.prog, vm.Osetitem)
}

func (c *codegen) emitStoreStructField(sName, fName string) {
	strct := c.fctx.loadStruct(sName)
	pos := strct.loadField(fName)
	emitInt(c.prog, int64(pos))
	emitOpcode(c.prog, vm.Orot)
	emitOpcode(c.prog, vm.Osetitem)
}

func (c *codegen) convertFuncDecl(decl *ast.FuncDecl) {
	var (
		f  *funcScope
		ok bool
	)

	f, ok = c.funcs[decl.Name.Name]
	if ok {
		c.setLabel(f.label)
	} else {
		f = c.newFunc(decl)
	}
	c.fctx = f

	emitInt(c.prog, f.stackSize())
	emitOpcode(c.prog, vm.Onewarray)
	emitOpcode(c.prog, vm.Otoaltstack)

	// We need to handle methods, which in Go, is just syntactic sugar.
	// The method receiver will be passed in as first argument.
	// We check if this declaration has a receiver and load it into scope.
	//
	// FIXME: For now we will hard cast this to a struct. We can later finetune this
	// to support other types.
	if decl.Recv != nil {
		for _, arg := range decl.Recv.List {
			strct := c.fctx.newStruct()

			ident := arg.Names[0]
			strct.initializeFields(ident, c.typeInfo)
			l := c.fctx.newLocal(ident.Name)
			c.emitStoreLocal(l)
		}
	}

	// Load the arguments in scope.
	for _, arg := range decl.Type.Params.List {
		name := arg.Names[0].Name // for now.
		l := c.fctx.newLocal(name)
		c.emitStoreLocal(l)
	}

	ast.Walk(c, decl.Body)
}

func (c *codegen) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {

	case *ast.AssignStmt:
		for i := 0; i < len(n.Lhs); i++ {
			// resolve the whole right hand side.
			ast.Walk(c, n.Rhs[i])
			// check if we are assigning to a struct or an identifier
			switch t := n.Lhs[i].(type) {
			case *ast.Ident:
				l := c.fctx.loadLocal(t.Name)
				c.emitStoreLocal(l)

			case *ast.SelectorExpr:
				switch n := t.X.(type) {
				case *ast.Ident:
					c.emitLoadLocal(n.Name)                    // load the struct
					c.emitStoreStructField(n.Name, t.Sel.Name) // store the field
				default:
					log.Fatal("nested selector assigns not supported yet")
				}
			}
		}
		return nil

	case *ast.ReturnStmt:
		if len(n.Results) > 1 {
			log.Fatal("multiple returns not supported.")
		}

		emitOpcode(c.prog, vm.Ojmp)
		emitOpcode(c.prog, vm.Opcode(0x03))
		emitOpcode(c.prog, vm.Opush0)

		if len(n.Results) > 0 {
			ast.Walk(c, n.Results[0])
		}

		emitOpcode(c.prog, vm.Onop)
		emitOpcode(c.prog, vm.Ofromaltstack)
		emitOpcode(c.prog, vm.Odrop)
		emitOpcode(c.prog, vm.Oret)
		return nil

	case *ast.IfStmt:
		lIf := c.newLabel()
		lElse := c.newLabel()
		if n.Cond != nil {
			ast.Walk(c, n.Cond)
			emitJmp(c.prog, vm.Ojmpifnot, int16(lElse))
		}

		c.setLabel(lIf)
		ast.Walk(c, n.Body)

		if n.Else != nil {
			// TODO: handle else statements.
			// emitJmp(c.prog, vm.Ojmp, int16(lEnd))
		}
		c.setLabel(lElse)
		if n.Else != nil {
			ast.Walk(c, n.Else)
		}
		return nil

	case *ast.BasicLit:
		c.emitLoadConst(c.getTypeInfo(n))
		return nil

	case *ast.Ident:
		if isIdentBool(n) {
			c.emitLoadConst(makeBoolFromIdent(n, c.typeInfo))
		} else {
			c.emitLoadLocal(n.Name)
		}
		return nil

	case *ast.CompositeLit:
		switch t := n.Type.(type) {
		case *ast.Ident:
			typ := c.typeInfo.ObjectOf(t).Type().Underlying()
			switch typ.(type) {
			case *types.Struct:
				c.convertStruct(n)
			}

		default:
			ln := len(n.Elts)
			for i := ln - 1; i >= 0; i-- {
				c.emitLoadConst(c.getTypeInfo(n.Elts[i]))
			}
			emitInt(c.prog, int64(ln))
			emitOpcode(c.prog, vm.Opack)
		}
		return nil

	case *ast.BinaryExpr:
		switch n.Op {
		case token.LAND:
			ast.Walk(c, n.X)
			emitJmp(c.prog, vm.Ojmpifnot, int16(len(c.l)-1))
			ast.Walk(c, n.Y)
			return nil

		case token.LOR:
			ast.Walk(c, n.X)
			emitJmp(c.prog, vm.Ojmpif, int16(len(c.l)-2))
			ast.Walk(c, n.Y)
			return nil

		default:
			// The AST package will try to resolve all basic literals for us.
			// If the typeinfo.Value is not nil we know that the expr is resolved
			// and needs no further action. e.g. x := 2 + 2 + 2 will be resolved to 6.
			if tinfo := c.getTypeInfo(n); tinfo.Value != nil {
				c.emitLoadConst(tinfo)
				return c
			}

			ast.Walk(c, n.X)
			ast.Walk(c, n.Y)

			switch n.Op {
			case token.ADD:
				emitOpcode(c.prog, vm.Oadd)
			case token.SUB:
				emitOpcode(c.prog, vm.Osub)
			case token.MUL:
				emitOpcode(c.prog, vm.Omul)
			case token.QUO:
				emitOpcode(c.prog, vm.Odiv)
			case token.LSS:
				emitOpcode(c.prog, vm.Olt)
			case token.LEQ:
				emitOpcode(c.prog, vm.Olte)
			case token.GTR:
				emitOpcode(c.prog, vm.Ogt)
			case token.GEQ:
				emitOpcode(c.prog, vm.Ogte)
			}
			return nil
		}

	case *ast.CallExpr:
		var (
			f       *funcScope
			ok      bool
			numArgs = len(n.Args)
		)

		switch fun := n.Fun.(type) {
		case *ast.Ident:
			f, ok = c.funcs[fun.Name]
			if !ok {
				log.Fatalf("could not resolve function %s", fun.Name)
			}
		case *ast.SelectorExpr:
			ast.Walk(c, fun.X)
			f, ok = c.funcs[fun.Sel.Name]
			if !ok {
				log.Fatalf("could not resolve function %s", fun.Sel.Name)
			}
			// Dont forget to add 1 extra argument when its a method.
			numArgs++
		}

		for _, arg := range n.Args {
			ast.Walk(c, arg)
		}
		if numArgs == 2 {
			emitOpcode(c.prog, vm.Oswap)
		}
		if numArgs == 3 {
			emitInt(c.prog, 2)
			emitOpcode(c.prog, vm.Oxswap)
		}

		// c# compiler adds a NOP (0x61) before every function call. Dont think its relevant
		// and we could easily removed it, but to be consistent with the original compiler I
		// will put them in. ^^
		emitOpcode(c.prog, vm.Onop)
		emitCall(c.prog, vm.Ocall, int16(f.label))
		return nil

	case *ast.SelectorExpr:
		switch t := n.X.(type) {
		case *ast.Ident:
			c.emitLoadLocal(t.Name)                   // load the struct
			c.emitLoadStructField(t.Name, n.Sel.Name) // load the field
		default:
			log.Fatal("nested selectors not supported yet")
		}
		return nil
	}
	return c
}

func (c *codegen) convertStruct(lit *ast.CompositeLit) {
	emitOpcode(c.prog, vm.Onop)
	emitInt(c.prog, int64(len(lit.Elts)))
	emitOpcode(c.prog, vm.Onewstruct)
	emitOpcode(c.prog, vm.Otoaltstack)

	// Create a new struct scope to store the positions of its variables.
	strct := c.fctx.newStruct()

	for _, field := range lit.Elts {
		f := field.(*ast.KeyValueExpr)
		// Walk to resolve the expression of the value.
		ast.Walk(c, f.Value)
		l := strct.newField(f.Key.(*ast.Ident).Name)
		c.emitStoreLocal(l)
	}
	emitOpcode(c.prog, vm.Ofromaltstack)
}

func (c *codegen) newFunc(decl *ast.FuncDecl) *funcScope {
	f := newFuncScope(decl, c.newLabel())
	c.funcs[f.name] = f
	return f
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

// CodeGen is the function that compiles the program to bytecode.
func CodeGen(f *ast.File, tInfo *types.Info) (*bytes.Buffer, error) {
	c := &codegen{
		prog:     new(bytes.Buffer),
		l:        []int{},
		funcs:    map[string]*funcScope{},
		typeInfo: tInfo,
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

	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			if n.Name.Name != mainIdent {
				c.convertFuncDecl(n)
			}
		}
	}

	c.writeJumps()

	return c.prog, nil
}

func (c *codegen) resolveFuncDecls(f *ast.File) {
	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			if n.Name.Name != mainIdent {
				c.newFunc(n)
			}
		}
	}
}

func (c *codegen) writeJumps() {
	b := c.prog.Bytes()
	for i, op := range b {
		j := i + 1
		switch vm.Opcode(op) {
		case vm.Ojmpifnot, vm.Ojmpif, vm.Ocall:
			index := binary.LittleEndian.Uint16(b[j : j+2])
			if int(index) > len(c.l) {
				continue
			}
			offset := uint16(c.l[index] - i)
			if offset < 0 {
				log.Fatalf("new offset is negative, table list %v", c.l)
			}
			binary.LittleEndian.PutUint16(b[j:j+2], offset)
		}
	}
}
