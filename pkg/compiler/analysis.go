package compiler

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"golang.org/x/tools/go/loader"
)

var (
	// Go language builtin functions.
	goBuiltins = []string{"len", "append", "panic", "make", "copy", "recover", "delete"}
	// Custom builtin utility functions.
	customBuiltins = []string{
		"FromAddress", "Equals", "Remove",
		"ToBool", "ToBytes", "ToString", "ToInteger",
	}
)

// newGlobal creates new global variable.
func (c *codegen) newGlobal(pkg string, name string) {
	name = c.getIdentName(pkg, name)
	c.globals[name] = len(c.globals)
}

// getIdentName returns fully-qualified name for a variable.
func (c *codegen) getIdentName(pkg string, name string) string {
	if fullName, ok := c.importMap[pkg]; ok {
		pkg = fullName
	}
	return pkg + "." + name
}

// traverseGlobals visits and initializes global variables.
// and returns number of variables initialized.
// Second return value is -1 if no `init()` functions were encountered
// and number of maximum amount of locals in any of init functions otherwise.
// Same for `_deploy()` functions (see docs/compiler.md).
func (c *codegen) traverseGlobals() (int, int, int) {
	var hasDefer bool
	var n, nConst int
	initLocals := -1
	deployLocals := -1
	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		nv, nc := countGlobals(f)
		n += nv
		nConst += nc
		if initLocals == -1 || deployLocals == -1 || !hasDefer {
			ast.Inspect(f, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.GenDecl:
					if n.Tok == token.VAR {
						for i := range n.Specs {
							for _, v := range n.Specs[i].(*ast.ValueSpec).Values {
								num := c.countLocalsCall(v, pkg)
								if num > initLocals {
									initLocals = num
								}
							}
						}
					}
				case *ast.FuncDecl:
					if isInitFunc(n) {
						num, _ := c.countLocals(n)
						if num > initLocals {
							initLocals = num
						}
					} else if isDeployFunc(n) {
						num, _ := c.countLocals(n)
						if num > deployLocals {
							deployLocals = num
						}
					}
					return !hasDefer
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
	if n+nConst != 0 || initLocals > -1 {
		if n > 255 {
			c.prog.BinWriter.Err = errors.New("too many global variables")
			return 0, initLocals, deployLocals
		}
		if n != 0 {
			emit.Instruction(c.prog.BinWriter, opcode.INITSSLOT, []byte{byte(n)})
		}
		if initLocals > 0 {
			emit.Instruction(c.prog.BinWriter, opcode.INITSLOT, []byte{byte(initLocals), 0})
		}
		seenBefore := false
		c.ForEachPackage(func(pkg *loader.PackageInfo) {
			if n+nConst > 0 {
				for _, f := range pkg.Files {
					c.fillImportMap(f, pkg.Pkg)
					c.convertGlobals(f, pkg.Pkg)
				}
			}
			if initLocals > -1 {
				for _, f := range pkg.Files {
					c.fillImportMap(f, pkg.Pkg)
					seenBefore = c.convertInitFuncs(f, pkg.Pkg, seenBefore) || seenBefore
				}
			}
			// because we reuse `convertFuncDecl` for init funcs,
			// we need to cleare scope, so that global variables
			// encountered after will be recognized as globals.
			c.scope = nil
		})
		// store auxiliary variables after all others.
		if hasDefer {
			c.exceptionIndex = len(c.globals)
			c.globals[exceptionVarName] = c.exceptionIndex
		}
	}
	return n, initLocals, deployLocals
}

// countGlobals counts the global variables in the program to add
// them with the stack size of the function.
// Second returned argument contains amount of global constants.
func countGlobals(f ast.Node) (int, int) {
	var numVar, numConst int
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		// Skip all function declarations if we have already encountered `defer`.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls we are sure that each value spec
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
// If the struct does not contain that field it will return -1.
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

// lastStmtIsReturn checks if last statement of the declaration was return statement..
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
//   A package with no imports is initialized by assigning initial values to all its package-level variables
//   followed by calling all init functions in the order they appear in the source, possibly in multiple files,
//   as presented to the compiler. If a package has imports, the imported packages are initialized before
//   initializing the package itself. If multiple packages import a package, the imported package
//   will be initialized only once. The importing of packages, by construction, guarantees
//   that there can be no cyclic initialization dependencies.
func (c *codegen) analyzePkgOrder() {
	seen := make(map[string]bool)
	info := c.buildInfo.program.Package(c.buildInfo.initialPackage)
	c.visitPkg(info.Pkg, seen)
}

func (c *codegen) visitPkg(pkg *types.Package, seen map[string]bool) {
	pkgPath := pkg.Path()
	if seen[pkgPath] {
		return
	}
	for _, imp := range pkg.Imports() {
		c.visitPkg(imp, seen)
	}
	seen[pkgPath] = true
	c.packages = append(c.packages, pkgPath)
}

func (c *codegen) fillDocumentInfo() {
	fset := c.buildInfo.program.Fset
	fset.Iterate(func(f *token.File) bool {
		filePath := f.Position(f.Pos(0)).Filename
		c.docIndex[filePath] = len(c.documents)
		c.documents = append(c.documents, filePath)
		return true
	})
}

func (c *codegen) analyzeFuncUsage() funcUsage {
	usage := funcUsage{}

	c.ForEachFile(func(f *ast.File, pkg *types.Package) {
		isMain := pkg == c.mainPkg.Pkg
		ast.Inspect(f, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				switch t := n.Fun.(type) {
				case *ast.Ident:
					var pkgPath string
					if !isMain {
						pkgPath = pkg.Path()
					}
					usage[c.getIdentName(pkgPath, t.Name)] = true
				case *ast.SelectorExpr:
					name, _ := c.getFuncNameFromSelector(t)
					usage[name] = true
				}
			case *ast.FuncDecl:
				// exported functions are always assumed to be used
				if isMain && n.Name.IsExported() {
					usage[c.getFuncNameFromDecl("", n)] = true
				}
			}
			return true
		})
	})
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
		strings.HasPrefix(fun.name, "Opcode"))
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
			s != "/native/management.Contract"
	}
	return true
}

// canInline returns true if function is to be inlined.
// Currently there is a static list of function which are inlined,
// this may change in future.
func canInline(s string) bool {
	if strings.HasPrefix(s, "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline") {
		return true
	}
	if !isInteropPath(s) {
		return false
	}
	return !strings.HasPrefix(s[len(interopPrefix):], "/neogointernal") &&
		!strings.HasPrefix(s[len(interopPrefix):], "/util")
}
