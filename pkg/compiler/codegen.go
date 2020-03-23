package compiler

import (
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"golang.org/x/tools/go/loader"
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

	// A mapping from label's names to their ids.
	labels map[labelWithType]uint16
	// A list of nested label names together with evaluation stack depth.
	labelList []labelWithStackSize

	// A label for the for-loop being currently visited.
	currentFor string
	// A label for the switch statement being visited.
	currentSwitch string
	// A label to be used in the next statement.
	nextLabel string

	// sequencePoints is mapping from method name to a slice
	// containing info about mapping from opcode's offset
	// to a text span in the source file.
	sequencePoints map[string][]DebugSeqPoint

	// Label table for recording jump destinations.
	l []int
}

type labelOffsetType byte

const (
	labelStart labelOffsetType = iota // labelStart is a default label type
	labelEnd                          // labelEnd is a type for labels that are targets for break
	labelPost                         // labelPost is a type for labels that are targets for continue
)

type labelWithType struct {
	name string
	typ  labelOffsetType
}

type labelWithStackSize struct {
	name string
	sz   int
}

// newLabel creates a new label to jump to
func (c *codegen) newLabel() (l uint16) {
	li := len(c.l)
	if li > math.MaxUint16 {
		c.prog.Err = errors.New("label number is too big")
		return
	}
	l = uint16(li)
	c.l = append(c.l, -1)
	return
}

// newNamedLabel creates a new label with a specified name.
func (c *codegen) newNamedLabel(typ labelOffsetType, name string) (l uint16) {
	l = c.newLabel()
	lt := labelWithType{name: name, typ: typ}
	c.labels[lt] = l
	return
}

func (c *codegen) setLabel(l uint16) {
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
		emit.Int(c.prog.BinWriter, val)
	case types.String, types.UntypedString:
		val := constant.StringVal(t.Value)
		emit.String(c.prog.BinWriter, val)
	case types.Bool, types.UntypedBool:
		val := constant.BoolVal(t.Value)
		emit.Bool(c.prog.BinWriter, val)
	case types.Byte:
		val, _ := constant.Int64Val(t.Value)
		b := byte(val)
		emit.Bytes(c.prog.BinWriter, []byte{b})
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
	emit.Opcode(c.prog.BinWriter, opcode.DUPFROMALTSTACK)
	emit.Int(c.prog.BinWriter, int64(pos))
	emit.Opcode(c.prog.BinWriter, opcode.PICKITEM)
}

func (c *codegen) emitStoreLocal(pos int) {
	emit.Opcode(c.prog.BinWriter, opcode.DUPFROMALTSTACK)

	if pos < 0 {
		c.prog.Err = fmt.Errorf("invalid position to store local: %d", pos)
		return
	}

	emit.Int(c.prog.BinWriter, int64(pos))
	emit.Opcode(c.prog.BinWriter, opcode.ROT)
	emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
}

func (c *codegen) emitLoadField(i int) {
	emit.Int(c.prog.BinWriter, int64(i))
	emit.Opcode(c.prog.BinWriter, opcode.PICKITEM)
}

func (c *codegen) emitStoreStructField(i int) {
	emit.Int(c.prog.BinWriter, int64(i))
	emit.Opcode(c.prog.BinWriter, opcode.ROT)
	emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
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
			// constants are loaded directly so there is no need
			// to store them as a local variables
			if n.Tok != token.CONST {
				ast.Walk(c, n)
			}
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

	f.rng.Start = uint16(c.prog.Len())
	c.scope = f
	ast.Inspect(decl, c.scope.analyzeVoidCalls) // @OPTIMIZE

	// All globals copied into the scope of the function need to be added
	// to the stack size of the function.
	emit.Int(c.prog.BinWriter, f.stackSize()+countGlobals(file))
	emit.Opcode(c.prog.BinWriter, opcode.NEWARRAY)
	emit.Opcode(c.prog.BinWriter, opcode.TOALTSTACK)

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
		c.saveSequencePoint(decl.Body)
		emit.Opcode(c.prog.BinWriter, opcode.FROMALTSTACK)
		emit.Opcode(c.prog.BinWriter, opcode.DROP)
		emit.Opcode(c.prog.BinWriter, opcode.RET)
	}

	f.rng.End = uint16(c.prog.Len() - 1)
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
				for _, id := range t.Names {
					c.scope.newLocal(id.Name)
					c.registerDebugVariable(id.Name, t.Type)
				}
				if len(t.Values) != 0 {
					for i, val := range t.Values {
						ast.Walk(c, val)
						l := c.scope.loadLocal(t.Names[i].Name)
						c.emitStoreLocal(l)
					}
				} else if c.isCompoundArrayType(t.Type) {
					emit.Opcode(c.prog.BinWriter, opcode.PUSH0)
					emit.Opcode(c.prog.BinWriter, opcode.NEWARRAY)
					l := c.scope.loadLocal(t.Names[0].Name)
					c.emitStoreLocal(l)
				} else if n, ok := c.isStructType(t.Type); ok {
					emit.Int(c.prog.BinWriter, int64(n))
					emit.Opcode(c.prog.BinWriter, opcode.NEWSTRUCT)
					l := c.scope.loadLocal(t.Names[0].Name)
					c.emitStoreLocal(l)
				}
			}
		}
		return nil

	case *ast.AssignStmt:
		multiRet := len(n.Rhs) != len(n.Lhs)
		c.saveSequencePoint(n)
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
				case token.DEFINE:
					if !multiRet {
						c.registerDebugVariable(t.Name, n.Rhs[i])
					}
					fallthrough
				default:
					if i == 0 || !multiRet {
						ast.Walk(c, n.Rhs[i])
					}

					if t.Name == "_" {
						emit.Opcode(c.prog.BinWriter, opcode.DROP)
					} else {
						l := c.scope.loadLocal(t.Name)
						c.emitStoreLocal(l)
					}
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
					emit.Opcode(c.prog.BinWriter, opcode.ROT)
					emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
				default:
					c.prog.Err = fmt.Errorf("unsupported index expression")
					return nil
				}
			}
		}
		return nil

	case *ast.SliceExpr:
		name := n.X.(*ast.Ident).Name
		c.emitLoadLocal(name)

		if n.Low != nil {
			ast.Walk(c, n.Low)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.PUSH0)
		}

		if n.High != nil {
			ast.Walk(c, n.High)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.OVER)
			emit.Opcode(c.prog.BinWriter, opcode.ARRAYSIZE)
		}

		emit.Opcode(c.prog.BinWriter, opcode.OVER)
		emit.Opcode(c.prog.BinWriter, opcode.SUB)
		emit.Opcode(c.prog.BinWriter, opcode.SUBSTR)

		return nil

	case *ast.ReturnStmt:
		l := c.newLabel()
		c.setLabel(l)

		cnt := 0
		for i := range c.labelList {
			cnt += c.labelList[i].sz
		}
		c.dropItems(cnt)

		// first result should be on top of the stack
		for i := len(n.Results) - 1; i >= 0; i-- {
			ast.Walk(c, n.Results[i])
		}

		c.saveSequencePoint(n)
		emit.Opcode(c.prog.BinWriter, opcode.FROMALTSTACK)
		emit.Opcode(c.prog.BinWriter, opcode.DROP) // Cleanup the stack.
		emit.Opcode(c.prog.BinWriter, opcode.RET)
		return nil

	case *ast.IfStmt:
		lIf := c.newLabel()
		lElse := c.newLabel()
		lElseEnd := c.newLabel()

		if n.Cond != nil {
			ast.Walk(c, n.Cond)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOT, lElse)
		}

		c.setLabel(lIf)
		ast.Walk(c, n.Body)
		if n.Else != nil {
			emit.Jmp(c.prog.BinWriter, opcode.JMP, lElseEnd)
		}

		c.setLabel(lElse)
		if n.Else != nil {
			ast.Walk(c, n.Else)
		}
		c.setLabel(lElseEnd)
		return nil

	case *ast.SwitchStmt:
		ast.Walk(c, n.Tag)

		eqOpcode := c.getEqualityOpcode(n.Tag)
		switchEnd, label := c.generateLabel(labelEnd)

		lastSwitch := c.currentSwitch
		c.currentSwitch = label
		c.pushStackLabel(label, 1)

		startLabels := make([]uint16, len(n.Body.List))
		for i := range startLabels {
			startLabels[i] = c.newLabel()
		}
		for i := range n.Body.List {
			lEnd := c.newLabel()
			lStart := startLabels[i]
			cc := n.Body.List[i].(*ast.CaseClause)

			if l := len(cc.List); l != 0 { // if not `default`
				for j := range cc.List {
					emit.Opcode(c.prog.BinWriter, opcode.DUP)
					ast.Walk(c, cc.List[j])
					emit.Opcode(c.prog.BinWriter, eqOpcode)
					if j == l-1 {
						emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOT, lEnd)
					} else {
						emit.Jmp(c.prog.BinWriter, opcode.JMPIF, lStart)
					}
				}
			}

			c.setLabel(lStart)
			last := len(cc.Body) - 1
			for j, stmt := range cc.Body {
				if j == last && isFallthroughStmt(stmt) {
					emit.Jmp(c.prog.BinWriter, opcode.JMP, startLabels[i+1])
					break
				}
				ast.Walk(c, stmt)
			}
			emit.Jmp(c.prog.BinWriter, opcode.JMP, switchEnd)
			c.setLabel(lEnd)
		}

		c.setLabel(switchEnd)
		c.dropStackLabel()

		c.currentSwitch = lastSwitch

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
			emit.Int(c.prog.BinWriter, int64(ln))
			emit.Opcode(c.prog.BinWriter, opcode.PACK)
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
			next := c.newLabel()
			end := c.newLabel()
			ast.Walk(c, n.X)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIF, next)
			emit.Opcode(c.prog.BinWriter, opcode.PUSHF)
			emit.Jmp(c.prog.BinWriter, opcode.JMP, end)
			c.setLabel(next)
			ast.Walk(c, n.Y)
			c.setLabel(end)
			return nil

		case token.LOR:
			next := c.newLabel()
			end := c.newLabel()
			ast.Walk(c, n.X)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOT, next)
			emit.Opcode(c.prog.BinWriter, opcode.PUSHT)
			emit.Jmp(c.prog.BinWriter, opcode.JMP, end)
			c.setLabel(next)
			ast.Walk(c, n.Y)
			c.setLabel(end)
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
					emit.Opcode(c.prog.BinWriter, opcode.CAT)
				} else {
					emit.Opcode(c.prog.BinWriter, opcode.ADD)
				}
			case n.Op == token.EQL:
				// VM has separate opcodes for number and string equality
				op := c.getEqualityOpcode(n.X)
				emit.Opcode(c.prog.BinWriter, op)
			case n.Op == token.NEQ:
				// VM has separate opcodes for number and string equality
				if isStringType(c.typeInfo.Types[n.X].Type) {
					emit.Opcode(c.prog.BinWriter, opcode.EQUAL)
					emit.Opcode(c.prog.BinWriter, opcode.NOT)
				} else {
					emit.Opcode(c.prog.BinWriter, opcode.NUMNOTEQUAL)
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

		c.saveSequencePoint(n)

		args := transformArgs(n.Fun, n.Args)

		// Handle the arguments
		for _, arg := range args {
			ast.Walk(c, arg)
		}
		// Do not swap for builtin functions.
		if !isBuiltin {
			c.emitReverse(numArgs)
		}

		// Check builtin first to avoid nil pointer on funcScope!
		switch {
		case isBuiltin:
			// Use the ident to check, builtins are not in func scopes.
			// We can be sure builtins are of type *ast.Ident.
			c.convertBuiltin(n)
		case isSyscall(f):
			c.convertSyscall(n, f.selector.Name, f.name)
		default:
			emit.Call(c.prog.BinWriter, opcode.CALL, f.label)
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
			emit.Opcode(c.prog.BinWriter, opcode.NEGATE)
		case token.NOT:
			emit.Opcode(c.prog.BinWriter, opcode.NOT)
		case token.XOR:
			emit.Opcode(c.prog.BinWriter, opcode.INVERT)
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

		emit.Opcode(c.prog.BinWriter, opcode.PICKITEM) // just pickitem here

		return nil

	case *ast.BranchStmt:
		var label string
		if n.Label != nil {
			label = n.Label.Name
		} else if n.Tok == token.BREAK {
			label = c.currentSwitch
		} else if n.Tok == token.CONTINUE {
			label = c.currentFor
		}

		cnt := 0
		for i := len(c.labelList) - 1; i >= 0 && c.labelList[i].name != label; i-- {
			cnt += c.labelList[i].sz
		}
		c.dropItems(cnt)

		switch n.Tok {
		case token.BREAK:
			end := c.getLabelOffset(labelEnd, label)
			emit.Jmp(c.prog.BinWriter, opcode.JMP, end)
		case token.CONTINUE:
			post := c.getLabelOffset(labelPost, label)
			emit.Jmp(c.prog.BinWriter, opcode.JMP, post)
		}

		return nil

	case *ast.LabeledStmt:
		c.nextLabel = n.Label.Name

		ast.Walk(c, n.Stmt)

		return nil

	case *ast.ForStmt:
		fstart, label := c.generateLabel(labelStart)
		fend := c.newNamedLabel(labelEnd, label)
		fpost := c.newNamedLabel(labelPost, label)

		lastLabel := c.currentFor
		lastSwitch := c.currentSwitch
		c.currentFor = label
		c.currentSwitch = label

		// Walk the initializer and condition.
		if n.Init != nil {
			ast.Walk(c, n.Init)
		}

		// Set label and walk the condition.
		c.pushStackLabel(label, 0)
		c.setLabel(fstart)
		if n.Cond != nil {
			ast.Walk(c, n.Cond)

			// Jump if the condition is false
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOT, fend)
		}

		// Walk body followed by the iterator (post stmt).
		ast.Walk(c, n.Body)
		c.setLabel(fpost)
		if n.Post != nil {
			ast.Walk(c, n.Post)
		}

		// Jump back to condition.
		emit.Jmp(c.prog.BinWriter, opcode.JMP, fstart)
		c.setLabel(fend)
		c.dropStackLabel()

		c.currentFor = lastLabel
		c.currentSwitch = lastSwitch

		return nil

	case *ast.RangeStmt:
		// currently only simple for-range loops are supported
		// for i := range ...
		if n.Value != nil {
			c.prog.Err = errors.New("range loops with value variable are not supported")
			return nil
		}

		start, label := c.generateLabel(labelStart)
		end := c.newNamedLabel(labelEnd, label)
		post := c.newNamedLabel(labelPost, label)

		lastFor := c.currentFor
		lastSwitch := c.currentSwitch
		c.currentFor = label
		c.currentSwitch = label

		ast.Walk(c, n.X)

		emit.Opcode(c.prog.BinWriter, opcode.ARRAYSIZE)
		emit.Opcode(c.prog.BinWriter, opcode.PUSH0)

		c.pushStackLabel(label, 2)
		c.setLabel(start)

		emit.Opcode(c.prog.BinWriter, opcode.OVER)
		emit.Opcode(c.prog.BinWriter, opcode.OVER)
		emit.Opcode(c.prog.BinWriter, opcode.LTE) // finish if len <= i
		emit.Jmp(c.prog.BinWriter, opcode.JMPIF, end)

		if n.Key != nil {
			emit.Opcode(c.prog.BinWriter, opcode.DUP)

			pos := c.scope.loadLocal(n.Key.(*ast.Ident).Name)
			c.emitStoreLocal(pos)
		}

		ast.Walk(c, n.Body)

		c.setLabel(post)

		emit.Opcode(c.prog.BinWriter, opcode.INC)
		emit.Jmp(c.prog.BinWriter, opcode.JMP, start)

		c.setLabel(end)
		c.dropStackLabel()

		c.currentFor = lastFor
		c.currentSwitch = lastSwitch

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

func isFallthroughStmt(c ast.Node) bool {
	s, ok := c.(*ast.BranchStmt)
	return ok && s.Tok == token.FALLTHROUGH
}

func (c *codegen) pushStackLabel(name string, size int) {
	c.labelList = append(c.labelList, labelWithStackSize{
		name: name,
		sz:   size,
	})
}

func (c *codegen) dropStackLabel() {
	last := len(c.labelList) - 1
	c.dropItems(c.labelList[last].sz)
	c.labelList = c.labelList[:last]
}

func (c *codegen) dropItems(n int) {
	if n < 4 {
		for i := 0; i < n; i++ {
			emit.Opcode(c.prog.BinWriter, opcode.DROP)
		}
		return
	}

	emit.Int(c.prog.BinWriter, int64(n))
	emit.Opcode(c.prog.BinWriter, opcode.PACK)
	emit.Opcode(c.prog.BinWriter, opcode.DROP)
}

// emitReverse reverses top num items of the stack.
func (c *codegen) emitReverse(num int) {
	switch num {
	case 2:
		emit.Opcode(c.prog.BinWriter, opcode.SWAP)
	case 3:
		emit.Int(c.prog.BinWriter, 2)
		emit.Opcode(c.prog.BinWriter, opcode.XSWAP)
	default:
		for i := 1; i < num; i++ {
			emit.Int(c.prog.BinWriter, int64(i))
			emit.Opcode(c.prog.BinWriter, opcode.ROLL)
		}
	}
}

// generateLabel returns a new label.
func (c *codegen) generateLabel(typ labelOffsetType) (uint16, string) {
	name := c.nextLabel
	if name == "" {
		name = fmt.Sprintf("@%d", len(c.l))
	}

	c.nextLabel = ""
	return c.newNamedLabel(typ, name), name
}

func (c *codegen) getLabelOffset(typ labelOffsetType, name string) uint16 {
	return c.labels[labelWithType{name: name, typ: typ}]
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

func (c *codegen) convertSyscall(expr *ast.CallExpr, api, name string) {
	api, ok := syscalls[api][name]
	if !ok {
		c.prog.Err = fmt.Errorf("unknown VM syscall api: %s", name)
		return
	}
	switch name {
	case "Notify":
		numArgs := len(expr.Args)
		emit.Int(c.prog.BinWriter, int64(numArgs))
		emit.Opcode(c.prog.BinWriter, opcode.PACK)
	}
	emit.Syscall(c.prog.BinWriter, api)

	// This NOP instruction is basically not needed, but if we do, we have a
	// one to one matching avm file with neo-python which is very nice for debugging.
	emit.Opcode(c.prog.BinWriter, opcode.NOP)
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
			emit.Opcode(c.prog.BinWriter, opcode.SIZE)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.ARRAYSIZE)
		}
	case "append":
		arg := expr.Args[0]
		typ := c.typeInfo.Types[arg].Type
		if isByteArrayType(typ) {
			emit.Opcode(c.prog.BinWriter, opcode.CAT)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.OVER)
			emit.Opcode(c.prog.BinWriter, opcode.SWAP)
			emit.Opcode(c.prog.BinWriter, opcode.APPEND)
		}
	case "panic":
		arg := expr.Args[0]
		if isExprNil(arg) {
			emit.Opcode(c.prog.BinWriter, opcode.DROP)
			emit.Opcode(c.prog.BinWriter, opcode.THROW)
		} else if isStringType(c.typeInfo.Types[arg].Type) {
			ast.Walk(c, arg)
			emit.Syscall(c.prog.BinWriter, "Neo.Runtime.Log")
			emit.Opcode(c.prog.BinWriter, opcode.THROW)
		} else {
			c.prog.Err = errors.New("panic should have string or nil argument")
		}
	case "SHA256":
		emit.Opcode(c.prog.BinWriter, opcode.SHA256)
	case "SHA1":
		emit.Opcode(c.prog.BinWriter, opcode.SHA1)
	case "AppCall":
		numArgs := len(expr.Args) - 1
		c.emitReverse(numArgs)

		emit.Opcode(c.prog.BinWriter, opcode.APPCALL)
		buf := c.getByteArray(expr.Args[0])
		if len(buf) != 20 {
			c.prog.Err = errors.New("invalid script hash")
		}

		c.prog.WriteBytes(buf)
	case "Equals":
		emit.Opcode(c.prog.BinWriter, opcode.EQUAL)
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
		emit.Bytes(c.prog.BinWriter, bytes)
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
	emit.Bytes(c.prog.BinWriter, buf)
}

func (c *codegen) convertMap(lit *ast.CompositeLit) {
	emit.Opcode(c.prog.BinWriter, opcode.NEWMAP)
	for i := range lit.Elts {
		elem := lit.Elts[i].(*ast.KeyValueExpr)
		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		ast.Walk(c, elem.Key)
		ast.Walk(c, elem.Value)
		emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
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

	emit.Opcode(c.prog.BinWriter, opcode.NOP)
	emit.Int(c.prog.BinWriter, int64(strct.NumFields()))
	emit.Opcode(c.prog.BinWriter, opcode.NEWSTRUCT)

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
				emit.Opcode(c.prog.BinWriter, opcode.DUP)

				pos := indexOfStruct(strct, fieldName)
				emit.Int(c.prog.BinWriter, int64(pos))

				ast.Walk(c, f.Value)

				emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
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

		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		emit.Int(c.prog.BinWriter, int64(i))
		c.emitLoadConst(typeAndVal)
		emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
	}
}

func (c *codegen) convertToken(tok token.Token) {
	switch tok {
	case token.ADD_ASSIGN:
		emit.Opcode(c.prog.BinWriter, opcode.ADD)
	case token.SUB_ASSIGN:
		emit.Opcode(c.prog.BinWriter, opcode.SUB)
	case token.MUL_ASSIGN:
		emit.Opcode(c.prog.BinWriter, opcode.MUL)
	case token.QUO_ASSIGN:
		emit.Opcode(c.prog.BinWriter, opcode.DIV)
	case token.REM_ASSIGN:
		emit.Opcode(c.prog.BinWriter, opcode.MOD)
	case token.ADD:
		emit.Opcode(c.prog.BinWriter, opcode.ADD)
	case token.SUB:
		emit.Opcode(c.prog.BinWriter, opcode.SUB)
	case token.MUL:
		emit.Opcode(c.prog.BinWriter, opcode.MUL)
	case token.QUO:
		emit.Opcode(c.prog.BinWriter, opcode.DIV)
	case token.REM:
		emit.Opcode(c.prog.BinWriter, opcode.MOD)
	case token.LSS:
		emit.Opcode(c.prog.BinWriter, opcode.LT)
	case token.LEQ:
		emit.Opcode(c.prog.BinWriter, opcode.LTE)
	case token.GTR:
		emit.Opcode(c.prog.BinWriter, opcode.GT)
	case token.GEQ:
		emit.Opcode(c.prog.BinWriter, opcode.GTE)
	case token.EQL:
		emit.Opcode(c.prog.BinWriter, opcode.NUMEQUAL)
	case token.NEQ:
		emit.Opcode(c.prog.BinWriter, opcode.NUMNOTEQUAL)
	case token.DEC:
		emit.Opcode(c.prog.BinWriter, opcode.DEC)
	case token.INC:
		emit.Opcode(c.prog.BinWriter, opcode.INC)
	case token.NOT:
		emit.Opcode(c.prog.BinWriter, opcode.NOT)
	case token.AND:
		emit.Opcode(c.prog.BinWriter, opcode.AND)
	case token.OR:
		emit.Opcode(c.prog.BinWriter, opcode.OR)
	case token.SHL:
		emit.Opcode(c.prog.BinWriter, opcode.SHL)
	case token.SHR:
		emit.Opcode(c.prog.BinWriter, opcode.SHR)
	case token.XOR:
		emit.Opcode(c.prog.BinWriter, opcode.XOR)
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

func (c *codegen) compile(info *buildInfo, pkg *loader.PackageInfo) error {
	// Resolve the entrypoint of the program.
	main, mainFile := resolveEntryPoint(mainIdent, pkg)
	if main == nil {
		c.prog.Err = fmt.Errorf("could not find func main. Did you forget to declare it? ")
		return c.prog.Err
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

	return c.prog.Err
}

func newCodegen(info *buildInfo, pkg *loader.PackageInfo) *codegen {
	return &codegen{
		buildInfo: info,
		prog:      io.NewBufBinWriter(),
		l:         []int{},
		funcs:     map[string]*funcScope{},
		labels:    map[labelWithType]uint16{},
		typeInfo:  &pkg.Info,

		sequencePoints: make(map[string][]DebugSeqPoint),
	}
}

// CodeGen compiles the program to bytecode.
func CodeGen(info *buildInfo) ([]byte, *DebugInfo, error) {
	pkg := info.program.Package(info.initialPackage)
	c := newCodegen(info, pkg)

	if err := c.compile(info, pkg); err != nil {
		return nil, nil, err
	}

	buf := c.prog.Bytes()
	if err := c.writeJumps(buf); err != nil {
		return nil, nil, err
	}
	return buf, c.emitDebugInfo(), nil
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

func (c *codegen) writeJumps(b []byte) error {
	ctx := vm.NewContext(b)
	for op, _, err := ctx.Next(); err == nil && ctx.NextIP() < len(b); op, _, err = ctx.Next() {
		switch op {
		case opcode.JMP, opcode.JMPIFNOT, opcode.JMPIF, opcode.CALL:
			// we can't use arg returned by ctx.Next() because it is copied
			nextIP := ctx.NextIP()
			arg := b[nextIP-2:]

			index := binary.LittleEndian.Uint16(arg)
			if int(index) > len(c.l) {
				return fmt.Errorf("unexpected label number: %d (max %d)", index, len(c.l))
			}
			offset := c.l[index] - nextIP + 3
			if offset > math.MaxUint16 {
				return fmt.Errorf("label offset is too big at the instruction %d: %d (max %d)",
					nextIP-3, offset, math.MaxUint16)
			}
			binary.LittleEndian.PutUint16(arg, uint16(offset))
		}
	}
	return nil
}
