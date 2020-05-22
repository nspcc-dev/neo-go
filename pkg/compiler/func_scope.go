package compiler

import (
	"go/ast"
	"go/token"
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

	// Program label of the scope
	label uint16

	// Range of opcodes corresponding to the function.
	rng DebugRange
	// Variables together with it's type in neo-vm.
	variables []string

	// Local variables
	locals    map[string]int
	arguments map[string]int

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

func newFuncScope(decl *ast.FuncDecl, label uint16) *funcScope {
	var name string
	if decl.Name != nil {
		name = decl.Name.Name
	}
	return &funcScope{
		name:      name,
		decl:      decl,
		label:     label,
		locals:    map[string]int{},
		arguments: map[string]int{},
		voidCalls: map[*ast.CallExpr]bool{},
		variables: []string{},
		i:         -1,
	}
}

// analyzeVoidCalls checks for functions that are not assigned
// and therefore we need to cleanup the return value from the stack.
func (c *funcScope) analyzeVoidCalls(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for i := 0; i < len(n.Rhs); i++ {
			switch n.Rhs[i].(type) {
			case *ast.CallExpr:
				return false
			}
		}
	case *ast.ReturnStmt:
		if len(n.Results) > 0 {
			switch n.Results[0].(type) {
			case *ast.CallExpr:
				return false
			}
		}
	case *ast.BinaryExpr:
		return false
	case *ast.CallExpr:
		c.voidCalls[n] = true
		return false
	}
	return true
}

func (c *funcScope) countLocals() int {
	size := 0
	ast.Inspect(c.decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncType:
			num := n.Results.NumFields()
			if num != 0 && len(n.Results.List[0].Names) != 0 {
				size += num
			}
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				size += len(n.Rhs)
			}
		case *ast.ReturnStmt, *ast.IfStmt:
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
	return size
}

func (c *funcScope) countArgs() int {
	n := c.decl.Type.Params.NumFields()
	if c.decl.Recv != nil {
		n += c.decl.Recv.NumFields()
	}
	return n
}

func (c *funcScope) stackSize() int64 {
	size := c.countLocals()
	numArgs := c.countArgs()
	return int64(size + numArgs + len(c.voidCalls))
}

// newVariable creates a new local variable or argument in the scope of the function.
func (c *funcScope) newVariable(t varType, name string) int {
	c.i++
	switch t {
	case varLocal:
		c.locals[name] = c.i
	case varArgument:
		c.arguments[name] = c.i
	default:
		panic("invalid type")
	}
	return c.i
}

// newLocal creates a new local variable into the scope of the function.
func (c *funcScope) newLocal(name string) int {
	return c.newVariable(varLocal, name)
}
