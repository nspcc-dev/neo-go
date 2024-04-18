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
	"math/big"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"golang.org/x/tools/go/packages"
)

type codegen struct {
	// Information about the program with all its dependencies.
	buildInfo *buildInfo

	// prog holds the output buffer.
	prog *io.BufBinWriter

	// Type information.
	typeInfo *types.Info
	// pkgInfoInline is a stack of type information for packages containing inline functions.
	pkgInfoInline []*packages.Package

	// A mapping of func identifiers with their scope.
	funcs map[string]*funcScope

	// A mapping of lambda functions into their scope.
	lambda map[string]*funcScope

	// reverseOffsetMap maps function offsets to a local variable count.
	reverseOffsetMap map[int]nameWithLocals

	// Current funcScope being converted.
	scope *funcScope

	globals map[string]int
	// staticVariables contains global (static in NDX-DN11) variable names and types.
	staticVariables []string
	// initVariables contains variables local to `_initialize` method.
	initVariables []string
	// deployVariables contains variables local to `_initialize` method.
	deployVariables []string

	// A mapping from label's names to their ids.
	labels map[labelWithType]uint16
	// A list of nested label names together with evaluation stack depth.
	labelList []labelWithStackSize
	// inlineContext contains info about inlined function calls.
	inlineContext []inlineContextSingle
	// globalInlineCount contains the amount of auxiliary variables introduced by
	// function inlining during global variables initialization.
	globalInlineCount int

	// A label for the for-loop being currently visited.
	currentFor string
	// A label for the switch statement being visited.
	currentSwitch string
	// A label to be used in the next statement.
	nextLabel string

	// sequencePoints is a mapping from the method name to a slice
	// containing info about mapping from opcode's offset
	// to a text span in the source file.
	sequencePoints map[string][]DebugSeqPoint

	// initEndOffset specifies the end of the initialization method.
	initEndOffset int
	// deployEndOffset specifies the end of the deployment method.
	deployEndOffset int

	// importMap contains mapping from package aliases to full package names for the current file.
	importMap map[string]string

	// constMap contains constants from foreign packages.
	constMap map[string]types.TypeAndValue

	// currPkg is the current package being processed.
	currPkg *packages.Package

	// mainPkg is the main package metadata.
	mainPkg *packages.Package

	// packages contains packages in the order they were loaded.
	packages     []string
	packageCache map[string]*packages.Package

	// exceptionIndex is the index of the static slot where the exception is stored.
	exceptionIndex int

	// documents contains paths to all files used by the program.
	documents []string
	// docIndex maps the file path to the index in the documents array.
	docIndex map[string]int

	// emittedEvents contains all events emitted by the contract.
	emittedEvents map[string][]EmittedEventInfo

	// invokedContracts contains invoked methods of other contracts.
	invokedContracts map[util.Uint160][]string

	// Label table for recording jump destinations.
	l []int

	// Tokens for CALLT instruction
	callTokens []nef.MethodToken
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

type nameWithLocals struct {
	name  string
	count int
}

type inlineContextSingle struct {
	// labelOffset contains size of labelList at the start of inline call processing.
	// For such calls, we need to drop only the newly created part of stack.
	labelOffset int
	// returnLabel contains label ID pointing to the first instruction right after the call.
	returnLabel uint16
}

type varType int

const (
	varGlobal varType = iota
	varLocal
	varArgument
)

// ErrUnsupportedTypeAssertion is returned when type assertion statement is not supported by the compiler.
var ErrUnsupportedTypeAssertion = errors.New("type assertion with two return values is not supported")

// newLabel creates a new label to jump to.
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

// newNamedLabel creates a new label with the specified name.
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
		types.Int32, types.Uint32, types.Int64:
		val, _ := constant.Int64Val(t.Value)
		emit.Int(c.prog.BinWriter, val)
	case types.Uint64:
		val, _ := constant.Int64Val(t.Value)
		emit.BigInt(c.prog.BinWriter, new(big.Int).SetUint64(uint64(val)))
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
	emit.Opcodes(c.prog.BinWriter, opcode.PICKITEM)
}

func (c *codegen) emitStoreStructField(i int) {
	emit.Int(c.prog.BinWriter, int64(i))
	emit.Opcodes(c.prog.BinWriter, opcode.ROT, opcode.SETITEM)
}

// getVarIndex returns variable type and position in the corresponding slot,
// according to the current scope.
func (c *codegen) getVarIndex(pkg string, name string) *varInfo {
	if pkg == "" {
		if c.scope != nil {
			vi := c.scope.vars.getVarInfo(name)
			if vi != nil {
				return vi
			}
		}
	}
	if i, ok := c.globals[c.getIdentName(pkg, name)]; ok {
		return &varInfo{refType: varGlobal, index: i}
	}

	c.scope.newVariable(varLocal, name)
	return c.scope.vars.getVarInfo(name)
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

// emitLoadVar loads the specified variable to the evaluation stack.
func (c *codegen) emitLoadVar(pkg string, name string) {
	vi := c.getVarIndex(pkg, name)
	if vi.ctx != nil && c.typeAndValueOf(vi.ctx.expr).Value != nil {
		c.emitLoadConst(c.typeAndValueOf(vi.ctx.expr))
		return
	} else if vi.ctx != nil {
		var oldScope []map[string]varInfo
		oldMap := c.importMap
		c.importMap = vi.ctx.importMap
		if c.scope != nil {
			oldScope = c.scope.vars.locals
			c.scope.vars.locals = vi.ctx.scope
		}

		ast.Walk(c, vi.ctx.expr)

		if c.scope != nil {
			c.scope.vars.locals = oldScope
		}
		c.importMap = oldMap
		return
	} else if vi.index == unspecifiedVarIndex {
		emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
		return
	}
	c.emitLoadByIndex(vi.refType, vi.index)
}

// emitLoadByIndex loads the specified variable type with index i.
func (c *codegen) emitLoadByIndex(t varType, i int) {
	base, _ := getBaseOpcode(t)
	if i < 7 {
		emit.Opcodes(c.prog.BinWriter, base+opcode.Opcode(i))
	} else {
		emit.Instruction(c.prog.BinWriter, base+7, []byte{byte(i)})
	}
}

// emitStoreVar stores top value from the evaluation stack in the specified variable.
func (c *codegen) emitStoreVar(pkg string, name string) {
	if name == "_" {
		emit.Opcodes(c.prog.BinWriter, opcode.DROP)
		return
	}
	vi := c.getVarIndex(pkg, name)
	c.emitStoreByIndex(vi.refType, vi.index)
}

// emitLoadByIndex stores top value in the specified variable type with index i.
func (c *codegen) emitStoreByIndex(t varType, i int) {
	_, base := getBaseOpcode(t)
	if i < 7 {
		emit.Opcodes(c.prog.BinWriter, base+opcode.Opcode(i))
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
			emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
		}
	case *types.Struct:
		num := t.NumFields()
		for i := num - 1; i >= 0; i-- {
			c.emitDefault(t.Field(i).Type())
		}
		emit.Int(c.prog.BinWriter, int64(num))
		emit.Opcodes(c.prog.BinWriter, opcode.PACKSTRUCT)
	default:
		emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
	}
}

// convertGlobals traverses the AST and only converts global declarations.
// If we call this in convertFuncDecl, it will load all global variables
// into the scope of the function.
func (c *codegen) convertGlobals(f *ast.File) {
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

func isInitFunc(decl *ast.FuncDecl) bool {
	return decl.Name.Name == "init" && decl.Recv == nil &&
		decl.Type.Params.NumFields() == 0 &&
		decl.Type.Results.NumFields() == 0
}

func (c *codegen) isVerifyFunc(decl *ast.FuncDecl) bool {
	return decl.Name.Name == "Verify" && decl.Recv == nil &&
		decl.Type.Results.NumFields() == 1 &&
		isBool(c.typeOf(decl.Type.Results.List[0].Type))
}

func (c *codegen) clearSlots(n int) {
	for i := 0; i < n; i++ {
		emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
		c.emitStoreByIndex(varLocal, i)
	}
}

// convertInitFuncs converts `init()` functions in file f and returns
// the number of locals in the last processed definition as well as maximum locals number encountered.
func (c *codegen) convertInitFuncs(f *ast.File, pkg *types.Package, lastCount int) (int, int) {
	maxCount := -1
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if isInitFunc(n) {
				if lastCount != -1 {
					c.clearSlots(lastCount)
				}

				f := c.convertFuncDecl(f, n, pkg)
				lastCount = f.vars.localsCnt
				if lastCount > maxCount {
					maxCount = lastCount
				}
			}
		case *ast.GenDecl:
			return false
		}
		return true
	})
	return lastCount, maxCount
}

func isDeployFunc(decl *ast.FuncDecl) bool {
	if decl.Name.Name != "_deploy" || decl.Recv != nil ||
		decl.Type.Params.NumFields() != 2 ||
		decl.Type.Results.NumFields() != 0 {
		return false
	}
	typ, ok := decl.Type.Params.List[1].Type.(*ast.Ident)
	return ok && typ.Name == "bool"
}

func (c *codegen) convertDeployFuncs() int {
	maxCount, lastCount := 0, -1
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		ast.Inspect(f, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				if isDeployFunc(n) {
					if lastCount != -1 {
						c.clearSlots(lastCount)
					}

					f := c.convertFuncDecl(f, n, pkg)
					lastCount = f.vars.localsCnt
					if lastCount > maxCount {
						maxCount = lastCount
					}
				}
			case *ast.GenDecl:
				return false
			}
			return true
		})
	})
	return maxCount
}

func (c *codegen) convertFuncDecl(file ast.Node, decl *ast.FuncDecl, pkg *types.Package) *funcScope {
	var (
		f            *funcScope
		ok, isLambda bool
	)
	isInit := isInitFunc(decl)
	isDeploy := isDeployFunc(decl)
	if isInit || isDeploy {
		f = c.newFuncScope(decl, c.newLabel())
	} else {
		f, ok = c.funcs[c.getFuncNameFromDecl("", decl)]
		if ok {
			// If this function is a syscall we will not convert it to bytecode.
			// If it's a potential custom builtin then it needs more specific usages research,
			// thus let's emit the code for it.
			if isSyscall(f) {
				return f
			}
			c.setLabel(f.label)
		} else if f, ok = c.lambda[c.getIdentName("", decl.Name.Name)]; ok {
			isLambda = ok
			c.setLabel(f.label)
		} else {
			f = c.newFunc(decl)
		}
	}

	f.rng.Start = uint16(c.prog.Len())
	c.scope = f
	ast.Inspect(decl, c.scope.analyzeVoidCalls) // @OPTIMIZE

	// All globals copied into the scope of the function need to be added
	// to the stack size of the function.
	if !isInit && !isDeploy {
		sizeArg := f.countArgs()
		if sizeArg > 255 {
			c.prog.Err = errors.New("maximum of 255 local variables is allowed")
		}
		emit.Instruction(c.prog.BinWriter, opcode.INITSLOT, []byte{byte(0), byte(sizeArg)})
	}

	f.vars.newScope()
	defer f.vars.dropScope()

	// We need to handle methods, which in Go, is just syntactic sugar.
	// The method receiver will be passed in as the first argument.
	// We check if this declaration has a receiver and load it into the scope.
	//
	// FIXME: For now, we will hard cast this to a struct. We can later fine tune this
	// to support other types.
	if decl.Recv != nil {
		for _, arg := range decl.Recv.List {
			// Use underscore instead of unnamed receiver name, e.g.:
			//  func (MyCustomStruct) DoSmth(arg1 int) {...}
			// Unnamed receiver will never be referenced, thus we can use the same approach as for multiple unnamed parameters handling, see #2204.
			recvName := "_"
			if len(arg.Names) != 0 {
				recvName = arg.Names[0].Name
			}
			// only create an argument here, it will be stored via INITSLOT
			c.scope.newVariable(varArgument, recvName)
		}
	}

	// Load the arguments in scope.
	for _, arg := range decl.Type.Params.List {
		for _, id := range arg.Names {
			// only create an argument here, it will be stored via INITSLOT
			c.scope.newVariable(varArgument, id.Name)
		}
	}

	// Emit defaults for named returns.
	if decl.Type.Results.NumFields() != 0 {
		for _, arg := range decl.Type.Results.List {
			for _, id := range arg.Names {
				if id.Name != "_" {
					i := c.scope.newLocal(id.Name)
					c.emitDefault(c.typeOf(arg.Type))
					c.emitStoreByIndex(varLocal, i)
				}
			}
		}
	}

	ast.Walk(c, decl.Body)

	// If we have reached the end of the function without encountering `return` statement,
	// we should clean alt.stack manually.
	// This can be the case with void and named-return functions.
	if !isInit && !isDeploy && !lastStmtIsReturn(decl.Body) {
		c.processDefers()
		c.saveSequencePoint(decl.Body)
		emit.Opcodes(c.prog.BinWriter, opcode.RET)
	}

	if isInit {
		c.initVariables = append(c.initVariables, f.variables...)
	} else if isDeploy {
		c.deployVariables = append(c.deployVariables, f.variables...)
	}

	f.rng.End = uint16(c.prog.Len() - 1)

	if !isLambda {
		for _, f := range c.lambda {
			if _, ok := c.lambda[c.getIdentName("", f.decl.Name.Name)]; !ok {
				panic("ICE: lambda name doesn't match map key")
			}
			c.convertFuncDecl(file, f.decl, pkg)
		}
		c.lambda = make(map[string]*funcScope)
	}

	if !isInit && !isDeploy {
		c.reverseOffsetMap[int(f.rng.Start)] = nameWithLocals{
			name:  f.name,
			count: f.vars.localsCnt,
		}
	}
	return f
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
		// Filter out generics usage.
		err := c.checkGenericsGenDecl(n, c.currPkg.PkgPath)
		if err != nil {
			c.prog.Err = err
			return nil // Program is invalid.
		}

		if n.Tok == token.VAR || n.Tok == token.CONST {
			c.saveSequencePoint(n)
		}
		if n.Tok == token.CONST {
			for _, spec := range n.Specs {
				vs := spec.(*ast.ValueSpec)
				for i := range vs.Names {
					obj := c.currPkg.Types.Scope().Lookup(vs.Names[i].Name)
					if obj != nil { // can be nil if unused
						c.constMap[c.getIdentName("", vs.Names[i].Name)] = types.TypeAndValue{
							Type:  obj.Type(),
							Value: obj.(*types.Const).Val(),
						}
					}
				}
			}
			return nil
		}
		for _, spec := range n.Specs {
			switch t := spec.(type) {
			case *ast.ValueSpec:
				// Filter out type assertion with two return values: var i, ok = v.(int)
				if len(t.Names) == 2 && len(t.Values) == 1 && n.Tok == token.VAR {
					err := checkTypeAssertWithOK(t.Values[0])
					if err != nil {
						c.prog.Err = err
						return nil
					}
				}
				multiRet := n.Tok == token.VAR && len(t.Values) != 0 && len(t.Names) != len(t.Values)
				for _, id := range t.Names {
					if id.Name != "_" {
						if c.scope == nil {
							// it is a global declaration
							c.newGlobal("", id.Name)
						} else {
							c.scope.newLocal(id.Name)
						}
						if !multiRet {
							c.registerDebugVariable(id.Name, t.Type)
						}
					}
				}
				for i, id := range t.Names {
					if id.Name != "_" {
						if len(t.Values) != 0 {
							if i == 0 || !multiRet {
								ast.Walk(c, t.Values[i])
							}
						} else {
							c.emitDefault(c.typeOf(t.Type))
						}
						c.emitStoreVar("", t.Names[i].Name)
						continue
					}
					// If var decl contains call then the code should be emitted for it, otherwise - do not evaluate.
					if len(t.Values) == 0 {
						continue
					}
					var hasCall bool
					if i == 0 || !multiRet {
						hasCall = containsCall(t.Values[i])
					}
					if hasCall {
						ast.Walk(c, t.Values[i])
					}
					if hasCall || i != 0 && multiRet {
						c.emitStoreVar("", "_") // drop unused after walk
					}
				}
			}
		}
		return nil

	case *ast.AssignStmt:
		// Filter out type assertion with two return values: i, ok = v.(int)
		if len(n.Lhs) == 2 && len(n.Rhs) == 1 && (n.Tok == token.DEFINE || n.Tok == token.ASSIGN) {
			err := checkTypeAssertWithOK(n.Rhs[0])
			if err != nil {
				c.prog.Err = err
				return nil
			}
		}
		multiRet := len(n.Rhs) != len(n.Lhs)
		c.saveSequencePoint(n)
		// Assign operations are grouped https://github.com/golang/go/blob/master/src/go/types/stmt.go#L160
		isAssignOp := token.ADD_ASSIGN <= n.Tok && n.Tok <= token.AND_NOT_ASSIGN
		if isAssignOp {
			// RHS can contain exactly one expression, thus there is no need to iterate.
			ast.Walk(c, n.Lhs[0])
			ast.Walk(c, n.Rhs[0])
			c.emitToken(n.Tok, c.typeOf(n.Rhs[0]))
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
				c.emitStoreVar("", t.Name)

			case *ast.SelectorExpr:
				if !isAssignOp {
					ast.Walk(c, n.Rhs[i])
				}
				typ := c.typeOf(t.X)
				if c.isInvalidType(typ) {
					// Store to other package global variable.
					c.emitStoreVar(t.X.(*ast.Ident).Name, t.Sel.Name)
					return nil
				}
				strct, ok := c.getStruct(typ)
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
				emit.Opcodes(c.prog.BinWriter, opcode.ROT, opcode.SETITEM)
			}
		}
		return nil

	case *ast.SliceExpr:
		if isCompoundSlice(c.typeOf(n.X).Underlying()) {
			c.prog.Err = errors.New("subslices are supported only for []byte")
			return nil
		}

		ast.Walk(c, n.X)

		if n.Low != nil {
			ast.Walk(c, n.Low)
		} else {
			emit.Opcodes(c.prog.BinWriter, opcode.PUSH0)
		}

		if n.High != nil {
			ast.Walk(c, n.High)
		} else {
			emit.Opcodes(c.prog.BinWriter, opcode.OVER, opcode.SIZE)
		}

		emit.Opcodes(c.prog.BinWriter, opcode.OVER, opcode.SUB, opcode.SUBSTR)

		return nil

	case *ast.ReturnStmt:
		l := c.newLabel()
		c.setLabel(l)

		cnt := 0
		start := 0
		if len(c.inlineContext) > 0 {
			start = c.inlineContext[len(c.inlineContext)-1].labelOffset
		}
		for i := start; i < len(c.labelList); i++ {
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
						if names[j].Name == "_" {
							c.emitDefault(c.typeOf(results.List[i].Type))
						} else {
							c.emitLoadVar("", names[j].Name)
						}
					}
				}
			}
		} else {
			// first result should be on top of the stack
			for i := len(n.Results) - 1; i >= 0; i-- {
				ast.Walk(c, n.Results[i])
			}
		}

		c.processDefers()

		c.saveSequencePoint(n)
		if len(c.pkgInfoInline) == 0 {
			emit.Opcodes(c.prog.BinWriter, opcode.RET)
		} else {
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, c.inlineContext[len(c.inlineContext)-1].returnLabel)
		}
		return nil

	case *ast.IfStmt:
		c.scope.vars.newScope()
		defer c.scope.vars.dropScope()

		lIf := c.newLabel()
		lElse := c.newLabel()
		lElseEnd := c.newLabel()

		if n.Init != nil {
			ast.Walk(c, n.Init)
		}
		if n.Cond != nil {
			c.emitBoolExpr(n.Cond, true, false, lElse)
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
		eqOpcode := opcode.EQUAL
		if n.Tag != nil {
			ast.Walk(c, n.Tag)
			eqOpcode, _ = convertToken(token.EQL, c.typeOf(n.Tag))
		} else {
			emit.Bool(c.prog.BinWriter, true)
		}
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
					emit.Opcodes(c.prog.BinWriter, opcode.DUP)
					ast.Walk(c, cc.List[j])
					emit.Opcodes(c.prog.BinWriter, eqOpcode)
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
		var found bool
		var l uint16
		for _, fs := range c.lambda {
			if fs.decl.Body == n.Body {
				found = true
				l = fs.label
				break
			}
		}
		if !found {
			l = c.newLabel()
			c.newLambda(l, n)
		}

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint16(buf, l)
		emit.Instruction(c.prog.BinWriter, opcode.PUSHA, buf)
		return nil

	case *ast.BasicLit:
		c.emitLoadConst(c.typeAndValueOf(n))
		return nil

	case *ast.StarExpr:
		_, ok := c.getStruct(c.typeOf(n.X))
		if !ok {
			c.prog.Err = errors.New("dereferencing is only supported on structs")
			return nil
		}
		ast.Walk(c, n.X)
		c.emitConvert(stackitem.StructT)
		return nil

	case *ast.Ident:
		if tv := c.typeAndValueOf(n); tv.Value != nil {
			c.emitLoadConst(tv)
		} else if n.Name == "nil" {
			emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
		} else {
			c.emitLoadVar("", n.Name)
		}
		return nil

	case *ast.CompositeLit:
		t := c.typeOf(n)
		switch typ := t.Underlying().(type) {
		case *types.Struct:
			c.convertStruct(n, false)
		case *types.Map:
			c.convertMap(n)
		default:
			if tn, ok := t.(*types.Named); ok && isInteropPath(tn.String()) {
				st, _, _, _ := scAndVMInteropTypeFromExpr(tn, false)
				expectedLen := -1
				switch st {
				case smartcontract.Hash160Type:
					expectedLen = 20
				case smartcontract.Hash256Type:
					expectedLen = 32
				}
				if expectedLen != -1 && expectedLen != len(n.Elts) {
					c.prog.Err = fmt.Errorf("%s type must have size %d", tn.Obj().Name(), expectedLen)
					return nil
				}
			}
			ln := len(n.Elts)
			// ByteArrays needs a different approach than normal arrays.
			if isByteSlice(typ) {
				c.convertByteArray(n.Elts)
				return nil
			}
			for i := ln - 1; i >= 0; i-- {
				ast.Walk(c, n.Elts[i])
			}
			emit.Int(c.prog.BinWriter, int64(ln))
			emit.Opcodes(c.prog.BinWriter, opcode.PACK)
		}

		return nil

	case *ast.BinaryExpr:
		c.emitBinaryExpr(n, false, false, 0)
		return nil

	case *ast.CallExpr:
		var (
			f         *funcScope
			ok        bool
			name      string
			numArgs   = len(n.Args)
			isBuiltin bool
			isFunc    bool
			isLiteral bool
		)

		switch fun := n.Fun.(type) {
		case *ast.Ident:
			f, ok = c.getFuncFromIdent(fun)
			isBuiltin = isGoBuiltin(fun.Name)
			if !ok && !isBuiltin {
				name = fun.Name
			}
			// distinguish lambda invocations from type conversions
			if fun.Obj != nil && fun.Obj.Kind == ast.Var {
				isFunc = true
			}
			if ok && canInline(f.pkg.Path(), f.decl.Name.Name, false) {
				c.inlineCall(f, n)
				return nil
			}
		case *ast.SelectorExpr:
			name, isMethod := c.getFuncNameFromSelector(fun)

			f, ok = c.funcs[name]
			if ok {
				f.selector = fun.X
				isBuiltin = isPotentialCustomBuiltin(f, n)
				if canInline(f.pkg.Path(), f.decl.Name.Name, isBuiltin) {
					c.inlineCall(f, n)
					return nil
				}
			} else {
				typ := c.typeOf(fun)
				ast.Walk(c, n.Args[0])
				c.emitExplicitConvert(c.typeOf(n.Args[0]), typ)
				return nil
			}
			if isMethod {
				// If this is a method call we need to walk the AST to load the struct locally.
				// Otherwise, this is a function call from an imported package and we can call it
				// directly.
				ast.Walk(c, fun.X)
				// Don't forget to add 1 extra argument when it's a method.
				numArgs++
			}
		case *ast.ArrayType:
			// For now we will assume that there are only byte slice conversions.
			// E.g. []byte("foobar") or []byte(scriptHash).
			ast.Walk(c, n.Args[0])
			c.emitConvert(stackitem.BufferT)
			return nil
		case *ast.InterfaceType:
			// It's a type conversion into some interface. Programmer is responsible
			// for the conversion to be appropriate, just load the arg.
			ast.Walk(c, n.Args[0])
			return nil
		case *ast.FuncLit:
			isLiteral = true
		}

		c.saveSequencePoint(n)

		args := transformArgs(f, n.Fun, isBuiltin, n.Args)

		// Handle the arguments
		for _, arg := range args {
			ast.Walk(c, arg)
			typ := c.typeOf(arg)
			_, ok := typ.Underlying().(*types.Struct)
			if ok && !isInteropPath(typ.String()) {
				// To clone struct fields we create a new array and append struct to it.
				// This way even non-pointer struct fields will be copied.
				emit.Opcodes(c.prog.BinWriter, opcode.NEWARRAY0,
					opcode.DUP, opcode.ROT, opcode.APPEND,
					opcode.POPITEM)
			}
		}
		// Do not swap for builtin functions.
		if !isBuiltin && (f != nil && !isSyscall(f)) {
			typ, ok := c.typeOf(n.Fun).(*types.Signature)
			if ok && typ.Variadic() && !n.Ellipsis.IsValid() {
				// pack variadic args into an array only if last argument is not of form `...`
				varSize := c.packVarArgs(n, typ)
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
			// Function was not found, thus it can only be an invocation of a func-typed variable or type conversion.
			// We care only about string conversions because all others are effectively no-op in NeoVM.
			// E.g. one cannot write `bool(int(a))`, only `int32(int(a))`.
			if isString(c.typeOf(n.Fun)) {
				c.emitConvert(stackitem.ByteArrayT)
			} else if isFunc {
				c.emitLoadVar("", name)
				emit.Opcodes(c.prog.BinWriter, opcode.CALLA)
			}
		case isLiteral:
			ast.Walk(c, n.Fun)
			emit.Opcodes(c.prog.BinWriter, opcode.CALLA)
		case isSyscall(f):
			c.convertSyscall(f, n)
		default:
			emit.Call(c.prog.BinWriter, opcode.CALLL, f.label)
		}

		if c.scope != nil && c.scope.voidCalls[n] {
			var sz int
			if f != nil {
				sz = f.decl.Type.Results.NumFields()
			} else if !isBuiltin {
				// lambda invocation
				f := c.typeOf(n.Fun).Underlying().(*types.Signature)
				sz = f.Results().Len()
			}
			for i := 0; i < sz; i++ {
				emit.Opcodes(c.prog.BinWriter, opcode.DROP)
			}
		}

		return nil

	case *ast.DeferStmt:
		catch := c.newLabel()
		finally := c.newLabel()
		param := make([]byte, 8)
		binary.LittleEndian.PutUint16(param[0:], catch)
		binary.LittleEndian.PutUint16(param[4:], finally)
		emit.Instruction(c.prog.BinWriter, opcode.TRYL, param)
		index := c.scope.newLocal(fmt.Sprintf("defer@%d", n.Call.Pos()))
		emit.Opcodes(c.prog.BinWriter, opcode.PUSH1)
		c.emitStoreByIndex(varLocal, index)
		c.scope.deferStack = append(c.scope.deferStack, deferInfo{
			catchLabel:   catch,
			finallyLabel: finally,
			expr:         n.Call,
			localIndex:   index,
		})
		return nil

	case *ast.SelectorExpr:
		typ := c.typeOf(n.X)
		if c.isInvalidType(typ) {
			// This is a global variable from a package.
			pkgAlias := n.X.(*ast.Ident).Name
			name := c.getIdentName(pkgAlias, n.Sel.Name)
			if tv, ok := c.constMap[name]; ok {
				c.emitLoadConst(tv)
			} else {
				c.emitLoadVar(pkgAlias, n.Sel.Name)
			}
			return nil
		}
		strct, ok := c.getStruct(typ)
		if !ok {
			c.prog.Err = fmt.Errorf("selectors are supported only on structs")
			return nil
		}
		ast.Walk(c, n.X) // load the struct
		i := indexOfStruct(strct, n.Sel.Name)
		c.emitLoadField(i) // load the field
		return nil

	case *ast.UnaryExpr:
		if n.Op == token.AND {
			// We support only taking address from struct literals.
			// For identifiers we can't support "taking address" in a general way
			// because both struct and array are reference types.
			lit, ok := n.X.(*ast.CompositeLit)
			if ok {
				c.convertStruct(lit, true)
				return nil
			}
			c.prog.Err = fmt.Errorf("'&' can be used only with struct literals")
			return nil
		}

		ast.Walk(c, n.X)
		// From https://golang.org/ref/spec#Operators
		// there can be only following unary operators
		// "+" | "-" | "!" | "^" | "*" | "&" | "<-" .
		// of which last three are not used in SC
		switch n.Op {
		case token.ADD:
			// +10 == 10, no need to do anything in this case
		case token.SUB:
			emit.Opcodes(c.prog.BinWriter, opcode.NEGATE)
		case token.NOT:
			emit.Opcodes(c.prog.BinWriter, opcode.NOT)
		case token.XOR:
			emit.Opcodes(c.prog.BinWriter, opcode.INVERT)
		default:
			c.prog.Err = fmt.Errorf("invalid unary operator: %s", n.Op)
			return nil
		}
		return nil

	case *ast.IncDecStmt:
		ast.Walk(c, n.X)
		c.emitToken(n.Tok, c.typeOf(n.X))

		// For now, only identifiers are supported for (post) for stmts.
		// for i := 0; i < 10; i++ {}
		// Where the post stmt is ( i++ )
		if ident, ok := n.X.(*ast.Ident); ok {
			c.emitStoreVar("", ident.Name)
		}
		return nil

	case *ast.IndexExpr:
		// Walk the expression, this could be either an Ident or SelectorExpr.
		// This will load local whatever X is.
		ast.Walk(c, n.X)
		ast.Walk(c, n.Index)
		emit.Opcodes(c.prog.BinWriter, opcode.PICKITEM) // just pickitem here

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
		// For slices, we iterate through indices from 0 to len-1, storing array, len and index on stack.
		// For maps, we iterate through indices from 0 to len-1, storing map, keyarray, size and index on stack.
		_, isMap := c.typeOf(n.X).Underlying().(*types.Map)
		emit.Opcodes(c.prog.BinWriter, opcode.DUP)
		if isMap {
			emit.Opcodes(c.prog.BinWriter, opcode.KEYS, opcode.DUP)
		}
		emit.Opcodes(c.prog.BinWriter, opcode.SIZE, opcode.PUSH0)

		stackSize := 3 // slice, len(slice), index
		if isMap {
			stackSize++ // map, keys, len(keys), index in keys
		}
		c.pushStackLabel(label, stackSize)
		c.setLabel(start)

		emit.Opcodes(c.prog.BinWriter, opcode.OVER, opcode.OVER)
		emit.Jmp(c.prog.BinWriter, opcode.JMPLEL, end)

		var (
			haveKey   bool
			haveVal   bool
			keyIdent  *ast.Ident
			keyLoaded bool
			valIdent  *ast.Ident
		)
		if n.Key != nil {
			keyIdent, haveKey = n.Key.(*ast.Ident)
			if !haveKey {
				c.prog.Err = errors.New("only simple identifiers can be used for range loop keys (see #2870)")
				return nil
			}
			haveKey = (keyIdent.Name != "_")
		}
		if n.Value != nil {
			valIdent, haveVal = n.Value.(*ast.Ident)
			if !haveVal {
				c.prog.Err = errors.New("only simple identifiers can be used for range loop values (see #2870)")
				return nil
			}
			haveVal = (valIdent.Name != "_")
		}
		if haveKey {
			if isMap {
				c.rangeLoadKey()
				if haveVal {
					emit.Opcodes(c.prog.BinWriter, opcode.DUP)
					keyLoaded = true
				}
			} else {
				emit.Opcodes(c.prog.BinWriter, opcode.DUP)
			}
			if n.Tok == token.DEFINE {
				c.scope.newLocal(keyIdent.Name)
			}
			c.emitStoreVar("", keyIdent.Name)
		}
		if haveVal {
			if !isMap || !keyLoaded {
				c.rangeLoadKey()
			}
			if isMap {
				// we have loaded only key from key array, now load value
				emit.Int(c.prog.BinWriter, 4)
				emit.Opcodes(c.prog.BinWriter,
					opcode.PICK, // load map itself (+1 because key was pushed)
					opcode.SWAP, // key should be on top
					opcode.PICKITEM)
			}
			if n.Tok == token.DEFINE {
				c.scope.newLocal(valIdent.Name)
			}
			c.emitStoreVar("", valIdent.Name)
		}

		ast.Walk(c, n.Body)

		c.setLabel(post)

		emit.Opcodes(c.prog.BinWriter, opcode.INC)
		emit.Jmp(c.prog.BinWriter, opcode.JMPL, start)

		c.setLabel(end)
		c.dropStackLabel()

		c.currentFor = lastFor
		c.currentSwitch = lastSwitch

		return nil

	// We don't really care about assertions for the core logic.
	// The only thing we need is to please the compiler type checking.
	// For this to work properly, we only need to walk the expression
	// which is not the assertion type.
	case *ast.TypeAssertExpr:
		ast.Walk(c, n.X)
		if c.isCallExprSyscall(n.X) {
			return nil
		}

		goTyp := c.typeOf(n.Type)
		if canConvert(goTyp.String()) {
			typ := toNeoType(goTyp)
			c.emitConvert(typ)
		}
		return nil
	}
	return c
}

func checkTypeAssertWithOK(n ast.Node) error {
	if t, ok := n.(*ast.TypeAssertExpr); ok &&
		t.Type != nil { // not a type switch
		return ErrUnsupportedTypeAssertion
	}
	return nil
}

// packVarArgs packs variadic arguments into an array
// and returns the amount of arguments packed.
func (c *codegen) packVarArgs(n *ast.CallExpr, typ *types.Signature) int {
	varSize := len(n.Args) - typ.Params().Len() + 1
	c.emitReverse(varSize)
	emit.Int(c.prog.BinWriter, int64(varSize))
	emit.Opcodes(c.prog.BinWriter, opcode.PACK)
	return varSize
}

func (c *codegen) isCallExprSyscall(e ast.Expr) bool {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name, _ := c.getFuncNameFromSelector(sel)
	f, ok := c.funcs[name]
	return ok && isSyscall(f)
}

// processDefers emits code for `defer` statements.
// TRY-related opcodes handle exception as follows:
//  1. CATCH block is executed only if exception has occurred.
//  2. FINALLY block is always executed, but after catch block.
//
// Go `defer` statements are a bit different:
//  1. `defer` is always executed irregardless of whether an exception has occurred.
//  2. `recover` can or can not handle a possible exception.
//
// Thus, we use the following approach:
//  1. Throwed exception is saved in a static field X, static fields Y and it is set to true.
//  2. For each defer local there is a dedicated local variable which is set to 1 if `defer` statement
//     is encountered during an actual execution.
//  3. CATCH and FINALLY blocks are the same, and both contain the same CALLs.
//  4. Right before the CATCH block, check a variable from (2). If it is null, jump to the end of CATCH+FINALLY block.
//  5. In CATCH block we set Y to true and emit default return values if it is the last defer.
//  6. Execute FINALLY block only if Y is false.
func (c *codegen) processDefers() {
	for i := len(c.scope.deferStack) - 1; i >= 0; i-- {
		stmt := c.scope.deferStack[i]
		after := c.newLabel()

		c.emitLoadByIndex(varLocal, c.scope.deferStack[i].localIndex)
		emit.Opcodes(c.prog.BinWriter, opcode.ISNULL)
		emit.Jmp(c.prog.BinWriter, opcode.JMPIFL, after)

		emit.Jmp(c.prog.BinWriter, opcode.ENDTRYL, after)

		c.setLabel(stmt.catchLabel)
		c.emitStoreByIndex(varGlobal, c.exceptionIndex)
		emit.Int(c.prog.BinWriter, 1)

		finalIndex := c.getVarIndex("", finallyVarName).index
		c.emitStoreByIndex(varLocal, finalIndex)
		ast.Walk(c, stmt.expr)
		if i == 0 {
			results := c.scope.decl.Type.Results
			if results.NumFields() != 0 {
				// After panic, default values must be returns, except for named returns,
				// which we don't support here for now.
				for i := len(results.List) - 1; i >= 0; i-- {
					c.emitDefault(c.typeOf(results.List[i].Type))
				}
			}
		}
		emit.Jmp(c.prog.BinWriter, opcode.ENDTRYL, after)

		c.setLabel(stmt.finallyLabel)
		before := c.newLabel()
		c.emitLoadByIndex(varLocal, finalIndex)
		emit.Jmp(c.prog.BinWriter, opcode.JMPIFL, before)
		ast.Walk(c, stmt.expr)
		c.setLabel(before)
		emit.Int(c.prog.BinWriter, 0)
		c.emitStoreByIndex(varLocal, finalIndex)
		emit.Opcodes(c.prog.BinWriter, opcode.ENDFINALLY)
		c.setLabel(after)
	}
}

// emitExplicitConvert handles `someType(someValue)` conversions between string/[]byte.
// Rules for conversion:
//  1. interop.* types are converted to ByteArray if not already.
//  2. Otherwise, convert between ByteArray/Buffer.
//  3. Rules for types which are not string/[]byte should already
//     be enforced by go parser.
func (c *codegen) emitExplicitConvert(from, to types.Type) {
	if isInteropPath(to.String()) {
		if isByteSlice(from) && !isString(from) {
			c.emitConvert(stackitem.ByteArrayT)
		}
	} else if isByteSlice(to) && !isByteSlice(from) {
		c.emitConvert(stackitem.BufferT)
	} else if isString(to) && !isString(from) {
		c.emitConvert(stackitem.ByteArrayT)
	}
}

func (c *codegen) isInvalidType(typ types.Type) bool {
	tb, ok := typ.(*types.Basic)
	return typ == nil || ok && tb.Kind() == types.Invalid
}

func (c *codegen) rangeLoadKey() {
	emit.Int(c.prog.BinWriter, 2)
	emit.Opcodes(c.prog.BinWriter,
		opcode.PICK, // load keys
		opcode.OVER, // load index in key array
		opcode.PICKITEM)
}

func isFallthroughStmt(c ast.Node) bool {
	s, ok := c.(*ast.BranchStmt)
	return ok && s.Tok == token.FALLTHROUGH
}

func (c *codegen) getCompareWithNilArg(n *ast.BinaryExpr) ast.Expr {
	if isExprNil(n.X) {
		return n.Y
	} else if isExprNil(n.Y) {
		return n.X
	}
	return nil
}

func (c *codegen) emitJumpOnCondition(cond bool, jmpLabel uint16) {
	if cond {
		emit.Jmp(c.prog.BinWriter, opcode.JMPIFL, jmpLabel)
	} else {
		emit.Jmp(c.prog.BinWriter, opcode.JMPIFNOTL, jmpLabel)
	}
}

// emitBoolExpr emits boolean expression. If needJump is true and expression evaluates to `cond`,
// jump to jmpLabel is performed and no item is left on stack.
func (c *codegen) emitBoolExpr(n ast.Expr, needJump bool, cond bool, jmpLabel uint16) {
	if be, ok := n.(*ast.BinaryExpr); ok {
		c.emitBinaryExpr(be, needJump, cond, jmpLabel)
	} else {
		ast.Walk(c, n)
		if needJump {
			c.emitJumpOnCondition(cond, jmpLabel)
		}
	}
}

// emitBinaryExpr emits binary expression. If needJump is true and expression evaluates to `cond`,
// jump to jmpLabel is performed and no item is left on stack.
func (c *codegen) emitBinaryExpr(n *ast.BinaryExpr, needJump bool, cond bool, jmpLabel uint16) {
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
		if needJump && isBool(tinfo.Type) {
			c.emitJumpOnCondition(cond, jmpLabel)
		}
		return
	} else if arg := c.getCompareWithNilArg(n); arg != nil {
		ast.Walk(c, arg)
		emit.Opcodes(c.prog.BinWriter, opcode.ISNULL)
		if needJump {
			c.emitJumpOnCondition(cond == (n.Op == token.EQL), jmpLabel)
		} else if n.Op == token.NEQ {
			emit.Opcodes(c.prog.BinWriter, opcode.NOT)
		}
		return
	}

	switch n.Op {
	case token.LAND, token.LOR:
		end := c.newLabel()

		// true || .. == true, false && .. == false
		condShort := n.Op == token.LOR
		if needJump {
			l := end
			if cond == condShort {
				l = jmpLabel
			}
			c.emitBoolExpr(n.X, true, condShort, l)
			c.emitBoolExpr(n.Y, true, cond, jmpLabel)
		} else {
			push := c.newLabel()
			c.emitBoolExpr(n.X, true, condShort, push)
			c.emitBoolExpr(n.Y, false, false, 0)
			emit.Jmp(c.prog.BinWriter, opcode.JMPL, end)
			c.setLabel(push)
			emit.Bool(c.prog.BinWriter, condShort)
		}
		c.setLabel(end)

	default:
		ast.Walk(c, n.X)
		ast.Walk(c, n.Y)
		typ := c.typeOf(n.X)
		if !needJump {
			c.emitToken(n.Op, typ)
			return
		}
		op, ok := getJumpForToken(n.Op, typ)
		if !ok {
			c.emitToken(n.Op, typ)
			c.emitJumpOnCondition(cond, jmpLabel)
			return
		}
		if !cond {
			op = negateJmp(op)
		}
		emit.Jmp(c.prog.BinWriter, op, jmpLabel)
	}
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
			emit.Opcodes(c.prog.BinWriter, opcode.DROP)
		}
		return
	}

	emit.Int(c.prog.BinWriter, int64(n))
	emit.Opcodes(c.prog.BinWriter, opcode.PACK, opcode.DROP)
}

// emitReverse reverses top num items of the stack.
func (c *codegen) emitReverse(num int) {
	switch num {
	case 0, 1:
	case 2:
		emit.Opcodes(c.prog.BinWriter, opcode.SWAP)
	case 3:
		emit.Opcodes(c.prog.BinWriter, opcode.REVERSE3)
	case 4:
		emit.Opcodes(c.prog.BinWriter, opcode.REVERSE4)
	default:
		emit.Int(c.prog.BinWriter, int64(num))
		emit.Opcodes(c.prog.BinWriter, opcode.REVERSEN)
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

// For `&&` and `||` it return an opcode which jumps only if result is known:
// false && .. == false, true || .. = true.
func getJumpForToken(tok token.Token, typ types.Type) (opcode.Opcode, bool) {
	switch tok {
	case token.GTR:
		return opcode.JMPGTL, true
	case token.GEQ:
		return opcode.JMPGEL, true
	case token.LSS:
		return opcode.JMPLTL, true
	case token.LEQ:
		return opcode.JMPLEL, true
	case token.EQL, token.NEQ:
		if isNumber(typ) {
			if tok == token.EQL {
				return opcode.JMPEQL, true
			}
			return opcode.JMPNEL, true
		}
	}
	return 0, false
}

func (c *codegen) getCallToken(hash util.Uint160, method string, pcount int, hasReturn bool, flag callflag.CallFlag) (uint16, error) {
	needed := nef.MethodToken{
		Hash:       hash,
		Method:     method,
		ParamCount: uint16(pcount),
		HasReturn:  hasReturn,
		CallFlag:   flag,
	}
	for i := range c.callTokens {
		if c.callTokens[i] == needed {
			return uint16(i), nil
		}
	}
	if len(c.callTokens) == math.MaxUint16 {
		return 0, errors.New("call token overflow")
	}
	c.callTokens = append(c.callTokens, needed)
	return uint16(len(c.callTokens) - 1), nil
}

func (c *codegen) convertSyscall(f *funcScope, expr *ast.CallExpr) {
	var callArgs = expr.Args[1:]

	if strings.HasPrefix(f.name, "CallWithToken") {
		callArgs = expr.Args[3:]
	}
	for _, arg := range callArgs {
		ast.Walk(c, arg)
	}
	tv := c.typeAndValueOf(expr.Args[0])
	if tv.Value == nil || !isString(tv.Type) {
		c.prog.Err = fmt.Errorf("bad intrinsic argument")
		return
	}
	arg0Str := constant.StringVal(tv.Value)

	if strings.HasPrefix(f.name, "Syscall") {
		c.emitReverse(len(callArgs))
		emit.Syscall(c.prog.BinWriter, arg0Str)
	} else if strings.HasPrefix(f.name, "CallWithToken") {
		var hasRet = !strings.HasSuffix(f.name, "NoRet")

		c.emitReverse(len(callArgs))

		hash, err := util.Uint160DecodeBytesBE([]byte(arg0Str))
		if err != nil {
			c.prog.Err = fmt.Errorf("bad callt hash: %w", err)
			return
		}

		tv = c.typeAndValueOf(expr.Args[1])
		if tv.Value == nil || !isString(tv.Type) {
			c.prog.Err = errors.New("bad callt method")
			return
		}
		method := constant.StringVal(tv.Value)

		tv = c.typeAndValueOf(expr.Args[2])
		if tv.Value == nil || !isNumber(tv.Type) {
			c.prog.Err = errors.New("bad callt call flags")
			return
		}
		flag, ok := constant.Uint64Val(tv.Value)
		if !ok || flag > 255 {
			c.prog.Err = errors.New("invalid callt flag")
			return
		}

		c.appendInvokedContract(hash, method, flag)

		tokNum, err := c.getCallToken(hash, method, len(callArgs), hasRet, callflag.CallFlag(flag))
		if err != nil {
			c.prog.Err = err
			return
		}
		tokBuf := make([]byte, 2)
		binary.LittleEndian.PutUint16(tokBuf, tokNum)
		emit.Instruction(c.prog.BinWriter, opcode.CALLT, tokBuf)
	} else {
		op, err := opcode.FromString(arg0Str)
		if err != nil {
			c.prog.Err = fmt.Errorf("invalid opcode: %s", op)
			return
		}
		emit.Opcodes(c.prog.BinWriter, op)
	}
}

// emitSliceHelper emits 3 items on stack: slice, its first index, and its size.
func (c *codegen) emitSliceHelper(e ast.Expr) {
	if !isByteSlice(c.typeOf(e)) {
		c.prog.Err = fmt.Errorf("copy is supported only for byte-slices")
		return
	}
	var hasLowIndex bool
	switch src := e.(type) {
	case *ast.SliceExpr:
		ast.Walk(c, src.X)
		if src.High != nil {
			ast.Walk(c, src.High)
		} else {
			emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.SIZE)
		}
		if src.Low != nil {
			ast.Walk(c, src.Low)
			hasLowIndex = true
		} else {
			emit.Int(c.prog.BinWriter, 0)
		}
	default:
		ast.Walk(c, src)
		emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.SIZE)
		emit.Int(c.prog.BinWriter, 0)
	}
	if !hasLowIndex {
		emit.Opcodes(c.prog.BinWriter, opcode.SWAP)
	} else {
		emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.ROT, opcode.SWAP, opcode.SUB)
	}
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
	case "copy":
		// stack for MEMCPY is: dst, dst_index, src, src_index, count
		c.emitSliceHelper(expr.Args[0])
		c.emitSliceHelper(expr.Args[1])
		emit.Int(c.prog.BinWriter, 3)
		emit.Opcodes(c.prog.BinWriter, opcode.ROLL, opcode.MIN)
		if !c.scope.voidCalls[expr] {
			// insert top item to the bottom of MEMCPY args, so that it is left on stack
			emit.Opcodes(c.prog.BinWriter, opcode.DUP)
			emit.Int(c.prog.BinWriter, 6)
			emit.Opcodes(c.prog.BinWriter, opcode.REVERSEN)
			emit.Int(c.prog.BinWriter, 5)
			emit.Opcodes(c.prog.BinWriter, opcode.REVERSEN)
		}
		emit.Opcodes(c.prog.BinWriter, opcode.MEMCPY)
	case "make":
		typ := c.typeOf(expr.Args[0])
		switch {
		case isMap(typ):
			emit.Opcodes(c.prog.BinWriter, opcode.NEWMAP)
		default:
			if len(expr.Args) == 3 {
				c.prog.Err = fmt.Errorf("`make()` with a capacity argument is not supported")
				return
			}
			ast.Walk(c, expr.Args[1])
			if isByteSlice(typ) {
				emit.Opcodes(c.prog.BinWriter, opcode.NEWBUFFER)
			} else {
				neoT := toNeoType(typ.(*types.Slice).Elem())
				emit.Instruction(c.prog.BinWriter, opcode.NEWARRAYT, []byte{byte(neoT)})
			}
		}
	case "len":
		emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.ISNULL)
		emit.Instruction(c.prog.BinWriter, opcode.JMPIF, []byte{2 + 1 + 2})
		emit.Opcodes(c.prog.BinWriter, opcode.SIZE)
		emit.Instruction(c.prog.BinWriter, opcode.JMP, []byte{2 + 1 + 1})
		emit.Opcodes(c.prog.BinWriter, opcode.DROP, opcode.PUSH0)
	case "append":
		arg := expr.Args[0]
		typ := c.typeOf(arg)
		ast.Walk(c, arg)
		emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.ISNULL)
		if isByteSlice(typ) {
			emit.Instruction(c.prog.BinWriter, opcode.JMPIFNOT, []byte{2 + 3})
			emit.Opcodes(c.prog.BinWriter, opcode.DROP, opcode.PUSHDATA1, 0)
			if expr.Ellipsis.IsValid() {
				ast.Walk(c, expr.Args[1])
			} else {
				c.convertByteArray(expr.Args[1:])
			}
			emit.Opcodes(c.prog.BinWriter, opcode.CAT)
		} else {
			emit.Instruction(c.prog.BinWriter, opcode.JMPIFNOT, []byte{2 + 2})
			emit.Opcodes(c.prog.BinWriter, opcode.DROP, opcode.NEWARRAY0)
			if expr.Ellipsis.IsValid() {
				ast.Walk(c, expr.Args[1])                    // x y
				emit.Opcodes(c.prog.BinWriter, opcode.PUSH0) // x y cnt=0
				start := c.newLabel()
				c.setLabel(start)
				emit.Opcodes(c.prog.BinWriter, opcode.PUSH2, opcode.PICK) // x y cnt x
				emit.Opcodes(c.prog.BinWriter, opcode.PUSH2, opcode.PICK) // x y cnt x y
				emit.Opcodes(c.prog.BinWriter, opcode.DUP, opcode.SIZE)   // x y cnt x y len(y)
				emit.Opcodes(c.prog.BinWriter, opcode.PUSH3, opcode.PICK) // x y cnt x y len(y) cnt
				after := c.newLabel()
				emit.Jmp(c.prog.BinWriter, opcode.JMPEQL, after)          // x y cnt x y
				emit.Opcodes(c.prog.BinWriter, opcode.PUSH2, opcode.PICK, // x y cnt x y cnt
					opcode.PICKITEM, // x y cnt x y[cnt]
					opcode.APPEND,   // x=append(x, y[cnt]) y cnt
					opcode.INC)      // x y cnt+1
				emit.Jmp(c.prog.BinWriter, opcode.JMPL, start)
				c.setLabel(after)
				for i := 0; i < 4; i++ { // leave x on stack
					emit.Opcodes(c.prog.BinWriter, opcode.DROP)
				}
			} else {
				for _, e := range expr.Args[1:] {
					emit.Opcodes(c.prog.BinWriter, opcode.DUP)
					ast.Walk(c, e)
					emit.Opcodes(c.prog.BinWriter, opcode.APPEND)
				}
			}
		}
	case "panic":
		emit.Opcodes(c.prog.BinWriter, opcode.THROW)
	case "recover":
		if !c.scope.voidCalls[expr] {
			c.emitLoadByIndex(varGlobal, c.exceptionIndex)
		}
		emit.Opcodes(c.prog.BinWriter, opcode.PUSHNULL)
		c.emitStoreByIndex(varGlobal, c.exceptionIndex)
	case "delete":
		emit.Opcodes(c.prog.BinWriter, opcode.REMOVE)
	case "ToHash160":
		// We can be sure that this is an ast.BasicLit just containing a simple
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
//  1. With ToHash160 in case if it behaves like builtin,
//     parameter conversion is happening at compile-time so there is no need to
//     push parameters on stack and perform an actual call
//  2. With panic, the generated code depends on the fact if an argument was nil or a string;
//     so, it should be handled accordingly.
func transformArgs(fs *funcScope, fun ast.Expr, isBuiltin bool, args []ast.Expr) []ast.Expr {
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		if isBuiltin && f.Sel.Name == "ToHash160" {
			return args[1:]
		}
		if fs != nil && isSyscall(fs) {
			return nil
		}
	case *ast.Ident:
		switch f.Name {
		case "make", "copy", "append":
			return nil
		}
	}

	return args
}

// emitConvert converts the top stack item to the specified type.
func (c *codegen) emitConvert(typ stackitem.Type) {
	if typ == stackitem.BooleanT {
		// DUP + ISTYPE + JMPIF costs 3 already with CONVERT of a cost 8192.
		// NOT+NOT at the same time costs 4 and always works (and is shorter).
		emit.Opcodes(c.prog.BinWriter, opcode.NOT, opcode.NOT)
	} else {
		emit.Opcodes(c.prog.BinWriter, opcode.DUP)
		emit.Instruction(c.prog.BinWriter, opcode.ISTYPE, []byte{byte(typ)})
		emit.Instruction(c.prog.BinWriter, opcode.JMPIF, []byte{2 + 2}) // After CONVERT.
		emit.Instruction(c.prog.BinWriter, opcode.CONVERT, []byte{byte(typ)})
	}
}

func (c *codegen) convertByteArray(elems []ast.Expr) {
	buf := make([]byte, len(elems))
	varIndices := []int{}
	for i := 0; i < len(elems); i++ {
		t := c.typeAndValueOf(elems[i])
		if t.Value != nil {
			val, _ := constant.Int64Val(t.Value)
			buf[i] = byte(val)
		} else {
			varIndices = append(varIndices, i)
		}
	}
	emit.Bytes(c.prog.BinWriter, buf)
	c.emitConvert(stackitem.BufferT)
	for _, i := range varIndices {
		emit.Opcodes(c.prog.BinWriter, opcode.DUP)
		emit.Int(c.prog.BinWriter, int64(i))
		ast.Walk(c, elems[i])
		emit.Opcodes(c.prog.BinWriter, opcode.SETITEM)
	}
}

func (c *codegen) convertMap(lit *ast.CompositeLit) {
	l := len(lit.Elts)
	for i := l - 1; i >= 0; i-- {
		elem := lit.Elts[i].(*ast.KeyValueExpr)
		ast.Walk(c, elem.Value)
		ast.Walk(c, elem.Key)
	}
	emit.Int(c.prog.BinWriter, int64(l))
	emit.Opcodes(c.prog.BinWriter, opcode.PACKMAP)
}

func (c *codegen) getStruct(typ types.Type) (*types.Struct, bool) {
	switch t := typ.Underlying().(type) {
	case *types.Struct:
		return t, true
	case *types.Pointer:
		strct, ok := t.Elem().Underlying().(*types.Struct)
		return strct, ok
	default:
		return nil, false
	}
}

func (c *codegen) convertStruct(lit *ast.CompositeLit, ptr bool) {
	// Create a new structScope to initialize and store
	// the positions of its variables.
	strct, ok := c.typeOf(lit).Underlying().(*types.Struct)
	if !ok {
		c.prog.Err = fmt.Errorf("the given literal is not of type struct: %v", lit)
		return
	}

	keyedLit := len(lit.Elts) > 0
	if keyedLit {
		_, ok := lit.Elts[0].(*ast.KeyValueExpr)
		keyedLit = keyedLit && ok
	}
	// We need to locally store all the fields, even if they are not initialized.
	// We will initialize all fields to their "zero" value.
	for i := strct.NumFields() - 1; i >= 0; i-- {
		sField := strct.Field(i)
		var initialized bool

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
	}
	emit.Int(c.prog.BinWriter, int64(strct.NumFields()))
	if ptr {
		emit.Opcodes(c.prog.BinWriter, opcode.PACK)
	} else {
		emit.Opcodes(c.prog.BinWriter, opcode.PACKSTRUCT)
	}
}

func (c *codegen) emitToken(tok token.Token, typ types.Type) {
	op, err := convertToken(tok, typ)
	if err != nil {
		c.prog.Err = err
		return
	}
	emit.Opcodes(c.prog.BinWriter, op)
}

func convertToken(tok token.Token, typ types.Type) (opcode.Opcode, error) {
	switch tok {
	case token.ADD_ASSIGN, token.ADD:
		// VM has separate opcodes for number and string concatenation
		if isString(typ) {
			return opcode.CAT, nil
		}
		return opcode.ADD, nil
	case token.SUB_ASSIGN:
		return opcode.SUB, nil
	case token.MUL_ASSIGN:
		return opcode.MUL, nil
	case token.QUO_ASSIGN:
		return opcode.DIV, nil
	case token.REM_ASSIGN:
		return opcode.MOD, nil
	case token.SUB:
		return opcode.SUB, nil
	case token.MUL:
		return opcode.MUL, nil
	case token.QUO:
		return opcode.DIV, nil
	case token.REM:
		return opcode.MOD, nil
	case token.LSS:
		return opcode.LT, nil
	case token.LEQ:
		return opcode.LE, nil
	case token.GTR:
		return opcode.GT, nil
	case token.GEQ:
		return opcode.GE, nil
	case token.EQL:
		// VM has separate opcodes for number and string equality
		if isNumber(typ) {
			return opcode.NUMEQUAL, nil
		}
		return opcode.EQUAL, nil
	case token.NEQ:
		// VM has separate opcodes for number and string equality
		if isNumber(typ) {
			return opcode.NUMNOTEQUAL, nil
		}
		return opcode.NOTEQUAL, nil
	case token.DEC:
		return opcode.DEC, nil
	case token.INC:
		return opcode.INC, nil
	case token.NOT:
		return opcode.NOT, nil
	case token.AND:
		return opcode.AND, nil
	case token.OR:
		return opcode.OR, nil
	case token.SHL:
		return opcode.SHL, nil
	case token.SHR:
		return opcode.SHR, nil
	case token.XOR:
		return opcode.XOR, nil
	default:
		return 0, fmt.Errorf("compiler could not convert token: %s", tok)
	}
}

func (c *codegen) newFunc(decl *ast.FuncDecl) *funcScope {
	f := c.newFuncScope(decl, c.newLabel())
	c.funcs[c.getFuncNameFromDecl("", decl)] = f
	return f
}

func (c *codegen) getFuncFromIdent(fun *ast.Ident) (*funcScope, bool) {
	var pkgName string
	if len(c.pkgInfoInline) != 0 {
		pkgName = c.pkgInfoInline[len(c.pkgInfoInline)-1].PkgPath
	}

	f, ok := c.funcs[c.getIdentName(pkgName, fun.Name)]
	return f, ok
}

// getFuncNameFromSelector returns fully-qualified function name from the selector expression.
// Second return value is true iff this was a method call, not foreign package call.
func (c *codegen) getFuncNameFromSelector(e *ast.SelectorExpr) (string, bool) {
	if c.typeInfo.Selections[e] != nil {
		typ := c.typeInfo.Types[e.X].Type.String()
		name := c.getIdentName(typ, e.Sel.Name)
		if name[0] == '*' {
			name = name[1:]
		}
		return name, true
	}

	ident := e.X.(*ast.Ident)
	return c.getIdentName(ident.Name, e.Sel.Name), false
}

func (c *codegen) newLambda(u uint16, lit *ast.FuncLit) {
	name := fmt.Sprintf("lambda@%d", u)
	f := c.newFuncScope(&ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: lit.Type,
		Body: lit.Body,
	}, u)
	c.lambda[c.getFuncNameFromDecl("", f.decl)] = f
}

func (c *codegen) compile(info *buildInfo, pkg *packages.Package) error {
	c.mainPkg = pkg
	c.analyzePkgOrder()
	if c.prog.Err != nil {
		return c.prog.Err
	}
	c.fillDocumentInfo()
	funUsage := c.analyzeFuncAndGlobalVarUsage()
	if c.prog.Err != nil {
		return c.prog.Err
	}

	// Bring all imported functions into scope.
	c.ForEachFile(c.resolveFuncDecls)

	hasDeploy := c.traverseGlobals()

	if hasDeploy {
		deployOffset := c.prog.Len()
		emit.Instruction(c.prog.BinWriter, opcode.INITSLOT, []byte{0, 2})
		locCount := c.convertDeployFuncs()
		c.reverseOffsetMap[deployOffset] = nameWithLocals{
			name:  "_deploy",
			count: locCount,
		}
		c.deployEndOffset = c.prog.Len()
		emit.Opcodes(c.prog.BinWriter, opcode.RET)
	}

	// sort map keys to generate code deterministically.
	keys := make([]*types.Package, 0, len(info.program))
	for _, p := range info.program {
		keys = append(keys, p.Types)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Path() < keys[j].Path() })

	// Generate the code for the program.
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		for _, decl := range f.Decls {
			switch n := decl.(type) {
			case *ast.FuncDecl:
				// Don't convert the function if it's not used. This will save a lot
				// of bytecode space.
				pkgPath := ""
				if pkg != c.mainPkg.Types { // not a main package
					pkgPath = pkg.Path()
				}
				name := c.getFuncNameFromDecl(pkgPath, n)
				if !isInitFunc(n) && !isDeployFunc(n) && funUsage.funcUsed(name) &&
					(!isInteropPath(pkg.Path()) && !canInline(pkg.Path(), n.Name.Name, false)) {
					c.convertFuncDecl(f, n, pkg)
				}
			}
		}
	})

	return c.prog.Err
}

func newCodegen(info *buildInfo, pkg *packages.Package) *codegen {
	return &codegen{
		buildInfo:        info,
		prog:             io.NewBufBinWriter(),
		l:                []int{},
		funcs:            map[string]*funcScope{},
		lambda:           map[string]*funcScope{},
		reverseOffsetMap: map[int]nameWithLocals{},
		globals:          map[string]int{},
		labels:           map[labelWithType]uint16{},
		typeInfo:         pkg.TypesInfo,
		constMap:         map[string]types.TypeAndValue{},
		docIndex:         map[string]int{},
		packageCache:     map[string]*packages.Package{},

		initEndOffset:   -1,
		deployEndOffset: -1,

		emittedEvents:    make(map[string][]EmittedEventInfo),
		invokedContracts: make(map[util.Uint160][]string),
		sequencePoints:   make(map[string][]DebugSeqPoint),
	}
}

// codeGen compiles the program to bytecode.
func codeGen(info *buildInfo) (*nef.File, *DebugInfo, error) {
	if len(info.program) == 0 {
		return nil, nil, errors.New("empty package")
	}
	pkg := info.program[0]
	c := newCodegen(info, pkg)

	if err := c.compile(info, pkg); err != nil {
		return nil, nil, err
	}

	buf, err := c.writeJumps(c.prog.Bytes())
	if err != nil {
		return nil, nil, err
	}

	methods := bitfield.New(len(buf))
	di := c.emitDebugInfo(buf)
	for i := range di.Methods {
		methods.Set(int(di.Methods[i].Range.Start))
	}
	f, err := nef.NewFile(buf)
	if err != nil {
		return nil, nil, fmt.Errorf("error while trying to create .nef file: %w", err)
	}
	if c.callTokens != nil {
		f.Tokens = c.callTokens
	}
	f.Checksum = f.CalculateChecksum()
	return f, di, vm.IsScriptCorrect(buf, methods)
}

func (c *codegen) resolveFuncDecls(f *ast.File, pkg *types.Package) {
	for _, decl := range f.Decls {
		switch n := decl.(type) {
		case *ast.FuncDecl:
			fs := c.newFunc(n)
			fs.file = f
		}
	}
}

func (c *codegen) writeJumps(b []byte) ([]byte, error) {
	ctx := vm.NewContext(b)
	var nopOffsets []int
	for op, param, err := ctx.Next(); err == nil && ctx.IP() < len(b); op, param, err = ctx.Next() {
		switch op {
		case opcode.JMP, opcode.JMPIFNOT, opcode.JMPIF, opcode.CALL,
			opcode.JMPEQ, opcode.JMPNE,
			opcode.JMPGT, opcode.JMPGE, opcode.JMPLE, opcode.JMPLT:
		case opcode.TRYL:
			_, err := c.replaceLabelWithOffset(ctx.IP(), param)
			if err != nil {
				return nil, err
			}
			_, err = c.replaceLabelWithOffset(ctx.IP(), param[4:])
			if err != nil {
				return nil, err
			}
		case opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL,
			opcode.JMPEQL, opcode.JMPNEL,
			opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLEL, opcode.JMPLTL,
			opcode.CALLL, opcode.PUSHA, opcode.ENDTRYL:
			offset, err := c.replaceLabelWithOffset(ctx.IP(), param)
			if err != nil {
				return nil, err
			}
			if op != opcode.PUSHA && math.MinInt8 <= offset && offset <= math.MaxInt8 {
				if op == opcode.JMPL && offset == 5 {
					copy(b[ctx.IP():], []byte{byte(opcode.NOP), byte(opcode.NOP), byte(opcode.NOP), byte(opcode.NOP), byte(opcode.NOP)})
					nopOffsets = append(nopOffsets, ctx.IP(), ctx.IP()+1, ctx.IP()+2, ctx.IP()+3, ctx.IP()+4)
				} else {
					copy(b[ctx.IP():], []byte{byte(toShortForm(op)), byte(offset), byte(opcode.NOP), byte(opcode.NOP), byte(opcode.NOP)})
					nopOffsets = append(nopOffsets, ctx.IP()+2, ctx.IP()+3, ctx.IP()+4)
				}
			}
		case opcode.INITSLOT:
			nextIP := ctx.NextIP()
			info := c.reverseOffsetMap[ctx.IP()]
			if argCount := b[nextIP-1]; info.count == 0 && argCount == 0 {
				copy(b[ctx.IP():], []byte{byte(opcode.NOP), byte(opcode.NOP), byte(opcode.NOP)})
				nopOffsets = append(nopOffsets, ctx.IP(), ctx.IP()+1, ctx.IP()+2)
				continue
			}

			if info.count > 255 {
				return nil, fmt.Errorf("func '%s' has %d local variables (maximum is 255)", info.name, info.count)
			}
			b[nextIP-2] = byte(info.count)
		}
	}

	if c.deployEndOffset >= 0 {
		_, end := correctRange(uint16(c.initEndOffset+1), uint16(c.deployEndOffset), nopOffsets)
		c.deployEndOffset = int(end)
	}
	if c.initEndOffset > 0 {
		_, end := correctRange(0, uint16(c.initEndOffset), nopOffsets)
		c.initEndOffset = int(end)
	}

	// Correct function ip range.
	// Note: indices are sorted in increasing order.
	for _, f := range c.funcs {
		f.rng.Start, f.rng.End = correctRange(f.rng.Start, f.rng.End, nopOffsets)
	}
	return removeNOPs(b, nopOffsets, c.sequencePoints), nil
}

func correctRange(start, end uint16, offsets []int) (uint16, uint16) {
	newStart, newEnd := start, end
loop:
	for _, ind := range offsets {
		switch {
		case ind > int(end):
			break loop
		case ind < int(start):
			newStart--
			newEnd--
		case ind >= int(start):
			newEnd--
		}
	}
	return newStart, newEnd
}

func (c *codegen) replaceLabelWithOffset(ip int, arg []byte) (int, error) {
	index := binary.LittleEndian.Uint16(arg)
	if int(index) > len(c.l) {
		return 0, fmt.Errorf("unexpected label number: %d (max %d)", index, len(c.l))
	}
	if c.l[index] < 0 {
		return 0, fmt.Errorf("invalid label target: %d at %d", c.l[index], ip)
	}
	offset := c.l[index] - ip
	if offset > math.MaxInt32 || offset < math.MinInt32 {
		return 0, fmt.Errorf("label offset is too big at the instruction %d: %d (max %d, min %d)",
			ip, offset, math.MaxInt32, math.MinInt32)
	}
	binary.LittleEndian.PutUint32(arg, uint32(offset))
	return offset, nil
}

// removeNOPs converts b to a program where all long JMP*/CALL* specified by absolute offsets
// are replaced with their corresponding short counterparts. It panics if either b or offsets are invalid.
// This is done in 2 passes:
// 1. Alter jump offsets taking into account parts to be removed.
// 2. Perform actual removal of jump targets.
// 3. Reevaluate debug sequence points offsets.
// Note: after jump offsets altering, there can appear new candidates for conversion.
// These are ignored for now.
func removeNOPs(b []byte, nopOffsets []int, sequencePoints map[string][]DebugSeqPoint) []byte {
	if len(nopOffsets) == 0 {
		return b
	}
	for i := range nopOffsets {
		if b[nopOffsets[i]] != byte(opcode.NOP) {
			panic("NOP offset is invalid")
		}
	}

	// 1. Alter existing jump offsets.
	ctx := vm.NewContext(b)
	for op, _, err := ctx.Next(); err == nil && ctx.IP() < len(b); op, _, err = ctx.Next() {
		// we can't use arg returned by ctx.Next() because it is copied
		nextIP := ctx.NextIP()
		ip := ctx.IP()
		switch op {
		case opcode.JMP, opcode.JMPIFNOT, opcode.JMPIF, opcode.CALL,
			opcode.JMPEQ, opcode.JMPNE,
			opcode.JMPGT, opcode.JMPGE, opcode.JMPLE, opcode.JMPLT, opcode.ENDTRY:
			offset := int(int8(b[nextIP-1]))
			offset += calcOffsetCorrection(ip, ip+offset, nopOffsets)
			b[nextIP-1] = byte(offset)
		case opcode.TRY:
			catchOffset := int(int8(b[nextIP-2]))
			catchOffset += calcOffsetCorrection(ip, ip+catchOffset, nopOffsets)
			b[nextIP-1] = byte(catchOffset)
			finallyOffset := int(int8(b[nextIP-1]))
			finallyOffset += calcOffsetCorrection(ip, ip+finallyOffset, nopOffsets)
			b[nextIP-1] = byte(finallyOffset)
		case opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL,
			opcode.JMPEQL, opcode.JMPNEL,
			opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLEL, opcode.JMPLTL,
			opcode.CALLL, opcode.PUSHA, opcode.ENDTRYL:
			arg := b[nextIP-4:]
			offset := int(int32(binary.LittleEndian.Uint32(arg)))
			offset += calcOffsetCorrection(ip, ip+offset, nopOffsets)
			binary.LittleEndian.PutUint32(arg, uint32(offset))
		case opcode.TRYL:
			arg := b[nextIP-8:]
			catchOffset := int(int32(binary.LittleEndian.Uint32(arg)))
			catchOffset += calcOffsetCorrection(ip, ip+catchOffset, nopOffsets)
			binary.LittleEndian.PutUint32(arg, uint32(catchOffset))
			arg = b[nextIP-4:]
			finallyOffset := int(int32(binary.LittleEndian.Uint32(arg)))
			finallyOffset += calcOffsetCorrection(ip, ip+finallyOffset, nopOffsets)
			binary.LittleEndian.PutUint32(arg, uint32(finallyOffset))
		}
	}

	// 2. Convert instructions.
	copyOffset := 0
	l := len(nopOffsets)
	for i := 0; i < l; i++ {
		start := nopOffsets[i]
		end := len(b)
		if i != l-1 {
			end = nopOffsets[i+1]
		}
		copy(b[start-copyOffset:], b[start+1:end])
		copyOffset++
	}

	// 3. Reevaluate debug sequence points.
	for _, seqPoints := range sequencePoints {
		for i := range seqPoints {
			diff := 0
			for _, offset := range nopOffsets {
				if offset < seqPoints[i].Opcode {
					diff++
				}
			}
			seqPoints[i].Opcode -= diff
		}
	}

	return b[:len(b)-copyOffset]
}

func calcOffsetCorrection(ip, target int, offsets []int) int {
	cnt := 0
	start := sort.Search(len(offsets), func(i int) bool {
		return offsets[i] >= ip || offsets[i] >= target
	})
	for i := start; i < len(offsets) && (offsets[i] < target || offsets[i] <= ip); i++ {
		ind := offsets[i]
		if ip <= ind && ind < target || target <= ind && ind < ip {
			cnt++
		}
	}
	if ip < target {
		return -cnt
	}
	return cnt
}

func negateJmp(op opcode.Opcode) opcode.Opcode {
	switch op {
	case opcode.JMPIFL:
		return opcode.JMPIFNOTL
	case opcode.JMPIFNOTL:
		return opcode.JMPIFL
	case opcode.JMPEQL:
		return opcode.JMPNEL
	case opcode.JMPNEL:
		return opcode.JMPEQL
	case opcode.JMPGTL:
		return opcode.JMPLEL
	case opcode.JMPGEL:
		return opcode.JMPLTL
	case opcode.JMPLEL:
		return opcode.JMPGTL
	case opcode.JMPLTL:
		return opcode.JMPGEL
	default:
		panic(fmt.Errorf("invalid opcode in negateJmp: %s", op))
	}
}

func toShortForm(op opcode.Opcode) opcode.Opcode {
	switch op {
	case opcode.JMPL:
		return opcode.JMP
	case opcode.JMPIFL:
		return opcode.JMPIF
	case opcode.JMPIFNOTL:
		return opcode.JMPIFNOT
	case opcode.JMPEQL:
		return opcode.JMPEQ
	case opcode.JMPNEL:
		return opcode.JMPNE
	case opcode.JMPGTL:
		return opcode.JMPGT
	case opcode.JMPGEL:
		return opcode.JMPGE
	case opcode.JMPLEL:
		return opcode.JMPLE
	case opcode.JMPLTL:
		return opcode.JMPLT
	case opcode.CALLL:
		return opcode.CALL
	case opcode.ENDTRYL:
		return opcode.ENDTRY
	default:
		panic(fmt.Errorf("invalid opcode: %s", op))
	}
}
