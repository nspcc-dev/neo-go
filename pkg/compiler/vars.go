package compiler

import (
	"go/ast"
	"strconv"
)

type varScope struct {
	localsCnt int
	arguments map[string]int
	locals    []map[string]varInfo
}

type varContext struct {
	importMap map[string]string
	expr      ast.Expr
	scope     []map[string]varInfo
}

type varInfo struct {
	refType varType
	index   int
	// ctx is set for inline arguments and contains
	// context for expression traversal.
	ctx *varContext
}

const unspecifiedVarIndex = -1

func newVarScope() varScope {
	return varScope{
		arguments: make(map[string]int),
	}
}

func (c *varScope) newScope() {
	c.locals = append(c.locals, map[string]varInfo{})
}

func (c *varScope) dropScope() {
	c.locals = c.locals[:len(c.locals)-1]
}

func (c *varScope) addAlias(name string, vt varType, index int, ctx *varContext) {
	c.locals[len(c.locals)-1][name] = varInfo{
		refType: vt,
		index:   index,
		ctx:     ctx,
	}
}

func (c *varScope) getVarInfo(name string) *varInfo {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if vi, ok := c.locals[i][name]; ok {
			return &vi
		}
	}
	if i, ok := c.arguments[name]; ok {
		return &varInfo{
			refType: varArgument,
			index:   i,
		}
	}
	return nil
}

// newVariable creates a new local variable or argument in the scope of the function.
func (c *varScope) newVariable(t varType, name string) int {
	var n int
	switch t {
	case varLocal:
		return c.newLocal(name)
	case varArgument:
		if name == "_" {
			// See #2204. This name won't actually be referenced.
			// This approach simplifies argument allocation and
			// makes debugging easier.
			name = "%_" + strconv.FormatUint(uint64(len(c.arguments)), 10)
		}
		_, ok := c.arguments[name]
		if ok {
			panic("argument is already allocated")
		}
		n = len(c.arguments)
		c.arguments[name] = n
	default:
		panic("invalid type")
	}
	return n
}

// newLocal creates a new local variable in the current scope.
func (c *varScope) newLocal(name string) int {
	idx := len(c.locals) - 1
	m := c.locals[idx]
	m[name] = varInfo{
		refType: varLocal,
		index:   c.localsCnt,
	}
	c.localsCnt++
	c.locals[idx] = m
	return c.localsCnt - 1
}
