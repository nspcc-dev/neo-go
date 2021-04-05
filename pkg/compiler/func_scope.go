package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
)

// A funcScope represents the scope within the function context.
// It holds al the local variables along with the initialized struct positions.
type funcScope struct {
	// Identifier of the function.
	name string

	// Selector of the function if there is any. Only functions imported
	// from other packages should have a selector.
	selector *ast.Ident

	// The declaration of the function in the AST. Nil if this scope is not a function.
	decl *ast.FuncDecl

	// Package where the function is defined.
	pkg *types.Package

	file *ast.File

	// Program label of the scope
	label uint16

	// Range of opcodes corresponding to the function.
	rng DebugRange
	// Variables together with it's type in neo-vm.
	variables []string

	// deferStack is a stack containing encountered `defer` statements.
	deferStack []deferInfo
	// finallyProcessed is a index of static slot with boolean flag determining
	// if `defer` statement was already processed.
	finallyProcessedIndex int

	// Local variables
	vars varScope

	// voidCalls are basically functions that return their value
	// into nothing. The stack has their return value but there
	// is nothing that consumes it. We need to keep track of
	// these functions so we can cleanup (drop) the returned
	// value from the stack. We also need to add every voidCall
	// return value to the stack size.
	voidCalls map[*ast.CallExpr]bool

	// Local variable counter.
	i int
}

type deferInfo struct {
	catchLabel   uint16
	finallyLabel uint16
	expr         *ast.CallExpr
}

func (c *codegen) newFuncScope(decl *ast.FuncDecl, label uint16) *funcScope {
	var name string
	if decl.Name != nil {
		name = decl.Name.Name
	}
	return &funcScope{
		name:      name,
		decl:      decl,
		label:     label,
		pkg:       c.currPkg,
		vars:      newVarScope(),
		voidCalls: map[*ast.CallExpr]bool{},
		variables: []string{},
		i:         -1,
	}
}

func (c *codegen) getFuncNameFromDecl(pkgPath string, decl *ast.FuncDecl) string {
	name := decl.Name.Name
	if decl.Recv != nil {
		switch t := decl.Recv.List[0].Type.(type) {
		case *ast.Ident:
			name = t.Name + "." + name
		case *ast.StarExpr:
			name = t.X.(*ast.Ident).Name + "." + name
		}
	}
	return c.getIdentName(pkgPath, name)
}

// analyzeVoidCalls checks for functions that are not assigned
// and therefore we need to cleanup the return value from the stack.
func (c *funcScope) analyzeVoidCalls(node ast.Node) bool {
	est, ok := node.(*ast.ExprStmt)
	if ok {
		ce, ok := est.X.(*ast.CallExpr)
		if ok {
			c.voidCalls[ce] = true
		}
	}
	return true
}

func (c *codegen) countLocalsCall(n ast.Expr, pkg *types.Package) int {
	ce, ok := n.(*ast.CallExpr)
	if !ok {
		return -1
	}

	var size int
	var name string
	switch fun := ce.Fun.(type) {
	case *ast.Ident:
		var pkgName string
		if pkg != nil {
			pkgName = pkg.Path()
		}
		name = c.getIdentName(pkgName, fun.Name)
	case *ast.SelectorExpr:
		name, _ = c.getFuncNameFromSelector(fun)
	default:
		return 0
	}
	if inner, ok := c.funcs[name]; ok && canInline(name) {
		sig, ok := c.typeOf(ce.Fun).(*types.Signature)
		if !ok {
			info := c.buildInfo.program.Package(pkg.Path())
			sig = info.Types[ce.Fun].Type.(*types.Signature)
		}
		for i := range ce.Args {
			switch ce.Args[i].(type) {
			case *ast.Ident:
			case *ast.BasicLit:
			default:
				size++
			}
		}
		// Variadic with direct var args.
		if sig.Variadic() && !ce.Ellipsis.IsValid() {
			size++
		}
		innerSz, _ := c.countLocalsInline(inner.decl, inner.pkg, inner)
		size += innerSz
	}
	return size
}

func (c *codegen) countLocals(decl *ast.FuncDecl) (int, bool) {
	return c.countLocalsInline(decl, nil, nil)
}

func (c *codegen) countLocalsInline(decl *ast.FuncDecl, pkg *types.Package, f *funcScope) (int, bool) {
	oldMap := c.importMap
	if pkg != nil {
		c.fillImportMap(f.file, pkg)
	}

	size := 0
	hasDefer := false
	ast.Inspect(decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			size += c.countLocalsCall(n, pkg)
		case *ast.FuncType:
			num := n.Results.NumFields()
			if num != 0 && len(n.Results.List[0].Names) != 0 {
				size += num
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				size += len(n.Lhs)
			}
		case *ast.DeferStmt:
			hasDefer = true
			return false
		case *ast.ReturnStmt:
			if pkg == nil {
				size++
			}
		case *ast.IfStmt:
			size++
		// This handles the inline GenDecl like "var x = 2"
		case *ast.ValueSpec:
			size += len(n.Names)
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				if n.Key != nil {
					size++
				}
				if n.Value != nil {
					size++
				}
			}
		}
		return true
	})
	if pkg != nil {
		c.importMap = oldMap
	}
	return size, hasDefer
}

func (c *codegen) countLocalsWithDefer(f *funcScope) int {
	size, hasDefer := c.countLocals(f.decl)
	if hasDefer {
		f.finallyProcessedIndex = size
		size++
	}
	return size
}

func (c *funcScope) countArgs() int {
	n := c.decl.Type.Params.NumFields()
	if c.decl.Recv != nil {
		n += c.decl.Recv.NumFields()
	}
	return n
}

// newVariable creates a new local variable or argument in the scope of the function.
func (c *funcScope) newVariable(t varType, name string) int {
	return c.vars.newVariable(t, name)
}

// newLocal creates a new local variable into the scope of the function.
func (c *funcScope) newLocal(name string) int {
	return c.newVariable(varLocal, name)
}
