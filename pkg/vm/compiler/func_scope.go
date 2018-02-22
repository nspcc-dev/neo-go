package compiler

import (
	"go/ast"
	"go/types"
	"log"
)

// A funcScope represents a scope within the function context.
// It holds al the local variables along with the initialized struct positions.
type funcScope struct {
	// function identifier
	name string

	// The declaration of the function in the AST
	decl *ast.FuncDecl

	// Program label of the function
	label int

	// Local scope of the function
	scope map[string]int

	// A mapping of structs positions with their scope
	structs map[int]*structScope

	// voidCalls are basically functions that return their value
	// into nothing. The stack has their return value but there
	// is nothing that consumes it. We need to keep track of
	// these functions so we can cleanup (drop) the returned
	// value from the stack. We also need to add every voidCall
	// return value to the stack size.
	voidCalls map[*ast.CallExpr]bool

	// local variable counter
	i int
}

func newFuncScope(decl *ast.FuncDecl, label int) *funcScope {
	return &funcScope{
		name:      decl.Name.Name,
		decl:      decl,
		label:     label,
		scope:     map[string]int{},
		structs:   map[int]*structScope{},
		voidCalls: map[*ast.CallExpr]bool{},
		i:         -1,
	}
}

// analyzeVoidCalls will check for functions that are not assigned
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
	case *ast.CallExpr:
		c.voidCalls[n] = true
	}
	return true
}

func (c *funcScope) stackSize() int64 {
	size := 0
	ast.Inspect(c.decl, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.AssignStmt, *ast.ReturnStmt, *ast.IfStmt:
			size++
		}
		return true
	})

	numArgs := len(c.decl.Type.Params.List)
	// Also take care of struct methods recv: e.g. (t Token).Foo().
	if c.decl.Recv != nil {
		numArgs += len(c.decl.Recv.List)
	}
	return int64(size + numArgs + len(c.voidCalls))
}

func (c *funcScope) newStruct(t *types.Struct) *structScope {
	strct := newStructScope(t)
	c.structs[len(c.scope)] = strct
	return strct
}

func (c *funcScope) loadStruct(name string) *structScope {
	l := c.loadLocal(name)
	strct, ok := c.structs[l]
	if !ok {
		log.Fatalf("could not resolve struct %s", name)
	}
	return strct
}

// newLocal creates a new local variable into the scope of the function.
func (c *funcScope) newLocal(name string) int {
	c.i++
	c.scope[name] = c.i
	return c.i
}

// loadLocal loads the position of a local variable inside the scope of the function.
func (c *funcScope) loadLocal(name string) int {
	i, ok := c.scope[name]
	if !ok {
		// should emit a compiler warning.
		return c.newLocal(name)
	}
	return i
}
