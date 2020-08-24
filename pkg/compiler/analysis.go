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
	goBuiltins = []string{"len", "append", "panic", "make"}
	// Custom builtin utility functions.
	customBuiltins = []string{
		"FromAddress", "Equals",
		"ToBool", "ToByteArray", "ToInteger",
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
// and returns number of variables initialized and
// true if any init functions were encountered.
func (c *codegen) traverseGlobals() (int, bool) {
	var n int
	var hasInit bool
	c.ForEachFile(func(f *ast.File, _ *types.Package) {
		n += countGlobals(f)
		if !hasInit {
			ast.Inspect(f, func(node ast.Node) bool {
				n, ok := node.(*ast.FuncDecl)
				if ok {
					if isInitFunc(n) {
						hasInit = true
					}
					return false
				}
				return true
			})
		}
	})
	if n != 0 || hasInit {
		if n > 255 {
			c.prog.BinWriter.Err = errors.New("too many global variables")
			return 0, hasInit
		}
		if n != 0 {
			emit.Instruction(c.prog.BinWriter, opcode.INITSSLOT, []byte{byte(n)})
		}
		c.ForEachPackage(func(pkg *loader.PackageInfo) {
			if n > 0 {
				for _, f := range pkg.Files {
					c.fillImportMap(f, pkg.Pkg)
					c.convertGlobals(f, pkg.Pkg)
				}
			}
			if hasInit {
				for _, f := range pkg.Files {
					c.fillImportMap(f, pkg.Pkg)
					c.convertInitFuncs(f, pkg.Pkg)
				}
			}
			// because we reuse `convertFuncDecl` for init funcs,
			// we need to cleare scope, so that global variables
			// encountered after will be recognized as globals.
			c.scope = nil
		})
	}
	return n, hasInit
}

// countGlobals counts the global variables in the program to add
// them with the stack size of the function.
func countGlobals(f ast.Node) (i int) {
	ast.Inspect(f, func(node ast.Node) bool {
		switch n := node.(type) {
		// Skip all function declarations.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls we are sure that each value spec
		// is a global declared variable or constant.
		case *ast.ValueSpec:
			i += len(n.Names)
		}
		return true
	})
	return
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
func lastStmtIsReturn(decl *ast.FuncDecl) (b bool) {
	if l := len(decl.Body.List); l != 0 {
		_, ok := decl.Body.List[l-1].(*ast.ReturnStmt)
		return ok
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
					usage[c.getIdentName("", t.Name)] = true
				case *ast.SelectorExpr:
					name, _ := c.getFuncNameFromSelector(t)
					usage[name] = true
				}
			case *ast.FuncDecl:
				// exported functions are always assumed to be used
				if isMain && n.Name.IsExported() {
					usage[c.getFuncNameFromDecl(pkg.Path(), n)] = true
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
	_, ok := syscalls[fun.pkg.Name()][fun.name]
	return ok
}

func isInteropPath(s string) bool {
	return strings.HasPrefix(s, "github.com/nspcc-dev/neo-go/pkg/interop")
}
