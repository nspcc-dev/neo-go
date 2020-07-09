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
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

	// A mapping of lambda functions into their scope.
	lambda map[string]*funcScope

	// Current funcScope being converted.
	scope *funcScope

	globals map[string]int

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

type varType int

const (
	varGlobal varType = iota
	varLocal
	varArgument
)

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

	typ, ok := t.Type.Underlying().(*types.Basic)
	if !ok {
		c.prog.Err = fmt.Errorf("compiler doesn't know how to convert this constant: %v", t)
		return
	}

	switch typ.Kind() {
	case types.Int, types.UntypedInt, types.Uint,
		types.Int8, types.Uint8,
		types.Int16, types.Uint16,
		types.Int32, types.Uint32, types.Int64, types.Uint64:
		val, _ := constant.Int64Val(t.Value)
		emit.Int(c.prog.BinWriter, val)
	case types.String, types.UntypedString:
		val := constant.StringVal(t.Value)
		emit.String(c.prog.BinWriter, val)
	case types.Bool, types.UntypedBool:
		val := constant.BoolVal(t.Value)
		emit.Bool(c.prog.BinWriter, val)
	default:
		c.prog.Err = fmt.Errorf("compiler doesn't know how to convert this basic type: %v", t)
		return
	}
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

// getVarIndex returns variable type and position in corresponding slot,
// according to current scope.
func (c *codegen) getVarIndex(name string) (varType, int) {
	if c.scope != nil {
		vt, val := c.scope.vars.getVarIndex(name)
		if val >= 0 {
			return vt, val
		}
	}
	if i, ok := c.globals[name]; ok {
		return varGlobal, i
	}

	return varLocal, c.scope.newVariable(varLocal, name)
}

func getBaseOpcode(t varType) (opcode.Opcode, opcode.Opcode) {
	switch t {
	case varGlobal:
		return opcode.LDSFLD0, opcode.STSFLD0
	case varLocal:
		return opcode.LDLOC0, opcode.STLOC0
	case varArgument:
		return opcode.LDARG0, opcode.STARG0
	default:
		panic("invalid type")
	}
}

// emitLoadVar loads specified variable to the evaluation stack.
func (c *codegen) emitLoadVar(name string) {
	t, i := c.getVarIndex(name)
	base, _ := getBaseOpcode(t)
	if i < 7 {
		emit.Opcode(c.prog.BinWriter, base+opcode.Opcode(i))
	} else {
		emit.Instruction(c.prog.BinWriter, base+7, []byte{byte(i)})
	}
}

// emitStoreVar stores top value from the evaluation stack in the specified variable.
func (c *codegen) emitStoreVar(name string) {
	if name == "_" {
		emit.Opcode(c.prog.BinWriter, opcode.DROP)
		return
	}
	t, i := c.getVarIndex(name)
	_, base := getBaseOpcode(t)
	if i < 7 {
		emit.Opcode(c.prog.BinWriter, base+opcode.Opcode(i))
	} else {
		emit.Instruction(c.prog.BinWriter, base+7, []byte{byte(i)})
	}
}

func (c *codegen) emitDefault(t types.Type) {
	switch t := t.Underlying().(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsInteger != 0:
			emit.Int(c.prog.BinWriter, 0)
		case info&types.IsString != 0:
			emit.Bytes(c.prog.BinWriter, []byte{})
		case info&types.IsBoolean != 0:
			emit.Bool(c.prog.BinWriter, false)
		default:
			emit.Opcode(c.prog.BinWriter, opcode.PUSHNULL)
		}
	case *types.Struct:
		num := t.NumFields()
		emit.Int(c.prog.BinWriter, int64(num))
		emit.Opcode(c.prog.BinWriter, opcode.NEWSTRUCT)
		for i := 0; i < num; i++ {
			emit.Opcode(c.prog.BinWriter, opcode.DUP)
			emit.Int(c.prog.BinWriter, int64(i))
			c.emitDefault(t.Field(i).Type())
			emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
		}
	default:
		emit.Opcode(c.prog.BinWriter, opcode.PUSHNULL)
	}
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

func (c *codegen) convertFuncDecl(file ast.Node, decl *ast.FuncDecl, pkg *types.Package) {
	var (
		f            *funcScope
		ok, isLambda bool
	)

	f, ok = c.funcs[decl.Name.Name]
	if ok {
		// If this function is a syscall or builtin we will not convert it to bytecode.
		if isSyscall(f) || isCustomBuiltin(f) {
			return
		}
		c.setLabel(f.label)
	} else if f, ok = c.lambda[decl.Name.Name]; ok {
		isLambda = ok
		c.setLabel(f.label)
	} else {
		f = c.newFunc(decl)
		f.pkg = pkg
	}

	f.rng.Start = uint16(c.prog.Len())
	c.scope = f
	ast.Inspect(decl, c.scope.analyzeVoidCalls) // @OPTIMIZE

	// All globals copied into the scope of the function need to be added
	// to the stack size of the function.
	sizeLoc := f.countLocals()
	if sizeLoc > 255 {
		c.prog.Err = errors.New("maximum of 255 local variables is allowed")
	}
	sizeArg := f.countArgs()
	if sizeArg > 255 {
		c.prog.Err = errors.New("maximum of 255 local variables is allowed")
	}
	if sizeLoc != 0 || sizeArg != 0 {
		emit.Instruction(c.prog.BinWriter, opcode.INITSLOT, []byte{byte(sizeLoc), byte(sizeArg)})
	}

	f.vars.newScope()
	defer f.vars.dropScope()

	// We need to handle methods, which in Go, is just syntactic sugar.
	// The method receiver will be passed in as first argument.
	// We check if this declaration has a receiver and load it into scope.
	//
	// FIXME: For now we will hard cast this to a struct. We can later fine tune this
	// to support other types.
	if decl.Recv != nil {
		for _, arg := range decl.Recv.List {
			// only create an argument here, it will be stored via INITSLOT
			c.scope.newVariable(varArgument, arg.Names[0].Name)
		}
	}

	// Load the arguments in scope.
	for _, arg := range decl.Type.Params.List {
		for _, id := range arg.Names {
			// only create an argument here, it will be stored via INITSLOT
			c.scope.newVariable(varArgument, id.Name)
		}
	}

	ast.Walk(c, decl.Body)

	// If we have reached the end of the function without encountering `return` statement,
	// we should clean alt.stack manually.
	// This can be the case with void and named-return functions.
	if !lastStmtIsReturn(decl) {
		c.saveSequencePoint(decl.Body)
		emit.Opcode(c.prog.BinWriter, opcode.RET)
	}

	f.rng.End = uint16(c.prog.Len() - 1)

	if !isLambda {
		for _, f := range c.lambda {
			c.convertFuncDecl(file, f.decl, pkg)
		}
		c.lambda = make(map[string]*funcScope)
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
				for _, id := range t.Names {
					if c.scope == nil {
						// it is a global declaration
						c.newGlobal(id.Name)
					} else {
						c.scope.newLocal(id.Name)
					}
					c.registerDebugVariable(id.Name, t.Type)
				}
				for i := range t.Names {
					if len(t.Values) != 0 {
						ast.Walk(c, t.Values[i])
					} else {
						c.emitDefault(c.typeOf(t.Type))
					}
					c.emitStoreVar(t.Names[i].Name)
				}
			}
		}
		return nil

	case *ast.AssignStmt:
		multiRet := len(n.Rhs) != len(n.Lhs)
		c.saveSequencePoint(n)
		// Assign operations are grouped https://github.com/golang/go/blob/master/src/go/types/stmt.go#L160
		isAssignOp := token.ADD_ASSIGN <= n.Tok && n.Tok <= token.AND_NOT_ASSIGN
		if isAssignOp {
			// RHS can contain exactly one expression, thus there is no need to iterate.
			ast.Walk(c, n.Lhs[0])
			ast.Walk(c, n.Rhs[0])
			c.convertToken(n.Tok)
		}
		for i := 0; i < len(n.Lhs); i++ {
			switch t := n.Lhs[i].(type) {
			case *ast.Ident:
				if n.Tok == token.DEFINE {
					if !multiRet {
						c.registerDebugVariable(t.Name, n.Rhs[i])
					}
					if t.Name != "_" {
						c.scope.newLocal(t.Name)
					}
				}
				if !isAssignOp && (i == 0 || !multiRet) {
					ast.Walk(c, n.Rhs[i])
				}
				c.emitStoreVar(t.Name)

			case *ast.SelectorExpr:
				if !isAssignOp {
					ast.Walk(c, n.Rhs[i])
				}
				strct, ok := c.typeOf(t.X).Underlying().(*types.Struct)
				if !ok {
					c.prog.Err = fmt.Errorf("nested selector assigns not supported yet")
					return nil
				}
				ast.Walk(c, t.X)                      // load the struct
				i := indexOfStruct(strct, t.Sel.Name) // get the index of the field
				c.emitStoreStructField(i)             // store the field

			// Assignments to index expressions.
			// slice[0] = 10
			case *ast.IndexExpr:
				if !isAssignOp {
					ast.Walk(c, n.Rhs[i])
				}
				ast.Walk(c, t.X)
				ast.Walk(c, t.Index)
				emit.Opcode(c.prog.BinWriter, opcode.ROT)
				emit.Opcode(c.prog.BinWriter, opcode.SETITEM)
			}
		}
		return nil

	case *ast.SliceExpr:
		name := n.X.(*ast.Ident).Name
		c.emitLoadVar(name)

		if n.Low != nil {
			ast.Walk(c, n.Low)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.PUSH0)
		}

		if n.High != nil {
			ast.Walk(c, n.High)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.OVER)
			emit.Opcode(c.prog.BinWriter, opcode.SIZE)
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

		if len(n.Results) == 0 {
			results := c.scope.decl.Type.Results
			if results.NumFields() != 0 {
				// function with named returns
				for i := len(results.List) - 1; i >= 0; i-- {
					names := results.List[i].Names
					for j := len(names) - 1; j >= 0; j-- {
						c.emitLoadVar(names[j].Name)
					}
				}
			}
		} else {
			// first result should be on top of the stack
			for i := len(n.Results) - 1; i >= 0; i-- {
				ast.Walk(c, n.Results[i])
			}
		}

		c.saveSequencePoint(n)
		emit.Opcode(c.prog.BinWriter, opcode.RET)
		return nil

	case *ast.IfStmt:
		c.scope.vars.newScope()
		defer c.scope.vars.dropScope()

		lIf := c.newLabel()
		lElse := c.newLabel()
		lElseEnd := c.newLabel()

		if n.Cond != nil {
			ast.Walk(c, n.Cond)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOTL, lElse)
		}

		c.setLabel(lIf)
		ast.Walk(c, n.Body)
		if n.Else != nil {
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, lElseEnd)
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
						emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOTL, lEnd)
					} else {
						emit.Jmp(c.prog.BinWriter, opcode.JMPIFL, lStart)
					}
				}
			}

			c.scope.vars.newScope()

			c.setLabel(lStart)
			last := len(cc.Body) - 1
			for j, stmt := range cc.Body {
				if j == last && isFallthroughStmt(stmt) {
					emit.Jmp(c.prog.BinWriter, opcode.JMPL, startLabels[i+1])
					break
				}
				ast.Walk(c, stmt)
			}
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, switchEnd)
			c.setLabel(lEnd)

			c.scope.vars.dropScope()
		}

		c.setLabel(switchEnd)
		c.dropStackLabel()

		c.currentSwitch = lastSwitch

		return nil

	case *ast.FuncLit:
		l := c.newLabel()
		c.newLambda(l, n)
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint16(buf, l)
		emit.Instruction(c.prog.BinWriter, opcode.PUSHA, buf)
		return nil

	case *ast.BasicLit:
		c.emitLoadConst(c.typeAndValueOf(n))
		return nil

	case *ast.Ident:
		if tv := c.typeAndValueOf(n); tv.Value != nil {
			c.emitLoadConst(tv)
		} else if n.Name == "nil" {
			emit.Opcode(c.prog.BinWriter, opcode.PUSHNULL)
		} else {
			c.emitLoadVar(n.Name)
		}
		return nil

	case *ast.CompositeLit:
		switch typ := c.typeOf(n).Underlying().(type) {
		case *types.Struct:
			c.convertStruct(n)
		case *types.Map:
			c.convertMap(n)
		default:
			ln := len(n.Elts)
			// ByteArrays needs a different approach than normal arrays.
			if isByteSlice(typ) {
				c.convertByteArray(n)
				return nil
			}
			for i := ln - 1; i >= 0; i-- {
				ast.Walk(c, n.Elts[i])
			}
			emit.Int(c.prog.BinWriter, int64(ln))
			emit.Opcode(c.prog.BinWriter, opcode.PACK)
		}

		return nil

	case *ast.BinaryExpr:
		switch n.Op {
		case token.LAND:
			next := c.newLabel()
			end := c.newLabel()
			ast.Walk(c, n.X)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFL, next)
			emit.Opcode(c.prog.BinWriter, opcode.PUSHF)
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, end)
			c.setLabel(next)
			ast.Walk(c, n.Y)
			c.setLabel(end)
			return nil

		case token.LOR:
			next := c.newLabel()
			end := c.newLabel()
			ast.Walk(c, n.X)
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOTL, next)
			emit.Opcode(c.prog.BinWriter, opcode.PUSHT)
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, end)
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
			tinfo := c.typeAndValueOf(n)
			if tinfo.Value != nil {
				c.emitLoadConst(tinfo)
				return nil
			}

			var checkForNull bool

			if isExprNil(n.X) {
				checkForNull = true
			} else {
				ast.Walk(c, n.X)
			}
			if isExprNil(n.Y) {
				checkForNull = true
			} else {
				ast.Walk(c, n.Y)
			}
			if checkForNull {
				emit.Opcode(c.prog.BinWriter, opcode.ISNULL)
				if n.Op == token.NEQ {
					emit.Opcode(c.prog.BinWriter, opcode.NOT)
				}

				return nil
			}

			switch {
			case n.Op == token.ADD:
				// VM has separate opcodes for number and string concatenation
				if isString(tinfo.Type) {
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
				if isString(c.typeOf(n.X)) {
					emit.Opcode(c.prog.BinWriter, opcode.NOTEQUAL)
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
			name      string
			numArgs   = len(n.Args)
			isBuiltin bool
			isFunc    bool
		)

		switch fun := n.Fun.(type) {
		case *ast.Ident:
			f, ok = c.funcs[fun.Name]
			isBuiltin = isGoBuiltin(fun.Name)
			if !ok && !isBuiltin {
				name = fun.Name
			}
			// distinguish lambda invocations from type conversions
			if fun.Obj != nil && fun.Obj.Kind == ast.Var {
				isFunc = true
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
			isBuiltin = isCustomBuiltin(f)
		case *ast.ArrayType:
			// For now we will assume that there are only byte slice conversions.
			// E.g. []byte("foobar") or []byte(scriptHash).
			ast.Walk(c, n.Args[0])
			c.emitConvert(stackitem.BufferT)
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
			typ, ok := c.typeOf(n.Fun).(*types.Signature)
			if ok && typ.Variadic() && !n.Ellipsis.IsValid() {
				// pack variadic args into an array only if last argument is not of form `...`
				varSize := len(n.Args) - typ.Params().Len() + 1
				c.emitReverse(varSize)
				emit.Int(c.prog.BinWriter, int64(varSize))
				emit.Opcode(c.prog.BinWriter, opcode.PACK)
				numArgs -= varSize - 1
			}
			c.emitReverse(numArgs)
		}

		// Check builtin first to avoid nil pointer on funcScope!
		switch {
		case isBuiltin:
			// Use the ident to check, builtins are not in func scopes.
			// We can be sure builtins are of type *ast.Ident.
			c.convertBuiltin(n)
		case name != "":
			// Function was not found thus is can be only an invocation of func-typed variable or type conversion.
			// We care only about string conversions because all others are effectively no-op in NeoVM.
			// E.g. one cannot write `bool(int(a))`, only `int32(int(a))`.
			if isString(c.typeOf(n.Fun)) {
				c.emitConvert(stackitem.ByteArrayT)
			} else if isFunc {
				c.emitLoadVar(name)
				emit.Opcode(c.prog.BinWriter, opcode.CALLA)
			}
		case isSyscall(f):
			c.convertSyscall(n, f.selector.Name, f.name)
		default:
			emit.Call(c.prog.BinWriter, opcode.CALLL, f.label)
		}

		return nil

	case *ast.SelectorExpr:
		strct, ok := c.typeOf(n.X).Underlying().(*types.Struct)
		if !ok {
			c.prog.Err = fmt.Errorf("selectors are supported only on structs")
			return nil
		}
		ast.Walk(c, n.X) // load the struct
		i := indexOfStruct(strct, n.Sel.Name)
		c.emitLoadField(i) // load the field
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
			c.emitStoreVar(ident.Name)
		}
		return nil

	case *ast.IndexExpr:
		// Walk the expression, this could be either an Ident or SelectorExpr.
		// This will load local whatever X is.
		ast.Walk(c, n.X)
		ast.Walk(c, n.Index)
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
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, end)
		case token.CONTINUE:
			post := c.getLabelOffset(labelPost, label)
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, post)
		}

		return nil

	case *ast.LabeledStmt:
		c.nextLabel = n.Label.Name

		ast.Walk(c, n.Stmt)

		return nil

	case *ast.BlockStmt:
		c.scope.vars.newScope()
		defer c.scope.vars.dropScope()

		for i := range n.List {
			ast.Walk(c, n.List[i])
		}

		return nil

	case *ast.ForStmt:
		c.scope.vars.newScope()
		defer c.scope.vars.dropScope()

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
			emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOTL, fend)
		}

		// Walk body followed by the iterator (post stmt).
		ast.Walk(c, n.Body)
		c.setLabel(fpost)
		if n.Post != nil {
			ast.Walk(c, n.Post)
		}

		// Jump back to condition.
		emit.Jmp(c.prog.BinWriter, opcode.JMPL, fstart)
		c.setLabel(fend)
		c.dropStackLabel()

		c.currentFor = lastLabel
		c.currentSwitch = lastSwitch

		return nil

	case *ast.RangeStmt:
		c.scope.vars.newScope()
		defer c.scope.vars.dropScope()

		start, label := c.generateLabel(labelStart)
		end := c.newNamedLabel(labelEnd, label)
		post := c.newNamedLabel(labelPost, label)

		lastFor := c.currentFor
		lastSwitch := c.currentSwitch
		c.currentFor = label
		c.currentSwitch = label

		ast.Walk(c, n.X)

		// Implementation is a bit different for slices and maps:
		// For slices we iterate index from 0 to len-1, storing array, len and index on stack.
		// For maps we iterate index from 0 to len-1, storing map, keyarray, size and index on stack.
		_, isMap := c.typeOf(n.X).Underlying().(*types.Map)
		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		if isMap {
			emit.Opcode(c.prog.BinWriter, opcode.KEYS)
			emit.Opcode(c.prog.BinWriter, opcode.DUP)
		}
		emit.Opcode(c.prog.BinWriter, opcode.SIZE)
		emit.Opcode(c.prog.BinWriter, opcode.PUSH0)

		stackSize := 3 // slice, len(slice), index
		if isMap {
			stackSize++ // map, keys, len(keys), index in keys
		}
		c.pushStackLabel(label, stackSize)
		c.setLabel(start)

		emit.Opcode(c.prog.BinWriter, opcode.OVER)
		emit.Opcode(c.prog.BinWriter, opcode.OVER)
		emit.Jmp(c.prog.BinWriter, opcode.JMPLEL, end)

		var keyLoaded bool
		needValue := n.Value != nil && n.Value.(*ast.Ident).Name != "_"
		if n.Key != nil && n.Key.(*ast.Ident).Name != "_" {
			if isMap {
				c.rangeLoadKey()
				if needValue {
					emit.Opcode(c.prog.BinWriter, opcode.DUP)
					keyLoaded = true
				}
			} else {
				emit.Opcode(c.prog.BinWriter, opcode.DUP)
			}
			c.emitStoreVar(n.Key.(*ast.Ident).Name)
		}
		if needValue {
			if !isMap || !keyLoaded {
				c.rangeLoadKey()
			}
			if isMap {
				// we have loaded only key from key array, now load value
				emit.Int(c.prog.BinWriter, 4)
				emit.Opcode(c.prog.BinWriter, opcode.PICK) // load map itself (+1 because key was pushed)
				emit.Opcode(c.prog.BinWriter, opcode.SWAP) // key should be on top
				emit.Opcode(c.prog.BinWriter, opcode.PICKITEM)
			}
			c.emitStoreVar(n.Value.(*ast.Ident).Name)
		}

		ast.Walk(c, n.Body)

		c.setLabel(post)

		emit.Opcode(c.prog.BinWriter, opcode.INC)
		emit.Jmp(c.prog.BinWriter, opcode.JMPL, start)

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
		typ := toNeoType(c.typeOf(n.Type))
		emit.Instruction(c.prog.BinWriter, opcode.CONVERT, []byte{byte(typ)})
		return nil
	}
	return c
}

func (c *codegen) rangeLoadKey() {
	emit.Int(c.prog.BinWriter, 2)
	emit.Opcode(c.prog.BinWriter, opcode.PICK) // load keys
	emit.Opcode(c.prog.BinWriter, opcode.OVER) // load index in key array
	emit.Opcode(c.prog.BinWriter, opcode.PICKITEM)
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
	case 0, 1:
	case 2:
		emit.Opcode(c.prog.BinWriter, opcode.SWAP)
	case 3:
		emit.Opcode(c.prog.BinWriter, opcode.REVERSE3)
	case 4:
		emit.Opcode(c.prog.BinWriter, opcode.REVERSE4)
	default:
		emit.Int(c.prog.BinWriter, int64(num))
		emit.Opcode(c.prog.BinWriter, opcode.REVERSEN)
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
	t, ok := c.typeOf(expr).Underlying().(*types.Basic)
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
		if !isByteSlice(c.typeOf(t.Type)) {
			return nil
		}
		buf := make([]byte, len(t.Elts))
		for i := 0; i < len(t.Elts); i++ {
			t := c.typeAndValueOf(t.Elts[i])
			val, _ := constant.Int64Val(t.Value)
			buf[i] = byte(val)
		}
		return buf
	case *ast.CallExpr:
		if tv := c.typeAndValueOf(t.Args[0]); tv.Value != nil {
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
	emit.Syscall(c.prog.BinWriter, api)
	switch name {
	case "GetTransaction", "GetBlock":
		c.emitConvert(stackitem.StructT)
	}

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
		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		emit.Opcode(c.prog.BinWriter, opcode.ISNULL)
		emit.Instruction(c.prog.BinWriter, opcode.JMPIF, []byte{2 + 1 + 2})
		emit.Opcode(c.prog.BinWriter, opcode.SIZE)
		emit.Instruction(c.prog.BinWriter, opcode.JMP, []byte{2 + 1 + 1})
		emit.Opcode(c.prog.BinWriter, opcode.DROP)
		emit.Opcode(c.prog.BinWriter, opcode.PUSH0)
	case "append":
		arg := expr.Args[0]
		typ := c.typeInfo.Types[arg].Type
		c.emitReverse(len(expr.Args))
		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		emit.Opcode(c.prog.BinWriter, opcode.ISNULL)
		emit.Instruction(c.prog.BinWriter, opcode.JMPIFNOT, []byte{2 + 3})
		if isByteSlice(typ) {
			emit.Opcode(c.prog.BinWriter, opcode.DROP)
			emit.Opcode(c.prog.BinWriter, opcode.PUSH0)
			emit.Opcode(c.prog.BinWriter, opcode.NEWBUFFER)
		} else {
			emit.Opcode(c.prog.BinWriter, opcode.DROP)
			emit.Opcode(c.prog.BinWriter, opcode.NEWARRAY0)
			emit.Opcode(c.prog.BinWriter, opcode.NOP)
		}
		// Jump target.
		for range expr.Args[1:] {
			if isByteSlice(typ) {
				emit.Opcode(c.prog.BinWriter, opcode.SWAP)
				emit.Opcode(c.prog.BinWriter, opcode.CAT)
			} else {
				emit.Opcode(c.prog.BinWriter, opcode.DUP)
				emit.Opcode(c.prog.BinWriter, opcode.ROT)
				emit.Opcode(c.prog.BinWriter, opcode.APPEND)
			}
		}
	case "panic":
		arg := expr.Args[0]
		if isExprNil(arg) {
			emit.Opcode(c.prog.BinWriter, opcode.DROP)
			emit.Opcode(c.prog.BinWriter, opcode.THROW)
		} else if isString(c.typeInfo.Types[arg].Type) {
			ast.Walk(c, arg)
			emit.Syscall(c.prog.BinWriter, "System.Runtime.Log")
			emit.Opcode(c.prog.BinWriter, opcode.THROW)
		} else {
			c.prog.Err = errors.New("panic should have string or nil argument")
		}
	case "ToInteger", "ToByteArray", "ToBool":
		typ := stackitem.IntegerT
		switch name {
		case "ToByteArray":
			typ = stackitem.ByteArrayT
		case "ToBool":
			typ = stackitem.BooleanT
		}
		c.emitConvert(typ)
	case "SHA256":
		emit.Syscall(c.prog.BinWriter, "Neo.Crypto.SHA256")
	case "AppCall":
		c.emitReverse(len(expr.Args))
		buf := c.getByteArray(expr.Args[0])
		if buf != nil && len(buf) != 20 {
			c.prog.Err = errors.New("invalid script hash")
		}
		emit.Syscall(c.prog.BinWriter, "System.Contract.Call")
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
		c.emitConvert(stackitem.BufferT)
	}
}

// transformArgs returns a list of function arguments
// which should be put on stack.
// There are special cases for builtins:
// 1. With FromAddress, parameter conversion is happening at compile-time
//    so there is no need to push parameters on stack and perform an actual call
// 2. With panic, generated code depends on if argument was nil or a string so
//    it should be handled accordingly.
func transformArgs(fun ast.Expr, args []ast.Expr) []ast.Expr {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		if f.Sel.Name == "FromAddress" {
			return args[1:]
		}
	case *ast.Ident:
		if f.Name == "panic" {
			return args[1:]
		}
	}

	return args
}

// emitConvert converts top stack item to the specified type.
func (c *codegen) emitConvert(typ stackitem.Type) {
	emit.Instruction(c.prog.BinWriter, opcode.CONVERT, []byte{byte(typ)})
}

func (c *codegen) convertByteArray(lit *ast.CompositeLit) {
	buf := make([]byte, len(lit.Elts))
	for i := 0; i < len(lit.Elts); i++ {
		t := c.typeAndValueOf(lit.Elts[i])
		val, _ := constant.Int64Val(t.Value)
		buf[i] = byte(val)
	}
	emit.Bytes(c.prog.BinWriter, buf)
	c.emitConvert(stackitem.BufferT)
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
	strct, ok := c.typeOf(lit).Underlying().(*types.Struct)
	if !ok {
		c.prog.Err = fmt.Errorf("the given literal is not of type struct: %v", lit)
		return
	}

	emit.Opcode(c.prog.BinWriter, opcode.NOP)
	emit.Int(c.prog.BinWriter, int64(strct.NumFields()))
	emit.Opcode(c.prog.BinWriter, opcode.NEWSTRUCT)

	keyedLit := len(lit.Elts) > 0
	if keyedLit {
		_, ok := lit.Elts[0].(*ast.KeyValueExpr)
		keyedLit = keyedLit && ok
	}
	// We need to locally store all the fields, even if they are not initialized.
	// We will initialize all fields to their "zero" value.
	for i := 0; i < strct.NumFields(); i++ {
		sField := strct.Field(i)
		var initialized bool

		emit.Opcode(c.prog.BinWriter, opcode.DUP)
		emit.Int(c.prog.BinWriter, int64(i))

		if !keyedLit {
			if len(lit.Elts) > i {
				ast.Walk(c, lit.Elts[i])
				initialized = true
			}
		} else {
			// Fields initialized by the program.
			for _, field := range lit.Elts {
				f := field.(*ast.KeyValueExpr)
				fieldName := f.Key.(*ast.Ident).Name

				if sField.Name() == fieldName {
					ast.Walk(c, f.Value)
					initialized = true
					break
				}
			}
		}
		if !initialized {
			c.emitDefault(sField.Type())
		}
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

func (c *codegen) newLambda(u uint16, lit *ast.FuncLit) {
	name := fmt.Sprintf("lambda@%d", u)
	c.lambda[name] = newFuncScope(&ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: lit.Type,
		Body: lit.Body,
	}, u)
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
			c.resolveFuncDecls(f, pkg.Pkg)
		}
	}

	c.traverseGlobals(mainFile)

	// convert the entry point first.
	c.convertFuncDecl(mainFile, main, pkg.Pkg)

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
						c.convertFuncDecl(f, n, k)
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
		lambda:    map[string]*funcScope{},
		globals:   map[string]int{},
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
	return buf, c.emitDebugInfo(buf), nil
}

func (c *codegen) resolveFuncDecls(f *ast.File, pkg *types.Package) {
	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			if n.Name.Name != mainIdent {
				c.newFunc(n)
				c.funcs[n.Name.Name].pkg = pkg
			}
		}
	}
}

func (c *codegen) writeJumps(b []byte) error {
	ctx := vm.NewContext(b)
	for op, _, err := ctx.Next(); err == nil && ctx.NextIP() < len(b); op, _, err = ctx.Next() {
		switch op {
		case opcode.JMP, opcode.JMPIFNOT, opcode.JMPIF, opcode.CALL,
			opcode.JMPEQ, opcode.JMPNE,
			opcode.JMPGT, opcode.JMPGE, opcode.JMPLE, opcode.JMPLT:
			// Noop, assumed to be correct already. If you're fixing #905,
			// make sure not to break "len" and "append" handling above.
		case opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL,
			opcode.JMPEQL, opcode.JMPNEL,
			opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLEL, opcode.JMPLTL,
			opcode.CALLL, opcode.PUSHA:
			// we can't use arg returned by ctx.Next() because it is copied
			nextIP := ctx.NextIP()
			arg := b[nextIP-4:]

			index := binary.LittleEndian.Uint16(arg)
			if int(index) > len(c.l) {
				return fmt.Errorf("unexpected label number: %d (max %d)", index, len(c.l))
			}
			var offset int
			if op == opcode.PUSHA {
				offset = c.l[index]
			} else {
				offset = c.l[index] - nextIP + 5
			}
			if offset > math.MaxInt32 || offset < math.MinInt32 {
				return fmt.Errorf("label offset is too big at the instruction %d: %d (max %d, min %d)",
					nextIP-5, offset, math.MaxInt32, math.MinInt32)
			}
			binary.LittleEndian.PutUint32(arg, uint32(offset))
		}
	}
	return nil
}
