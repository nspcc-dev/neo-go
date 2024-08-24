package compiler

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"golang.org/x/tools/go/packages"
)

// Various exported functions usage errors.
var (
	// ErrMissingExportedParamName is returned when exported contract method has unnamed parameter.
	ErrMissingExportedParamName = errors.New("exported method is not allowed to have unnamed parameter")
	// ErrInvalidExportedRetCount is returned when exported contract method has invalid return values count.
	ErrInvalidExportedRetCount = errors.New("exported method is not allowed to have more than one return value")
	// ErrGenericsUnsuppored is returned when generics-related tokens are encountered.
	ErrGenericsUnsuppored = errors.New("generics are currently unsupported, please, see the https://github.com/nspcc-dev/neo-go/issues/2376")
)

var (
	// Go language builtin functions.
	goBuiltins = []string{"len", "append", "panic", "make", "copy", "recover", "delete"}
	// Custom builtin utility functions that contain some meaningful code inside and
	// require code generation using standard rules, but sometimes (depending on
	// the expression usage condition) may be optimized at compile time.
	potentialCustomBuiltins = map[string]func(f ast.Expr) bool{
		"ToHash160": func(f ast.Expr) bool {
			c, ok := f.(*ast.CallExpr)
			if !ok {
				return false
			}
			if len(c.Args) != 1 {
				return false
			}
			switch c.Args[0].(type) {
			case *ast.BasicLit:
				return true
			default:
				return false
			}
		},
	}
)

// newGlobal creates a new global variable.
func (c *codegen) newGlobal(pkg string, name string) {
	name = c.getIdentName(pkg, name)
	c.globals[name] = len(c.globals)
}

// getIdentName returns a fully-qualified name for a variable.
func (c *codegen) getIdentName(pkg string, name string) string {
	if fullName, ok := c.importMap[pkg]; ok {
		pkg = fullName
	}
	return pkg + "." + name
}

// traverseGlobals visits and initializes global variables.
// It returns `true` if contract has `_deploy` function.
func (c *codegen) traverseGlobals() bool {
	var hasDefer bool
	var n, nConst int
	var hasUnusedCall bool
	var hasDeploy bool
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		nv, nc, huc := countGlobals(f, !hasUnusedCall)
		n += nv
		nConst += nc
		if huc {
			hasUnusedCall = true
		}
		if !hasDeploy || !hasDefer {
			ast.Inspect(f, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.FuncDecl:
					hasDeploy = hasDeploy || isDeployFunc(n)
				case *ast.DeferStmt:
					hasDefer = true
					return false
				}
				return true
			})
		}
	})
	if hasDefer {
		n++
	}

	if n > 255 {
		c.prog.BinWriter.Err = errors.New("too many global variables")
		return hasDeploy
	}

	if n != 0 {
		emit.Instruction(c.prog.BinWriter, opcode.INITSSLOT, []byte{byte(n)})
	}

	initOffset := c.prog.Len()
	emit.Instruction(c.prog.BinWriter, opcode.INITSLOT, []byte{0, 0})

	lastCnt, maxCnt := -1, -1
	c.ForEachPackage(func(pkg *packages.Package) {
		if n+nConst > 0 || hasUnusedCall {
			for _, f := range pkg.Syntax {
				c.fillImportMap(f, pkg)
				c.convertGlobals(f)
			}
		}
		for _, f := range pkg.Syntax {
			c.fillImportMap(f, pkg)

			var currMax int
			lastCnt, currMax = c.convertInitFuncs(f, pkg.Types, lastCnt)
			maxCnt = max(currMax, maxCnt)
		}
		// because we reuse `convertFuncDecl` for init funcs,
		// we need to clear scope, so that global variables
		// encountered after will be recognized as globals.
		c.scope = nil
	})

	maxCnt = max(c.globalInlineCount, maxCnt)

	// Here we remove `INITSLOT` if no code was emitted for `init` function.
	// Note that the `INITSSLOT` must stay in place.
	hasNoInit := initOffset+3 == c.prog.Len()
	if hasNoInit {
		buf := c.prog.Bytes()
		c.prog.Reset()
		c.prog.WriteBytes(buf[:initOffset])
	}

	if initOffset != 0 || !hasNoInit { // if there are some globals or `init()`.
		c.initEndOffset = c.prog.Len()
		emit.Opcodes(c.prog.BinWriter, opcode.RET)

		if maxCnt >= 0 {
			c.reverseOffsetMap[initOffset] = nameWithLocals{
				name:  "init",
				count: maxCnt,
			}
		}
	}

	// store auxiliary variables after all others.
	if hasDefer {
		c.exceptionIndex = len(c.globals)
		c.globals[exceptionVarName] = c.exceptionIndex
	}

	return hasDeploy
}

// countGlobals counts the global variables in the program to add
// them with the stack size of the function.
// Second returned argument contains the amount of global constants.
// If checkUnusedCalls set to true then unnamed global variables containing call
// will be searched for and their presence is returned as the last argument.
func countGlobals(f ast.Node, checkUnusedCalls bool) (int, int, bool) {
	var numVar, numConst int
	var hasUnusedCall bool
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		// Skip all function declarations if we have already encountered `defer`.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls, we are sure that each value spec
		// is a globally declared variable or constant.
		case *ast.GenDecl:
			isVar := n.Tok == token.VAR
			if isVar || n.Tok == token.CONST {
				for _, s := range n.Specs {
					valueSpec := s.(*ast.ValueSpec)
					multiRet := len(valueSpec.Values) != 0 && len(valueSpec.Names) != len(valueSpec.Values) // e.g. var A, B = f() where func f() (int, int)
					for j, id := range valueSpec.Names {
						if id.Name != "_" { // If variable has name, then it's treated as used - that's countGlobals' caller responsibility to guarantee that.
							if isVar {
								numVar++
							} else {
								numConst++
							}
						} else if isVar && len(valueSpec.Values) != 0 && checkUnusedCalls && !hasUnusedCall {
							indexToCheck := j
							if multiRet {
								indexToCheck = 0
							}
							hasUnusedCall = containsCall(valueSpec.Values[indexToCheck])
						}
					}
				}
			}
			return false
		}
		return true
	})
	return numVar, numConst, hasUnusedCall
}

// containsCall traverses node and looks if it contains a function or method call.
func containsCall(n ast.Node) bool {
	var hasCall bool
	ast.Inspect(n, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.CallExpr:
			hasCall = true
		case *ast.Ident:
			// Can safely skip idents immediately, we're interested at function calls only.
			return false
		}
		return !hasCall
	})
	return hasCall
}

// isExprNil looks if the given expression is a `nil`.
func isExprNil(e ast.Expr) bool {
	v, ok := e.(*ast.Ident)
	return ok && v.Name == "nil"
}

// indexOfStruct returns the index of the given field inside that struct.
// If the struct does not contain that field, it will return -1.
func indexOfStruct(strct *types.Struct, fldName string) int {
	for i := 0; i < strct.NumFields(); i++ {
		if strct.Field(i).Name() == fldName {
			return i
		}
	}
	return -1
}

type funcUsage map[string]bool

func (f funcUsage) funcUsed(name string) bool {
	_, ok := f[name]
	return ok
}

// lastStmtIsReturn checks if the last statement of the declaration was return statement.
func lastStmtIsReturn(body *ast.BlockStmt) (b bool) {
	if l := len(body.List); l != 0 {
		switch inner := body.List[l-1].(type) {
		case *ast.BlockStmt:
			return lastStmtIsReturn(inner)
		case *ast.ReturnStmt:
			return true
		default:
			return false
		}
	}
	return false
}

// analyzePkgOrder sets the order in which packages should be processed.
// From Go spec:
//
//	A package with no imports is initialized by assigning initial values to all its package-level variables
//	followed by calling all init functions in the order they appear in the source, possibly in multiple files,
//	as presented to the compiler. If a package has imports, the imported packages are initialized before
//	initializing the package itself. If multiple packages import a package, the imported package
//	will be initialized only once. The importing of packages, by construction, guarantees
//	that there can be no cyclic initialization dependencies.
func (c *codegen) analyzePkgOrder() {
	seen := make(map[string]bool)
	info := c.buildInfo.program[0]
	c.visitPkg(info, seen)
}

func (c *codegen) visitPkg(pkg *packages.Package, seen map[string]bool) {
	if seen[pkg.PkgPath] {
		return
	}
	for _, imp := range pkg.Types.Imports() {
		var subpkg = pkg.Imports[imp.Path()]
		if subpkg == nil {
			if c.prog.Err == nil {
				c.prog.Err = fmt.Errorf("failed to load %q package from %q, import cycle?", imp.Path(), pkg.PkgPath)
			}
			return
		}
		c.visitPkg(subpkg, seen)
	}
	seen[pkg.PkgPath] = true
	c.packages = append(c.packages, pkg.PkgPath)
	c.packageCache[pkg.PkgPath] = pkg
}

func (c *codegen) fillDocumentInfo() {
	fset := c.buildInfo.config.Fset
	fset.Iterate(func(f *token.File) bool {
		filePath := f.Position(f.Pos(0)).Filename
		c.docIndex[filePath] = len(c.documents)
		c.documents = append(c.documents, filePath)
		return true
	})
}

// analyzeFuncAndGlobalVarUsage traverses all code and returns a map with functions
// which should be present in the emitted code.
// This is done using BFS starting from exported functions or
// the function used in variable declarations (graph edge corresponds to
// the function being called in declaration). It also analyzes global variables
// usage preserving the same traversal strategy and rules. Unused global variables
// are renamed to "_" in the end. Global variable is treated as "used" iff:
// 1. It belongs either to main or to exported package AND is used directly from the exported (or _init\_deploy) method of the main package.
// 2. It belongs either to main or to exported package AND is used non-directly from the exported (or _init\_deploy) method of the main package
// (e.g. via series of function calls or in some expression that is "used").
// 3. It belongs either to main or to exported package AND contains function call inside its value definition.
func (c *codegen) analyzeFuncAndGlobalVarUsage() funcUsage {
	type declPair struct {
		decl      *ast.FuncDecl
		importMap map[string]string
		path      string
	}
	// globalVar represents a global variable declaration node with the corresponding package context.
	type globalVar struct {
		decl      *ast.GenDecl // decl contains global variables declaration node (there can be multiple declarations in a single node).
		specIdx   int          // specIdx is the index of variable specification in the list of GenDecl specifications.
		varIdx    int          // varIdx is the index of variable name in the specification names.
		ident     *ast.Ident   // ident is a named global variable identifier got from the specified node.
		importMap map[string]string
		path      string
	}
	// nodeCache contains top-level function declarations.
	nodeCache := make(map[string]declPair)
	// globalVarsCache contains both used and unused declared named global vars.
	globalVarsCache := make(map[string]globalVar)
	// diff contains used functions that are not yet marked as "used" and those definition
	// requires traversal in the subsequent stages.
	diff := funcUsage{}
	// globalVarsDiff contains used named global variables that are not yet marked as "used"
	// and those declaration requires traversal in the subsequent stages.
	globalVarsDiff := funcUsage{}
	// usedExpressions contains a set of ast.Nodes that are used in the program and need to be evaluated
	// (either they are used from the used functions OR belong to global variable declaration and surrounded by a function call)
	var usedExpressions []nodeContext
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		var pkgPath string
		isMain := pkg == c.mainPkg.Types
		if !isMain {
			pkgPath = pkg.Path()
		}

		ast.Inspect(f, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				// functions invoked in variable declarations in imported packages
				// are marked as used.
				var name string
				switch t := n.Fun.(type) {
				case *ast.Ident:
					name = c.getIdentName(pkgPath, t.Name)
				case *ast.SelectorExpr:
					name, _ = c.getFuncNameFromSelector(t)
				default:
					return true
				}
				diff[name] = true
			case *ast.FuncDecl:
				name := c.getFuncNameFromDecl(pkgPath, n)

				// filter out generic functions
				err := c.checkGenericsFuncDecl(n, name)
				if err != nil {
					c.prog.Err = err
					return false // Program is invalid.
				}

				// exported functions and methods are always assumed to be used
				if isMain && n.Name.IsExported() || isInitFunc(n) || isDeployFunc(n) {
					diff[name] = true
				}
				// exported functions are not allowed to have unnamed parameters  or multiple return values
				if isMain && n.Name.IsExported() && n.Recv == nil {
					if n.Type.Params.List != nil {
						for i, param := range n.Type.Params.List {
							if param.Names == nil {
								c.prog.Err = fmt.Errorf("%w: %s", ErrMissingExportedParamName, n.Name)
								return false // Program is invalid.
							}
							for _, name := range param.Names {
								if name == nil || name.Name == "_" {
									c.prog.Err = fmt.Errorf("%w: %s/%d", ErrMissingExportedParamName, n.Name, i)
									return false // Program is invalid.
								}
							}
						}
					}
					if retCnt := n.Type.Results.NumFields(); retCnt > 1 {
						c.prog.Err = fmt.Errorf("%w: %s/%d return values", ErrInvalidExportedRetCount, n.Name, retCnt)
					}
				}
				nodeCache[name] = declPair{n, c.importMap, pkgPath}
				return false // will be processed in the next stage
			case *ast.GenDecl:
				// Filter out generics usage.
				err := c.checkGenericsGenDecl(n, pkgPath)
				if err != nil {
					c.prog.Err = err
					return false // Program is invalid.
				}

				// After skipping all funcDecls, we are sure that each value spec
				// is a globally declared variable or constant. We need to gather global
				// vars from both main and imported packages.
				if n.Tok == token.VAR {
					for i, s := range n.Specs {
						valSpec := s.(*ast.ValueSpec)
						for j, id := range valSpec.Names {
							if id.Name != "_" {
								name := c.getIdentName(pkgPath, id.Name)
								globalVarsCache[name] = globalVar{
									decl:      n,
									specIdx:   i,
									varIdx:    j,
									ident:     id,
									importMap: c.importMap,
									path:      pkgPath,
								}
							}
							// Traverse both named/unnamed global variables, check whether function/method call
							// is present inside variable value and if so, mark all its children as "used" for
							// further traversal and evaluation.
							if len(valSpec.Values) == 0 {
								continue
							}
							multiRet := len(valSpec.Values) != len(valSpec.Names)
							if (j == 0 || !multiRet) && containsCall(valSpec.Values[j]) {
								usedExpressions = append(usedExpressions, nodeContext{
									node:      valSpec.Values[j],
									path:      pkgPath,
									importMap: c.importMap,
									typeInfo:  c.typeInfo,
									currPkg:   c.currPkg,
								})
							}
						}
					}
				}
			}
			return true
		})
	})
	if c.prog.Err != nil {
		return nil
	}

	// Handle nodes that contain (or surrounded by) function calls and are a part
	// of global variable declaration.
	c.pickVarsFromNodes(usedExpressions, func(name string) {
		if _, gOK := globalVarsCache[name]; gOK {
			globalVarsDiff[name] = true
		}
	})

	// Traverse the set of upper-layered used functions and construct the functions' usage map.
	// At the same time, go through the whole set of used functions and mark global vars used
	// from these functions as "used". Also mark the global variables from the previous step
	// and their children as "used".
	usage := funcUsage{}
	globalVarsUsage := funcUsage{}
	for len(diff) != 0 || len(globalVarsDiff) != 0 {
		nextDiff := funcUsage{}
		nextGlobalVarsDiff := funcUsage{}
		usedExpressions = usedExpressions[:0]
		for name := range diff {
			fd, ok := nodeCache[name]
			if !ok || usage[name] {
				continue
			}
			usage[name] = true

			pkg := c.mainPkg
			if fd.path != "" {
				pkg = c.packageCache[fd.path]
			}
			c.typeInfo = pkg.TypesInfo
			c.currPkg = pkg
			c.importMap = fd.importMap
			ast.Inspect(fd.decl, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					switch t := n.Fun.(type) {
					case *ast.Ident:
						nextDiff[c.getIdentName(fd.path, t.Name)] = true
					case *ast.SelectorExpr:
						name, _ := c.getFuncNameFromSelector(t)
						nextDiff[name] = true
					}
				}
				return true
			})
			usedExpressions = append(usedExpressions, nodeContext{
				node:      fd.decl.Body,
				path:      fd.path,
				importMap: c.importMap,
				typeInfo:  c.typeInfo,
				currPkg:   c.currPkg,
			})
		}

		// Traverse used global vars in a separate cycle so that we're sure there's no other unrelated vars.
		// Mark their children as "used".
		for name := range globalVarsDiff {
			fd, ok := globalVarsCache[name]
			if !ok || globalVarsUsage[name] {
				continue
			}
			globalVarsUsage[name] = true
			pkg := c.mainPkg
			if fd.path != "" {
				pkg = c.packageCache[fd.path]
			}
			valSpec := fd.decl.Specs[fd.specIdx].(*ast.ValueSpec)
			if len(valSpec.Values) == 0 {
				continue
			}
			multiRet := len(valSpec.Values) != len(valSpec.Names)
			if fd.varIdx == 0 || !multiRet {
				usedExpressions = append(usedExpressions, nodeContext{
					node:      valSpec.Values[fd.varIdx],
					path:      fd.path,
					importMap: fd.importMap,
					typeInfo:  pkg.TypesInfo,
					currPkg:   pkg,
				})
			}
		}
		c.pickVarsFromNodes(usedExpressions, func(name string) {
			if _, gOK := globalVarsCache[name]; gOK {
				nextGlobalVarsDiff[name] = true
			}
		})
		diff = nextDiff
		globalVarsDiff = nextGlobalVarsDiff
	}

	// Tiny hack: rename all remaining unused global vars. After that these unused
	// vars will be handled as any other unnamed unused variables, i.e.
	// c.traverseGlobals() won't take them into account during static slot creation
	// and the code won't be emitted for them.
	for name, node := range globalVarsCache {
		if _, ok := globalVarsUsage[name]; !ok {
			node.ident.Name = "_"
		}
	}
	return usage
}

// checkGenericFuncDecl checks whether provided ast.FuncDecl has generic code.
func (c *codegen) checkGenericsFuncDecl(n *ast.FuncDecl, funcName string) error {
	var errGenerics error

	// Generic function receiver.
	if n.Recv != nil {
		switch t := n.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			switch t.X.(type) {
			case *ast.IndexExpr:
				// func (x *Pointer[T]) Load() *T
				errGenerics = errors.New("generic pointer function receiver")
			}
		case *ast.IndexExpr:
			// func (x Structure[T]) Load() *T
			errGenerics = errors.New("generic function receiver")
		}
	}

	// Generic function parameters type: func SumInts[V int64 | int32](vals []V) V
	if n.Type.TypeParams != nil {
		errGenerics = errors.New("function type parameters")
	}

	if errGenerics != nil {
		return fmt.Errorf("%w: %s has %s", ErrGenericsUnsuppored, funcName, errGenerics.Error())
	}

	return nil
}

// checkGenericsGenDecl checks whether provided ast.GenDecl has generic code.
func (c *codegen) checkGenericsGenDecl(n *ast.GenDecl, pkgPath string) error {
	// Generic type declaration:
	// 	type List[T any] struct
	// 	type List[T any] interface
	if n.Tok == token.TYPE {
		for _, s := range n.Specs {
			typeSpec := s.(*ast.TypeSpec)
			if typeSpec.TypeParams != nil {
				return fmt.Errorf("%w: type %s is generic", ErrGenericsUnsuppored, c.getIdentName(pkgPath, typeSpec.Name.Name))
			}
		}
	}

	return nil
}

// nodeContext contains ast node with the corresponding import map, type info and package information
// required to retrieve fully qualified node name (if so).
type nodeContext struct {
	node      ast.Node
	path      string
	importMap map[string]string
	typeInfo  *types.Info
	currPkg   *packages.Package
}

// derive returns provided node with the parent's context.
func (c nodeContext) derive(n ast.Node) nodeContext {
	return nodeContext{
		node:      n,
		path:      c.path,
		importMap: c.importMap,
		typeInfo:  c.typeInfo,
		currPkg:   c.currPkg,
	}
}

// pickVarsFromNodes searches for variables used in the given set of nodes
// calling markAsUsed for each variable. Be careful while using codegen after
// pickVarsFromNodes, it changes importMap, currPkg and typeInfo.
func (c *codegen) pickVarsFromNodes(nodes []nodeContext, markAsUsed func(name string)) {
	for len(nodes) != 0 {
		var nextExprToCheck []nodeContext
		for _, val := range nodes {
			// Set variable context for proper name extraction.
			c.importMap = val.importMap
			c.currPkg = val.currPkg
			c.typeInfo = val.typeInfo
			ast.Inspect(val.node, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.KeyValueExpr: // var _ = f() + CustomInt{Int: Unused}.Int + 3 => mark Unused as "used".
					nextExprToCheck = append(nextExprToCheck, val.derive(n.Value))
					return false
				case *ast.CallExpr:
					switch t := n.Fun.(type) {
					case *ast.Ident:
						// Do nothing, used functions are handled in a separate cycle.
					case *ast.SelectorExpr:
						nextExprToCheck = append(nextExprToCheck, val.derive(t))
					}
					for _, arg := range n.Args {
						switch arg.(type) {
						case *ast.BasicLit:
						default:
							nextExprToCheck = append(nextExprToCheck, val.derive(arg))
						}
					}
					return false
				case *ast.SelectorExpr:
					if c.typeInfo.Selections[n] != nil {
						switch t := n.X.(type) {
						case *ast.Ident:
							nextExprToCheck = append(nextExprToCheck, val.derive(t))
						case *ast.CompositeLit:
							nextExprToCheck = append(nextExprToCheck, val.derive(t))
						case *ast.SelectorExpr: // imp_pkg.Anna.GetAge() => mark Anna (exported global struct) as used.
							nextExprToCheck = append(nextExprToCheck, val.derive(t))
						}
					} else {
						ident := n.X.(*ast.Ident)
						name := c.getIdentName(ident.Name, n.Sel.Name)
						markAsUsed(name)
					}
					return false
				case *ast.CompositeLit: // var _ = f(1) + []int{1, Unused, 3}[1] => mark Unused as "used".
					for _, e := range n.Elts {
						switch e.(type) {
						case *ast.BasicLit:
						default:
							nextExprToCheck = append(nextExprToCheck, val.derive(e))
						}
					}
					return false
				case *ast.Ident:
					name := c.getIdentName(val.path, n.Name)
					markAsUsed(name)
					return false
				case *ast.DeferStmt:
					nextExprToCheck = append(nextExprToCheck, val.derive(n.Call.Fun))
					return false
				case *ast.BasicLit:
					return false
				}
				return true
			})
		}
		nodes = nextExprToCheck
	}
}

func isGoBuiltin(name string) bool {
	return slices.Contains(goBuiltins, name)
}

func isPotentialCustomBuiltin(f *funcScope, expr ast.Expr) bool {
	if !isInteropPath(f.pkg.Path()) {
		return false
	}
	for name, isBuiltin := range potentialCustomBuiltins {
		if f.name == name && isBuiltin(expr) {
			return true
		}
	}
	return false
}

func isSyscall(fun *funcScope) bool {
	if fun.selector == nil || fun.pkg == nil || !isInteropPath(fun.pkg.Path()) {
		return false
	}
	return fun.pkg.Name() == "neogointernal" && (strings.HasPrefix(fun.name, "Syscall") ||
		strings.HasPrefix(fun.name, "Opcode") || strings.HasPrefix(fun.name, "CallWithToken"))
}

const interopPrefix = "github.com/nspcc-dev/neo-go/pkg/interop"

func isInteropPath(s string) bool {
	return strings.HasPrefix(s, interopPrefix)
}

// canConvert returns true if type doesn't need to be converted on type assertion.
func canConvert(s string) bool {
	if len(s) != 0 && s[0] == '*' {
		s = s[1:]
	}
	if isInteropPath(s) {
		s = s[len(interopPrefix):]
		return s != "/iterator.Iterator" && s != "/storage.Context" &&
			s != "/native/ledger.Block" && s != "/native/ledger.Transaction" &&
			s != "/native/management.Contract" && s != "/native/neo.AccountState" &&
			s != "/native/ledger.BlockSR"
	}
	return true
}

// canInline returns true if the function is to be inlined.
// The list of functions that can be inlined is not static, it depends on the function usages.
// isBuiltin denotes whether code generation for dynamic builtin function will be performed
// manually.
func canInline(s string, name string, isBuiltin bool) bool {
	if strings.HasPrefix(s, "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline") {
		return true
	}
	if !isInteropPath(s) {
		return false
	}
	return !strings.HasPrefix(s[len(interopPrefix):], "/neogointernal") &&
		!(strings.HasPrefix(s[len(interopPrefix):], "/util") && name == "FromAddress") &&
		!(strings.HasPrefix(s[len(interopPrefix):], "/lib/address") && name == "ToHash160" && isBuiltin)
}
