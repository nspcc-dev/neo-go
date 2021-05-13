package compiler

import (
	"go/types"
)

type varScope struct {
	localsCnt int
	arguments map[string]int
	locals    []map[string]varInfo
}

type varInfo struct {
	refType varType
	index   int
	tv      types.TypeAndValue
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

func (c *varScope) addAlias(name string, vt varType, index int, tv types.TypeAndValue) {
	c.locals[len(c.locals)-1][name] = varInfo{
		refType: vt,
		index:   index,
		tv:      tv,
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
