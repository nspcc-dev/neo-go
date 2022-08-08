package compiler

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"golang.org/x/tools/go/packages"
)

// ErrMissingExportedParamName is returned when exported contract method has unnamed parameter.
var ErrMissingExportedParamName = errors.New("exported method is not allowed to have unnamed parameter")

var (
	// Go language builtin functions.
	goBuiltins = []string{"len", "append", "panic", "make", "copy", "recover", "delete"}
	// Custom builtin utility functions.
	customBuiltins = []string{
		"FromAddress",
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
	var hasDeploy bool
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		nv, nc := countGlobals(f)
		n += nv
		nConst += nc
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
		if n+nConst > 0 {
			for _, f := range pkg.Syntax {
				c.fillImportMap(f, pkg)
				c.convertGlobals(f, pkg.Types)
			}
		}
		for _, f := range pkg.Syntax {
			c.fillImportMap(f, pkg)

			var currMax int
			lastCnt, currMax = c.convertInitFuncs(f, pkg.Types, lastCnt)
			if currMax > maxCnt {
				maxCnt = currMax
			}
		}
		// because we reuse `convertFuncDecl` for init funcs,
		// we need to clear scope, so that global variables
		// encountered after will be recognized as globals.
		c.scope = nil
	})

	if c.globalInlineCount > maxCnt {
		maxCnt = c.globalInlineCount
	}

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
func countGlobals(f ast.Node) (int, int) {
	var numVar, numConst int
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		// Skip all function declarations if we have already encountered `defer`.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls, we are sure that each value spec
		// is a global declared variable or constant.
		case *ast.GenDecl:
			isVar := n.Tok == token.VAR
			if isVar || n.Tok == token.CONST {
				for _, s := range n.Specs {
					for _, id := range s.(*ast.ValueSpec).Names {
						if id.Name != "_" {
							if isVar {
								numVar++
							} else {
								numConst++
							}
						}
					}
				}
			}
			return false
		}
		return true
	})
	return numVar, numConst
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
		c.visitPkg(pkg.Imports[imp.Path()], seen)
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

// analyzeFuncUsage traverses all code and returns a map with functions
// which should be present in the emitted code.
// This is done using BFS starting from exported functions or
// the function used in variable declarations (graph edge corresponds to
// the function being called in declaration).
func (c *codegen) analyzeFuncUsage() funcUsage {
	type declPair struct {
		decl      *ast.FuncDecl
		importMap map[string]string
		path      string
	}

	// nodeCache contains top-level function declarations .
	nodeCache := make(map[string]declPair)
	diff := funcUsage{}
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

				// exported functions are always assumed to be used
				if isMain && n.Name.IsExported() || isInitFunc(n) || isDeployFunc(n) {
					diff[name] = true
				}
				if isMain && n.Name.IsExported() {
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
				}
				nodeCache[name] = declPair{n, c.importMap, pkgPath}
				return false // will be processed in the next stage
			}
			return true
		})
	})
	if c.prog.Err != nil {
		return nil
	}

	usage := funcUsage{}
	for len(diff) != 0 {
		nextDiff := funcUsage{}
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
		}
		diff = nextDiff
	}
	return usage
}

func isGoBuiltin(name string) bool {
	for i := range goBuiltins {
		if name == goBuiltins[i] {
			return true
		}
	}
	return false
}

func isCustomBuiltin(f *funcScope) bool {
	if !isInteropPath(f.pkg.Path()) {
		return false
	}
	for _, n := range customBuiltins {
		if f.name == n {
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
// Currently, there is a static list of functions which are inlined,
// this may change in future.
func canInline(s string, name string) bool {
	if strings.HasPrefix(s, "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline") {
		return true
	}
	if !isInteropPath(s) {
		return false
	}
	return !strings.HasPrefix(s[len(interopPrefix):], "/neogointernal") &&
		!(strings.HasPrefix(s[len(interopPrefix):], "/util") && name == "FromAddress")
}
