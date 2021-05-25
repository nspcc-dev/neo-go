package compiler

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// inlineCall inlines call of n for function represented by f.
// Call `f(a,b)` for definition `func f(x,y int)` is translated to block:
//   {
//      x := a
//      y := b
//      <inline body of f directly>
//   }
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

	c.processNotify(f, n.Args)

	// When inlined call is used during global initialization
	// there is no func scope, thus this if.
	if c.scope == nil {
		c.scope = &funcScope{}
		c.scope.vars.newScope()
		defer func() {
			if cnt := c.scope.vars.localsCnt; cnt > c.globalInlineCount {
				c.globalInlineCount = cnt
			}
			c.scope = nil
		}()
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
		if !c.hasCalls(n.Args[i]) {
			// If argument contains no calls, we save context and traverse the expression
			// when argument is emitted.
			c.scope.vars.locals = newScope
			c.scope.vars.addAlias(name, -1, unspecifiedVarIndex, &varContext{
				importMap: c.importMap,
				expr:      n.Args[i],
				scope:     oldScope,
			})
			continue
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

func (c *codegen) processNotify(f *funcScope, args []ast.Expr) {
	if f != nil && f.pkg.Path() == interopPrefix+"/runtime" && f.name == "Notify" {
		// Sometimes event name is stored in a var.
		// Skip in this case.
		tv := c.typeAndValueOf(args[0])
		if tv.Value == nil {
			return
		}

		params := make([]string, 0, len(args[1:]))
		for _, p := range args[1:] {
			st, _ := c.scAndVMTypeFromExpr(p)
			params = append(params, st.String())
		}

		name := constant.StringVal(tv.Value)
		if len(name) > runtime.MaxEventNameLen {
			c.prog.Err = fmt.Errorf("event name '%s' should be less than %d",
				name, runtime.MaxEventNameLen)
			return
		}
		c.emittedEvents[name] = append(c.emittedEvents[name], params)
	}
}

// hasCalls returns true if expression contains any calls.
// We uses this as a rough heuristic to determine if expression calculation
// has any side-effects.
func (c *codegen) hasCalls(expr ast.Expr) bool {
	var has bool
	ast.Inspect(expr, func(n ast.Node) bool {
		_, ok := n.(*ast.CallExpr)
		if ok {
			has = true
		}
		return !has
	})
	return has
}
