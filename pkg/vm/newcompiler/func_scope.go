package newcompiler

import (
	"go/ast"
	"log"
)

type funcScope struct {
	name  string         // function identifier
	decl  *ast.FuncDecl  // the declaration in the AST
	label int            // position of the function in the program
	scope map[string]int // local scope of the function
	i     int            // local variable counter
}

func newFuncScope(decl *ast.FuncDecl, label int) *funcScope {
	return &funcScope{
		name:  decl.Name.Name,
		decl:  decl,
		label: label,
		scope: map[string]int{},
		i:     -1,
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
	return int64(size + numArgs)

}

func (c *funcScope) newVar(name string) int {
	c.i++
	c.scope[name] = c.i
	return c.i
}

func (c *funcScope) loadVar(name string) int {
	i, ok := c.scope[name]
	if !ok {
		log.Fatalf("could not resolve local variable %s", name)
	}
	return i
}
