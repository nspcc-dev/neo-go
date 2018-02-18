package newcompiler

import (
	"go/ast"
	"log"
)

type structScope struct {
	// identifier of the initialized struct in the program.
	name string

	// a mapping of field identifier and its position.
	fields map[string]int
}

func newStructScope() *structScope {
	return &structScope{
		fields: map[string]int{},
	}
}

func (s *structScope) newField(name string) int {
	i := len(s.fields)
	s.fields[name] = i
	return i
}

func (s *structScope) loadField(name string) int {
	i, ok := s.fields[name]
	if !ok {
		log.Fatalf("could not resolve field name %s for struct %s", name, s.name)
	}
	return i
}

type funcScope struct {
	// function identifier
	name string

	// the declaration in the AST
	decl *ast.FuncDecl

	// program label of the function
	label int

	// local scope of the function
	scope map[string]int

	// mapping of structs ident with its scope
	structs map[string]*structScope

	// local variable counter
	i int
}

func newFuncScope(decl *ast.FuncDecl, label int) *funcScope {
	return &funcScope{
		name:    decl.Name.Name,
		decl:    decl,
		label:   label,
		scope:   map[string]int{},
		structs: map[string]*structScope{},
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
	return int64(size + numArgs)

}

// newVar creates a new local variable into the scope of the function.
func (c *funcScope) newVar(name string) int {
	c.i++
	c.scope[name] = c.i
	return c.i
}

// loadVar loads the position of a local variable inside the scope of the function.
func (c *funcScope) loadVar(name string) int {
	i, ok := c.scope[name]
	if !ok {
		// should emit a compiler warning.
		return c.newVar(name)
	}
	return i
}

// newStruct create a new "struct" scope for this function.
func (c *funcScope) newStruct(name string) *structScope {
	strct := &structScope{
		name:   name,
		fields: map[string]int{},
	}
	c.structs[name] = strct
	return strct
}
