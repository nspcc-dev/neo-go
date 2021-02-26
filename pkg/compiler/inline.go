package compiler

import (
	"go/ast"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// inlineCall inlines call of n for function represented by f.
// Call `f(a,b)` for definition `func f(x,y int)` is translated to block:
// {
//    x := a
//    y := b
//    <inline body of f directly>
// }
func (c *codegen) inlineCall(f *funcScope, n *ast.CallExpr) {
	labelSz := len(c.labelList)
	offSz := len(c.inlineLabelOffsets)
	c.inlineLabelOffsets = append(c.inlineLabelOffsets, labelSz)
	defer func() {
		c.inlineLabelOffsets = c.inlineLabelOffsets[:offSz]
		c.labelList = c.labelList[:labelSz]
	}()

	pkg := c.buildInfo.program.Package(f.pkg.Path())
	sig := c.typeOf(n.Fun).(*types.Signature)

	// When inlined call is used during global initialization
	// there is no func scope, thus this if.
	if c.scope == nil {
		c.scope = &funcScope{}
		c.scope.vars.newScope()
		defer func() { c.scope = nil }()
	}

	// Arguments need to be walked with the current scope,
	// while stored in the new.
	oldScope := c.scope.vars.locals
	c.scope.vars.newScope()
	newScope := make([]map[string]varInfo, len(c.scope.vars.locals))
	copy(newScope, c.scope.vars.locals)
	defer c.scope.vars.dropScope()

	hasVarArgs := !n.Ellipsis.IsValid()
	needPack := sig.Variadic() && hasVarArgs
	for i := range n.Args {
		c.scope.vars.locals = oldScope
		// true if normal arg or var arg is `slice...`
		needStore := i < sig.Params().Len()-1 || !sig.Variadic() || !hasVarArgs
		if !needStore {
			break
		}
		name := sig.Params().At(i).Name()
		if tv := c.typeAndValueOf(n.Args[i]); tv.Value != nil {
			c.scope.vars.locals = newScope
			c.scope.vars.addAlias(name, varLocal, unspecifiedVarIndex, tv)
			continue
		}
		if arg, ok := n.Args[i].(*ast.Ident); ok {
			// When function argument is variable or const, we may avoid
			// introducing additional variables for parameters.
			// This is done by providing additional alias to variable.
			if vi := c.scope.vars.getVarInfo(arg.Name); vi != nil {
				c.scope.vars.locals = newScope
				c.scope.vars.addAlias(name, vi.refType, vi.index, vi.tv)
				continue
			} else if arg.Name == "nil" {
				c.scope.vars.locals = newScope
				c.scope.vars.addAlias(name, varLocal, unspecifiedVarIndex, types.TypeAndValue{})
				continue
			} else if index, ok := c.globals[c.getIdentName("", arg.Name)]; ok {
				c.scope.vars.locals = newScope
				c.scope.vars.addAlias(name, varGlobal, index, types.TypeAndValue{})
				continue
			}
		}
		ast.Walk(c, n.Args[i])
		c.scope.vars.locals = newScope
		c.scope.newLocal(name)
		c.emitStoreVar("", name)
	}

	if needPack {
		// traverse variadic args and pack them
		// if they are provided directly i.e. without `...`
		c.scope.vars.locals = oldScope
		for i := sig.Params().Len() - 1; i < len(n.Args); i++ {
			ast.Walk(c, n.Args[i])
		}
		c.scope.vars.locals = newScope
		c.packVarArgs(n, sig)
		name := sig.Params().At(sig.Params().Len() - 1).Name()
		c.scope.newLocal(name)
		c.emitStoreVar("", name)
	}

	c.pkgInfoInline = append(c.pkgInfoInline, pkg)
	oldMap := c.importMap
	c.fillImportMap(f.file, pkg.Pkg)
	ast.Inspect(f.decl, c.scope.analyzeVoidCalls)
	ast.Walk(c, f.decl.Body)
	if c.scope.voidCalls[n] {
		for i := 0; i < f.decl.Type.Results.NumFields(); i++ {
			emit.Opcodes(c.prog.BinWriter, opcode.DROP)
		}
	}
	c.importMap = oldMap
	c.pkgInfoInline = c.pkgInfoInline[:len(c.pkgInfoInline)-1]
}
