package newcompiler

import (
	"go/ast"
	"log"
)

// A funcScope represents a scope within the function context.
// It holds al the local variables along with the initialized struct positions.
type funcScope struct {
	// function identifier
	name string

	// The declaration of the function in the AST
	decl *ast.FuncDecl

	// program label of the function
	label int

	// local scope of the function
	scope map[string]int

	// mapping of structs positions with their scope
	structs map[int]*structScope

	// local variable counter
	i int
}

func newFuncScope(decl *ast.FuncDecl, label int) *funcScope {
	return &funcScope{
		name:    decl.Name.Name,
		decl:    decl,
		label:   label,
		scope:   map[string]int{},
		structs: map[int]*structScope{},
		i:       -1,
	}
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
		numArgs = len(c.decl.Recv.List)
	}
	return int64(size + numArgs)
}

func (c *funcScope) newStruct() *structScope {
	strct := newStructScope()
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
