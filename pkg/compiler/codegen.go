package compiler

import (
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

// The identifier of the entry function. Default set to Main.
const mainIdent = "Main"

type codegen struct {
	// Information about the program with all its dependencies.
	buildInfo *buildInfo

	// prog holds the output buffer.
	prog *io.BufBinWriter

	// Type information.
	typeInfo *types.Info

	// A mapping of func identifiers with their scope.
	funcs map[string]*funcScope

	// Current funcScope being converted.
	scope *funcScope

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

// pc returns the program offset off the last instruction.
func (c *codegen) pc() int {
	return c.prog.Len() - 1
}

func (c *codegen) emitLoadConst(t types.TypeAndValue) {
	if c.prog.Err != nil {
		return
	}
	switch typ := t.Type.Underlying().(type) {
	case *types.Basic:
		c.convertBasicType(t, typ)
	default:
		c.prog.Err = fmt.Errorf("compiler doesn't know how to convert this constant: %v", t)
		return
	}
}

func (c *codegen) convertBasicType(t types.TypeAndValue, typ *types.Basic) {
	switch typ.Kind() {
	case types.Int, types.UntypedInt, types.Uint:
		val, _ := constant.Int64Val(t.Value)
		emitInt(c.prog.BinWriter, val)
	case types.String, types.UntypedString:
		val := constant.StringVal(t.Value)
		emitString(c.prog.BinWriter, val)
	case types.Bool, types.UntypedBool:
		val := constant.BoolVal(t.Value)
		emitBool(c.prog.BinWriter, val)
	case types.Byte:
		val, _ := constant.Int64Val(t.Value)
		b := byte(val)
		emitBytes(c.prog.BinWriter, []byte{b})
	default:
		c.prog.Err = fmt.Errorf("compiler doesn't know how to convert this basic type: %v", t)
		return
	}
}

func (c *codegen) emitLoadLocal(name string) {
	pos := c.scope.loadLocal(name)
	if pos < 0 {
		c.prog.Err = fmt.Errorf("cannot load local variable with position: %d", pos)
		return
	}
	c.emitLoadLocalPos(pos)
}

func (c *codegen) emitLoadLocalPos(pos int) {
	emitOpcode(c.prog.BinWriter, opcode.DUPFROMALTSTACK)
	emitInt(c.prog.BinWriter, int64(pos))
	emitOpcode(c.prog.BinWriter, opcode.PICKITEM)
}

func (c *codegen) emitStoreLocal(pos int) {
	emitOpcode(c.prog.BinWriter, opcode.DUPFROMALTSTACK)

	if pos < 0 {
		c.prog.Err = fmt.Errorf("invalid position to store local: %d", pos)
		return
	}

	emitInt(c.prog.BinWriter, int64(pos))
	emitOpcode(c.prog.BinWriter, opcode.ROT)
	emitOpcode(c.prog.BinWriter, opcode.SETITEM)
}

func (c *codegen) emitLoadField(i int) {
	emitInt(c.prog.BinWriter, int64(i))
	emitOpcode(c.prog.BinWriter, opcode.PICKITEM)
}

func (c *codegen) emitStoreStructField(i int) {
	emitInt(c.prog.BinWriter, int64(i))
	emitOpcode(c.prog.BinWriter, opcode.ROT)
	emitOpcode(c.prog.BinWriter, opcode.SETITEM)
}

// convertGlobals traverses the AST and only converts global declarations.
// If we call this in convertFuncDecl then it will load all global variables
// into the scope of the function.
func (c *codegen) convertGlobals(f ast.Node) {
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			return false
		case *ast.GenDecl:
			ast.Walk(c, n)
		}
		return true
	})
}

func (c *codegen) convertFuncDecl(file ast.Node, decl *ast.FuncDecl) {
	var (
		f  *funcScope
		ok bool
	)

	f, ok = c.funcs[decl.Name.Name]
	if ok {
		// If this function is a syscall we will not convert it to bytecode.
		if isSyscall(f) {
			return
		}
		c.setLabel(f.label)
	} else {
		f = c.newFunc(decl)
	}

	c.scope = f
	ast.Inspect(decl, c.scope.analyzeVoidCalls) // @OPTIMIZE

	// All globals copied into the scope of the function need to be added
	// to the stack size of the function.
	emitInt(c.prog.BinWriter, f.stackSize()+countGlobals(file))
	emitOpcode(c.prog.BinWriter, opcode.NEWARRAY)
	emitOpcode(c.prog.BinWriter, opcode.TOALTSTACK)

	// We need to handle methods, which in Go, is just syntactic sugar.
	// The method receiver will be passed in as first argument.
	// We check if this declaration has a receiver and load it into scope.
	//
	// FIXME: For now we will hard cast this to a struct. We can later fine tune this
	// to support other types.
	if decl.Recv != nil {
		for _, arg := range decl.Recv.List {
			ident := arg.Names[0]
			// Currently only method receives for struct types is supported.
			_, ok := c.typeInfo.Defs[ident].Type().Underlying().(*types.Struct)
			if !ok {
				c.prog.Err = fmt.Errorf("method receives for non-struct types is not yet supported")
				return
			}
			l := c.scope.newLocal(ident.Name)
			c.emitStoreLocal(l)
		}
	}

	// Load the arguments in scope.
	for _, arg := range decl.Type.Params.List {
		name := arg.Names[0].Name // for now.
		l := c.scope.newLocal(name)
		c.emitStoreLocal(l)
	}
	// Load in all the global variables in to the scope of the function.
	// This is not necessary for syscalls.
	if !isSyscall(f) {
		c.convertGlobals(file)
	}

	ast.Walk(c, decl.Body)

	// If this function returns the void (no return stmt) we will cleanup its junk on the stack.
	if !hasReturnStmt(decl) {
		emitOpcode(c.prog.BinWriter, opcode.FROMALTSTACK)
		emitOpcode(c.prog.BinWriter, opcode.DROP)
		emitOpcode(c.prog.BinWriter, opcode.RET)
	}
}

func (c *codegen) Visit(node ast.Node) ast.Visitor {
	if c.prog.Err != nil {
		return nil
	}
	switch n := node.(type) {

	// General declarations.
	// var (
	//     x = 2
	// )
	case *ast.GenDecl:
		for _, spec := range n.Specs {
			switch t := spec.(type) {
			case *ast.ValueSpec:
				for i, val := range t.Values {
					ast.Walk(c, val)
					l := c.scope.newLocal(t.Names[i].Name)
					c.emitStoreLocal(l)
				}
			}
		}
		return nil

	case *ast.AssignStmt:
		multiRet := len(n.Rhs) != len(n.Lhs)

		for i := 0; i < len(n.Lhs); i++ {
			switch t := n.Lhs[i].(type) {
			case *ast.Ident:
				switch n.Tok {
				case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN:
					c.emitLoadLocal(t.Name)
					ast.Walk(c, n.Rhs[0]) // can only add assign to 1 expr on the RHS
					c.convertToken(n.Tok)
					l := c.scope.loadLocal(t.Name)
					c.emitStoreLocal(l)
				default:
					if i == 0 || !multiRet {
						ast.Walk(c, n.Rhs[i])
					}

					l := c.scope.loadLocal(t.Name)
					c.emitStoreLocal(l)
				}

			case *ast.SelectorExpr:
				switch expr := t.X.(type) {
				case *ast.Ident:
					ast.Walk(c, n.Rhs[i])
					typ := c.typeInfo.ObjectOf(expr).Type().Underlying()
					if strct, ok := typ.(*types.Struct); ok {
						c.emitLoadLocal(expr.Name)            // load the struct
						i := indexOfStruct(strct, t.Sel.Name) // get the index of the field
						c.emitStoreStructField(i)             // store the field
					}
				default:
					c.prog.Err = fmt.Errorf("nested selector assigns not supported yet")
					return nil
				}

			// Assignments to index expressions.
			// slice[0] = 10
			case *ast.IndexExpr:
				ast.Walk(c, n.Rhs[i])
				name := t.X.(*ast.Ident).Name
				c.emitLoadLocal(name)
				switch ind := t.Index.(type) {
				case *ast.BasicLit:
					indexStr := ind.Value
					index, err := strconv.Atoi(indexStr)
					if err != nil {
						c.prog.Err = fmt.Errorf("failed to convert slice index to integer")
						return nil
					}
					c.emitStoreStructField(index)
				case *ast.Ident:
					c.emitLoadLocal(ind.Name)
					emitOpcode(c.prog.BinWriter, opcode.ROT)
					emitOpcode(c.prog.BinWriter, opcode.SETITEM)
				default:
					c.prog.Err = fmt.Errorf("unsupported index expression")
					return nil
				}
			}
		}
		return nil

	case *ast.ReturnStmt:
		l := c.newLabel()
		c.setLabel(l)

		// first result should be on top of the stack
		for i := len(n.Results) - 1; i >= 0; i-- {
			ast.Walk(c, n.Results[i])
		}

		emitOpcode(c.prog.BinWriter, opcode.FROMALTSTACK)
		emitOpcode(c.prog.BinWriter, opcode.DROP) // Cleanup the stack.
		emitOpcode(c.prog.BinWriter, opcode.RET)
		return nil

	case *ast.IfStmt:
		lIf := c.newLabel()
		lElse := c.newLabel()
		lElseEnd := c.newLabel()

		if n.Cond != nil {
			ast.Walk(c, n.Cond)
			emitJmp(c.prog.BinWriter, opcode.JMPIFNOT, int16(lElse))
		}

		c.setLabel(lIf)
		ast.Walk(c, n.Body)
		if n.Else != nil {
			emitJmp(c.prog.BinWriter, opcode.JMP, int16(lElseEnd))
		}

		c.setLabel(lElse)
		if n.Else != nil {
			ast.Walk(c, n.Else)
		}
		c.setLabel(lElseEnd)
		return nil

	case *ast.SwitchStmt:
		// fallthrough is not supported
		ast.Walk(c, n.Tag)

		eqOpcode := c.getEqualityOpcode(n.Tag)
		switchEnd := c.newLabel()

		for i := range n.Body.List {
			lEnd := c.newLabel()
			lStart := c.newLabel()
			cc := n.Body.List[i].(*ast.CaseClause)

			if l := len(cc.List); l != 0 { // if not `default`
				for j := range cc.List {
					emitOpcode(c.prog.BinWriter, opcode.DUP)
					ast.Walk(c, cc.List[j])
					emitOpcode(c.prog.BinWriter, eqOpcode)
					if j == l-1 {
						emitJmp(c.prog.BinWriter, opcode.JMPIFNOT, int16(lEnd))
					} else {
						emitJmp(c.prog.BinWriter, opcode.JMPIF, int16(lStart))
					}
				}
			}

			c.setLabel(lStart)
			for _, stmt := range cc.Body {
				ast.Walk(c, stmt)
			}
			emitJmp(c.prog.BinWriter, opcode.JMP, int16(switchEnd))
			c.setLabel(lEnd)
		}

		c.setLabel(switchEnd)
		emitOpcode(c.prog.BinWriter, opcode.DROP)

		return nil

	case *ast.BasicLit:
		c.emitLoadConst(c.typeInfo.Types[n])
		return nil

	case *ast.Ident:
		if isIdentBool(n) {
			value, err := makeBoolFromIdent(n, c.typeInfo)
			if err != nil {
				c.prog.Err = err
				return nil
			}
			c.emitLoadConst(value)
		} else if tv := c.typeInfo.Types[n]; tv.Value != nil {
			c.emitLoadConst(tv)
		} else {
			c.emitLoadLocal(n.Name)
		}
		return nil

	case *ast.CompositeLit:
		var typ types.Type

		switch t := n.Type.(type) {
		case *ast.Ident:
			typ = c.typeInfo.ObjectOf(t).Type().Underlying()
		case *ast.SelectorExpr:
			typ = c.typeInfo.ObjectOf(t.Sel).Type().Underlying()
		case *ast.MapType:
			typ = c.typeInfo.TypeOf(t)
		default:
			ln := len(n.Elts)
			// ByteArrays needs a different approach than normal arrays.
			if isByteArray(n, c.typeInfo) {
				c.convertByteArray(n)
				return nil
			}
			for i := ln - 1; i >= 0; i-- {
				ast.Walk(c, n.Elts[i])
			}
			emitInt(c.prog.BinWriter, int64(ln))
			emitOpcode(c.prog.BinWriter, opcode.PACK)
			return nil
		}

		switch typ.(type) {
		case *types.Struct:
			c.convertStruct(n)
		case *types.Map:
			c.convertMap(n)
		}

		return nil

	case *ast.BinaryExpr:
		switch n.Op {
		case token.LAND:
			ast.Walk(c, n.X)
			emitJmp(c.prog.BinWriter, opcode.JMPIFNOT, int16(len(c.l)-1))
			ast.Walk(c, n.Y)
			return nil

		case token.LOR:
			ast.Walk(c, n.X)
			emitJmp(c.prog.BinWriter, opcode.JMPIF, int16(len(c.l)-3))
			ast.Walk(c, n.Y)
			return nil

		default:
			// The AST package will try to resolve all basic literals for us.
			// If the typeinfo.Value is not nil we know that the expr is resolved
			// and needs no further action. e.g. x := 2 + 2 + 2 will be resolved to 6.
			// NOTE: Constants will also be automatically resolved be the AST parser.
			// example:
			// const x = 10
			// x + 2 will results into 12
			tinfo := c.typeInfo.Types[n]
			if tinfo.Value != nil {
				c.emitLoadConst(tinfo)
				return nil
			}

			ast.Walk(c, n.X)
			ast.Walk(c, n.Y)

			switch {
			case n.Op == token.ADD:
				// VM has separate opcodes for number and string concatenation
				if isStringType(tinfo.Type) {
					emitOpcode(c.prog.BinWriter, opcode.CAT)
				} else {
					emitOpcode(c.prog.BinWriter, opcode.ADD)
				}
			case n.Op == token.EQL:
				// VM has separate opcodes for number and string equality
				op := c.getEqualityOpcode(n.X)
				emitOpcode(c.prog.BinWriter, op)
			case n.Op == token.NEQ:
				// VM has separate opcodes for number and string equality
				if isStringType(c.typeInfo.Types[n.X].Type) {
					emitOpcode(c.prog.BinWriter, opcode.EQUAL)
					emitOpcode(c.prog.BinWriter, opcode.NOT)
				} else {
					emitOpcode(c.prog.BinWriter, opcode.NUMNOTEQUAL)
				}
			default:
				c.convertToken(n.Op)
			}
			return nil
		}

	case *ast.CallExpr:
		var (
			f         *funcScope
			ok        bool
			numArgs   = len(n.Args)
			isBuiltin = isBuiltin(n.Fun)
		)

		switch fun := n.Fun.(type) {
		case *ast.Ident:
			f, ok = c.funcs[fun.Name]
			if !ok && !isBuiltin {
				c.prog.Err = fmt.Errorf("could not resolve function %s", fun.Name)
				return nil
			}
		case *ast.SelectorExpr:
			// If this is a method call we need to walk the AST to load the struct locally.
			// Otherwise this is a function call from a imported package and we can call it
			// directly.
			if c.typeInfo.Selections[fun] != nil {
				ast.Walk(c, fun.X)
				// Dont forget to add 1 extra argument when its a method.
				numArgs++
			}

			f, ok = c.funcs[fun.Sel.Name]
			// @FIXME this could cause runtime errors.
			f.selector = fun.X.(*ast.Ident)
			if !ok {
				c.prog.Err = fmt.Errorf("could not resolve function %s", fun.Sel.Name)
				return nil
			}
		case *ast.ArrayType:
			// For now we will assume that there are only byte slice conversions.
			// E.g. []byte("foobar") or []byte(scriptHash).
			ast.Walk(c, n.Args[0])
			return nil
		}

		args := transformArgs(n.Fun, n.Args)

		// Handle the arguments
		for _, arg := range args {
			ast.Walk(c, arg)
		}
		// Do not swap for builtin functions.
		if !isBuiltin {
			if numArgs == 2 {
				emitOpcode(c.prog.BinWriter, opcode.SWAP)
			} else if numArgs == 3 {
				emitInt(c.prog.BinWriter, 2)
				emitOpcode(c.prog.BinWriter, opcode.XSWAP)
			} else {
				for i := 1; i < numArgs; i++ {
					emitInt(c.prog.BinWriter, int64(i))
					emitOpcode(c.prog.BinWriter, opcode.ROLL)
				}
			}
		}

		// Check builtin first to avoid nil pointer on funcScope!
		switch {
		case isBuiltin:
			// Use the ident to check, builtins are not in func scopes.
			// We can be sure builtins are of type *ast.Ident.
			c.convertBuiltin(n)
		case isSyscall(f):
			c.convertSyscall(f.selector.Name, f.name)
		default:
			emitCall(c.prog.BinWriter, opcode.CALL, int16(f.label))
		}

		return nil

	case *ast.SelectorExpr:
		switch t := n.X.(type) {
		case *ast.Ident:
			typ := c.typeInfo.ObjectOf(t).Type().Underlying()
			if strct, ok := typ.(*types.Struct); ok {
				c.emitLoadLocal(t.Name) // load the struct
				i := indexOfStruct(strct, n.Sel.Name)
				c.emitLoadField(i) // load the field
			}
		default:
			c.prog.Err = fmt.Errorf("nested selectors not supported yet")
			return nil
		}
		return nil

	case *ast.UnaryExpr:
		ast.Walk(c, n.X)
		// From https://golang.org/ref/spec#Operators
		// there can be only following unary operators
		// "+" | "-" | "!" | "^" | "*" | "&" | "<-" .
		// of which last three are not used in SC
		switch n.Op {
		case token.ADD:
			// +10 == 10, no need to do anything in this case
		case token.SUB:
			emitOpcode(c.prog.BinWriter, opcode.NEGATE)
		case token.NOT:
			emitOpcode(c.prog.BinWriter, opcode.NOT)
		case token.XOR:
			emitOpcode(c.prog.BinWriter, opcode.INVERT)
		default:
			c.prog.Err = fmt.Errorf("invalid unary operator: %s", n.Op)
			return nil
		}
		return nil

	case *ast.IncDecStmt:
		ast.Walk(c, n.X)
		c.convertToken(n.Tok)

		// For now only identifiers are supported for (post) for stmts.
		// for i := 0; i < 10; i++ {}
		// Where the post stmt is ( i++ )
		if ident, ok := n.X.(*ast.Ident); ok {
			pos := c.scope.loadLocal(ident.Name)
			c.emitStoreLocal(pos)
		}
		return nil

	case *ast.IndexExpr:
		// Walk the expression, this could be either an Ident or SelectorExpr.
		// This will load local whatever X is.
		ast.Walk(c, n.X)

		switch n.Index.(type) {
		case *ast.BasicLit:
			t := c.typeInfo.Types[n.Index]
			switch typ := t.Type.Underlying().(type) {
			case *types.Basic:
				c.convertBasicType(t, typ)
			default:
				c.prog.Err = fmt.Errorf("compiler can't use following type as an index: %T", typ)
				return nil
			}
		default:
			ast.Walk(c, n.Index)
		}

		emitOpcode(c.prog.BinWriter, opcode.PICKITEM) // just pickitem here

		return nil

	case *ast.ForStmt:
		var (
			fstart = c.newLabel()
			fend   = c.newLabel()
		)

		// Walk the initializer and condition.
		if n.Init != nil {
			ast.Walk(c, n.Init)
		}

		// Set label and walk the condition.
		c.setLabel(fstart)
		ast.Walk(c, n.Cond)

		// Jump if the condition is false
		emitJmp(c.prog.BinWriter, opcode.JMPIFNOT, int16(fend))

		// Walk body followed by the iterator (post stmt).
		ast.Walk(c, n.Body)
		if n.Post != nil {
			ast.Walk(c, n.Post)
		}

		// Jump back to condition.
		emitJmp(c.prog.BinWriter, opcode.JMP, int16(fstart))
		c.setLabel(fend)

		return nil

	// We dont really care about assertions for the core logic.
	// The only thing we need is to please the compiler type checking.
	// For this to work properly, we only need to walk the expression
	// not the assertion type.
	case *ast.TypeAssertExpr:
		ast.Walk(c, n.X)
		return nil
	}
	return c
}

func (c *codegen) getEqualityOpcode(expr ast.Expr) opcode.Opcode {
	t, ok := c.typeInfo.Types[expr].Type.Underlying().(*types.Basic)
	if ok && t.Info()&types.IsNumeric != 0 {
		return opcode.NUMEQUAL
	}

	return opcode.EQUAL
}

// getByteArray returns byte array value from constant expr.
// Only literals are supported.
func (c *codegen) getByteArray(expr ast.Expr) []byte {
	switch t := expr.(type) {
	case *ast.CompositeLit:
		if !isByteArray(t, c.typeInfo) {
			return nil
		}
		buf := make([]byte, len(t.Elts))
		for i := 0; i < len(t.Elts); i++ {
			t := c.typeInfo.Types[t.Elts[i]]
			val, _ := constant.Int64Val(t.Value)
			buf[i] = byte(val)
		}
		return buf
	case *ast.CallExpr:
		if tv := c.typeInfo.Types[t.Args[0]]; tv.Value != nil {
			val := constant.StringVal(tv.Value)
			return []byte(val)
		}

		return nil
	default:
		return nil
	}
}

func (c *codegen) convertSyscall(api, name string) {
	api, ok := syscalls[api][name]
	if !ok {
		c.prog.Err = fmt.Errorf("unknown VM syscall api: %s", name)
		return
	}
	emitSyscall(c.prog.BinWriter, api)

	// This NOP instruction is basically not needed, but if we do, we have a
	// one to one matching avm file with neo-python which is very nice for debugging.
	emitOpcode(c.prog.BinWriter, opcode.NOP)
}

func (c *codegen) convertBuiltin(expr *ast.CallExpr) {
	var name string
	switch t := expr.Fun.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.SelectorExpr:
		name = t.Sel.Name
	}

	switch name {
	case "len":
		arg := expr.Args[0]
		typ := c.typeInfo.Types[arg].Type
		if isStringType(typ) {
			emitOpcode(c.prog.BinWriter, opcode.SIZE)
		} else {
			emitOpcode(c.prog.BinWriter, opcode.ARRAYSIZE)
		}
	case "append":
		arg := expr.Args[0]
		typ := c.typeInfo.Types[arg].Type
		if isByteArrayType(typ) {
			emitOpcode(c.prog.BinWriter, opcode.CAT)
		} else {
			emitOpcode(c.prog.BinWriter, opcode.SWAP)
			emitOpcode(c.prog.BinWriter, opcode.DUP)
			emitOpcode(c.prog.BinWriter, opcode.PUSH2)
			emitOpcode(c.prog.BinWriter, opcode.XSWAP)
			emitOpcode(c.prog.BinWriter, opcode.APPEND)
		}
	case "panic":
		arg := expr.Args[0]
		if isExprNil(arg) {
			emitOpcode(c.prog.BinWriter, opcode.DROP)
			emitOpcode(c.prog.BinWriter, opcode.THROW)
		} else if isStringType(c.typeInfo.Types[arg].Type) {
			ast.Walk(c, arg)
			emitSyscall(c.prog.BinWriter, "Neo.Runtime.Log")
			emitOpcode(c.prog.BinWriter, opcode.THROW)
		} else {
			c.prog.Err = errors.New("panic should have string or nil argument")
		}
	case "SHA256":
		emitOpcode(c.prog.BinWriter, opcode.SHA256)
	case "SHA1":
		emitOpcode(c.prog.BinWriter, opcode.SHA1)
	case "Hash256":
		emitOpcode(c.prog.BinWriter, opcode.HASH256)
	case "Hash160":
		emitOpcode(c.prog.BinWriter, opcode.HASH160)
	case "VerifySignature":
		emitOpcode(c.prog.BinWriter, opcode.VERIFY)
	case "AppCall":
		emitOpcode(c.prog.BinWriter, opcode.APPCALL)
		buf := c.getByteArray(expr.Args[0])
		if len(buf) != 20 {
			c.prog.Err = errors.New("invalid script hash")
		}

		c.prog.WriteBytes(buf)
	case "Equals":
		emitOpcode(c.prog.BinWriter, opcode.EQUAL)
	case "FromAddress":
		// We can be sure that this is a ast.BasicLit just containing a simple
		// address string. Note that the string returned from calling Value will
		// contain double quotes that need to be stripped.
		addressStr := expr.Args[0].(*ast.BasicLit).Value
		addressStr = strings.Replace(addressStr, "\"", "", 2)
		uint160, err := address.StringToUint160(addressStr)
		if err != nil {
			c.prog.Err = err
			return
		}
		bytes := uint160.BytesBE()
		emitBytes(c.prog.BinWriter, bytes)
	}
}

// transformArgs returns a list of function arguments
// which should be put on stack.
// There are special cases for builtins:
// 1. When using AppCall, script hash is a part of the instruction so
//    it should be emitted after APPCALL.
// 2. With FromAddress, parameter conversion is happening at compile-time
//    so there is no need to push parameters on stack and perform an actual call
// 3. With panic, generated code depends on if argument was nil or a string so
//    it should be handled accordingly.
func transformArgs(fun ast.Expr, args []ast.Expr) []ast.Expr {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		if f.Sel.Name == "AppCall" || f.Sel.Name == "FromAddress" {
			return args[1:]
		}
	case *ast.Ident:
		if f.Name == "panic" {
			return args[1:]
		}
	}

	return args
}

func (c *codegen) convertByteArray(lit *ast.CompositeLit) {
	buf := make([]byte, len(lit.Elts))
	for i := 0; i < len(lit.Elts); i++ {
		t := c.typeInfo.Types[lit.Elts[i]]
		val, _ := constant.Int64Val(t.Value)
		buf[i] = byte(val)
	}
	emitBytes(c.prog.BinWriter, buf)
}

func (c *codegen) convertMap(lit *ast.CompositeLit) {
	emitOpcode(c.prog.BinWriter, opcode.NEWMAP)
	for i := range lit.Elts {
		elem := lit.Elts[i].(*ast.KeyValueExpr)
		emitOpcode(c.prog.BinWriter, opcode.DUP)
		ast.Walk(c, elem.Key)
		ast.Walk(c, elem.Value)
		emitOpcode(c.prog.BinWriter, opcode.SETITEM)
	}
}

func (c *codegen) convertStruct(lit *ast.CompositeLit) {
	// Create a new structScope to initialize and store
	// the positions of its variables.
	strct, ok := c.typeInfo.TypeOf(lit).Underlying().(*types.Struct)
	if !ok {
		c.prog.Err = fmt.Errorf("the given literal is not of type struct: %v", lit)
		return
	}

	emitOpcode(c.prog.BinWriter, opcode.NOP)
	emitInt(c.prog.BinWriter, int64(strct.NumFields()))
	emitOpcode(c.prog.BinWriter, opcode.NEWSTRUCT)
	emitOpcode(c.prog.BinWriter, opcode.TOALTSTACK)

	// We need to locally store all the fields, even if they are not initialized.
	// We will initialize all fields to their "zero" value.
	for i := 0; i < strct.NumFields(); i++ {
		sField := strct.Field(i)
		fieldAdded := false

		// Fields initialized by the program.
		for _, field := range lit.Elts {
			f := field.(*ast.KeyValueExpr)
			fieldName := f.Key.(*ast.Ident).Name

			if sField.Name() == fieldName {
				ast.Walk(c, f.Value)
				pos := indexOfStruct(strct, fieldName)
				c.emitStoreLocal(pos)
				fieldAdded = true
				break
			}
		}
		if fieldAdded {
			continue
		}

		typeAndVal, err := typeAndValueForField(sField)
		if err != nil {
			c.prog.Err = err
			return
		}
		c.emitLoadConst(typeAndVal)
		c.emitStoreLocal(i)
	}
	emitOpcode(c.prog.BinWriter, opcode.FROMALTSTACK)
}

func (c *codegen) convertToken(tok token.Token) {
	switch tok {
	case token.ADD_ASSIGN:
		emitOpcode(c.prog.BinWriter, opcode.ADD)
	case token.SUB_ASSIGN:
		emitOpcode(c.prog.BinWriter, opcode.SUB)
	case token.MUL_ASSIGN:
		emitOpcode(c.prog.BinWriter, opcode.MUL)
	case token.QUO_ASSIGN:
		emitOpcode(c.prog.BinWriter, opcode.DIV)
	case token.REM_ASSIGN:
		emitOpcode(c.prog.BinWriter, opcode.MOD)
	case token.ADD:
		emitOpcode(c.prog.BinWriter, opcode.ADD)
	case token.SUB:
		emitOpcode(c.prog.BinWriter, opcode.SUB)
	case token.MUL:
		emitOpcode(c.prog.BinWriter, opcode.MUL)
	case token.QUO:
		emitOpcode(c.prog.BinWriter, opcode.DIV)
	case token.REM:
		emitOpcode(c.prog.BinWriter, opcode.MOD)
	case token.LSS:
		emitOpcode(c.prog.BinWriter, opcode.LT)
	case token.LEQ:
		emitOpcode(c.prog.BinWriter, opcode.LTE)
	case token.GTR:
		emitOpcode(c.prog.BinWriter, opcode.GT)
	case token.GEQ:
		emitOpcode(c.prog.BinWriter, opcode.GTE)
	case token.EQL:
		emitOpcode(c.prog.BinWriter, opcode.NUMEQUAL)
	case token.NEQ:
		emitOpcode(c.prog.BinWriter, opcode.NUMNOTEQUAL)
	case token.DEC:
		emitOpcode(c.prog.BinWriter, opcode.DEC)
	case token.INC:
		emitOpcode(c.prog.BinWriter, opcode.INC)
	case token.NOT:
		emitOpcode(c.prog.BinWriter, opcode.NOT)
	case token.AND:
		emitOpcode(c.prog.BinWriter, opcode.AND)
	case token.OR:
		emitOpcode(c.prog.BinWriter, opcode.OR)
	case token.SHL:
		emitOpcode(c.prog.BinWriter, opcode.SHL)
	case token.SHR:
		emitOpcode(c.prog.BinWriter, opcode.SHR)
	case token.XOR:
		emitOpcode(c.prog.BinWriter, opcode.XOR)
	default:
		c.prog.Err = fmt.Errorf("compiler could not convert token: %s", tok)
		return
	}
}

func (c *codegen) newFunc(decl *ast.FuncDecl) *funcScope {
	f := newFuncScope(decl, c.newLabel())
	c.funcs[f.name] = f
	return f
}

// CodeGen compiles the program to bytecode.
func CodeGen(info *buildInfo) ([]byte, error) {
	pkg := info.program.Package(info.initialPackage)
	c := &codegen{
		buildInfo: info,
		prog:      io.NewBufBinWriter(),
		l:         []int{},
		funcs:     map[string]*funcScope{},
		typeInfo:  &pkg.Info,
	}

	// Resolve the entrypoint of the program.
	main, mainFile := resolveEntryPoint(mainIdent, pkg)
	if main == nil {
		c.prog.Err = fmt.Errorf("could not find func main. Did you forget to declare it? ")
		return []byte{}, c.prog.Err
	}

	funUsage := analyzeFuncUsage(info.program.AllPackages)

	// Bring all imported functions into scope.
	for _, pkg := range info.program.AllPackages {
		for _, f := range pkg.Files {
			c.resolveFuncDecls(f)
		}
	}

	// convert the entry point first.
	c.convertFuncDecl(mainFile, main)

	// sort map keys to generate code deterministically.
	keys := make([]*types.Package, 0, len(info.program.AllPackages))
	for p := range info.program.AllPackages {
		keys = append(keys, p)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Path() < keys[j].Path() })

	// Generate the code for the program.
	for _, k := range keys {
		pkg := info.program.AllPackages[k]
		c.typeInfo = &pkg.Info

		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				switch n := decl.(type) {
				case *ast.FuncDecl:
					// Don't convert the function if it's not used. This will save a lot
					// of bytecode space.
					if n.Name.Name != mainIdent && funUsage.funcUsed(n.Name.Name) {
						c.convertFuncDecl(f, n)
					}
				}
			}
		}
	}

	if c.prog.Err != nil {
		return nil, c.prog.Err
	}
	buf := c.prog.Bytes()
	c.writeJumps(buf)
	return buf, nil
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

func (c *codegen) writeJumps(b []byte) {
	for i, op := range b {
		j := i + 1
		switch opcode.Opcode(op) {
		case opcode.JMP, opcode.JMPIFNOT, opcode.JMPIF, opcode.CALL:
			index := int16(binary.LittleEndian.Uint16(b[j : j+2]))
			if int(index) > len(c.l) || int(index) < 0 {
				continue
			}
			offset := uint16(c.l[index] - i)
			binary.LittleEndian.PutUint16(b[j:j+2], offset)
		}
	}
}
