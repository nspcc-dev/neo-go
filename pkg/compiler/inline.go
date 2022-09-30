package compiler

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// inlineCall inlines call of n for function represented by f.
// Call `f(a,b)` for definition `func f(x,y int)` is translated to block:
//
//	{
//	   x := a
//	   y := b
//	   <inline body of f directly>
//	}
func (c *codegen) inlineCall(f *funcScope, n *ast.CallExpr) {
	offSz := len(c.inlineContext)
	c.inlineContext = append(c.inlineContext, inlineContextSingle{
		labelOffset: len(c.labelList),
		returnLabel: c.newLabel(),
	})

	defer func() {
		c.labelList = c.labelList[:c.inlineContext[offSz].labelOffset]
		c.inlineContext = c.inlineContext[:offSz]
	}()

	pkg := c.packageCache[f.pkg.Path()]
	sig := c.typeOf(n.Fun).(*types.Signature)

	hasVarArgs := !n.Ellipsis.IsValid()
	eventParams := c.processStdlibCall(f, n.Args, !hasVarArgs)

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

	if f.decl.Recv != nil {
		c.scope.vars.locals = newScope
		name := f.decl.Recv.List[0].Names[0].Name
		c.scope.vars.addAlias(name, -1, unspecifiedVarIndex, &varContext{
			importMap: c.importMap,
			expr:      f.selector,
			scope:     oldScope,
		})
	}
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
			// In case of runtime.Notify, its arguments need to be converted to proper type.
			// i's initial value is 1 (variadic args start).
			if eventParams != nil && eventParams[i-1] != nil {
				c.emitConvert(*eventParams[i-1])
			}
		}
		c.scope.vars.locals = newScope
		c.packVarArgs(n, sig)
		name := sig.Params().At(sig.Params().Len() - 1).Name()
		c.scope.newLocal(name)
		c.emitStoreVar("", name)
	}

	c.pkgInfoInline = append(c.pkgInfoInline, pkg)
	oldMap := c.importMap
	oldDefers := c.scope.deferStack
	c.scope.deferStack = nil
	c.fillImportMap(f.file, pkg)
	ast.Inspect(f.decl, c.scope.analyzeVoidCalls)
	ast.Walk(c, f.decl.Body)
	c.setLabel(c.inlineContext[offSz].returnLabel)
	if c.scope.voidCalls[n] {
		for i := 0; i < f.decl.Type.Results.NumFields(); i++ {
			emit.Opcodes(c.prog.BinWriter, opcode.DROP)
		}
	}
	c.processDefers()
	c.scope.deferStack = oldDefers
	c.importMap = oldMap
	c.pkgInfoInline = c.pkgInfoInline[:len(c.pkgInfoInline)-1]
}

func (c *codegen) processStdlibCall(f *funcScope, args []ast.Expr, hasEllipsis bool) []*stackitem.Type {
	if f == nil {
		return nil
	}

	var eventParams []*stackitem.Type
	if f.pkg.Path() == interopPrefix+"/runtime" && (f.name == "Notify" || f.name == "Log") {
		eventParams = c.processNotify(f, args, hasEllipsis)
	}

	if f.pkg.Path() == interopPrefix+"/contract" && f.name == "Call" {
		c.processContractCall(f, args)
	}
	return eventParams
}

// processNotify checks whether notification emitting rules are met and returns expected
// notification signature if found.
func (c *codegen) processNotify(f *funcScope, args []ast.Expr, hasEllipsis bool) []*stackitem.Type {
	if c.scope != nil && c.isVerifyFunc(c.scope.decl) &&
		c.scope.pkg == c.mainPkg.Types && (c.buildInfo.options == nil || !c.buildInfo.options.NoEventsCheck) {
		c.prog.Err = fmt.Errorf("runtime.%s is not allowed in `Verify`", f.name)
		return nil
	}

	if f.name == "Log" {
		return nil
	}

	// Sometimes event name is stored in a var. Or sometimes event args are provided
	// via ellipses (`slice...`).
	// Skip in this case.  Also, don't enforce runtime.Notify parameters conversion.
	tv := c.typeAndValueOf(args[0])
	if tv.Value == nil || hasEllipsis {
		return nil
	}

	params := make([]string, 0, len(args[1:]))
	vParams := make([]*stackitem.Type, 0, len(args[1:]))
	for _, p := range args[1:] {
		st, vt, _ := c.scAndVMTypeFromExpr(p)
		params = append(params, st.String())
		vParams = append(vParams, &vt)
	}

	name := constant.StringVal(tv.Value)
	if len(name) > runtime.MaxEventNameLen {
		c.prog.Err = fmt.Errorf("event name '%s' should be less than %d",
			name, runtime.MaxEventNameLen)
		return nil
	}
	var eventFound bool
	if c.buildInfo.options != nil && c.buildInfo.options.ContractEvents != nil && !c.buildInfo.options.NoEventsCheck {
		for _, e := range c.buildInfo.options.ContractEvents {
			if e.Name == name && len(e.Parameters) == len(vParams) {
				eventFound = true
				for i, scParam := range e.Parameters {
					expectedType := scParam.Type.ConvertToStackitemType()
					// No need to cast if the desired type is unknown.
					if expectedType == stackitem.AnyT ||
						// Do not cast if desired type is Interop, the actual type is likely to be Any, leave the resolving to runtime.Notify.
						expectedType == stackitem.InteropT ||
						// No need to cast if actual parameter type matches the desired one.
						*vParams[i] == expectedType ||
						// expectedType doesn't contain Buffer anyway, but if actual variable type is Buffer,
						// then runtime.Notify will convert it to ByteArray automatically, thus no need to emit conversion code.
						(*vParams[i] == stackitem.BufferT && expectedType == stackitem.ByteArrayT) {
						vParams[i] = nil
					} else {
						// For other cases the conversion code will be emitted using vParams...
						vParams[i] = &expectedType
						// ...thus, update emitted notification info in advance.
						params[i] = scParam.Type.String()
					}
				}
			}
		}
	}
	c.emittedEvents[name] = append(c.emittedEvents[name], params)
	// Do not enforce perfect expected/actual events match on this step, the final
	// check wil be performed after compilation if --no-events option is off.
	if eventFound {
		return vParams
	}
	return nil
}

func (c *codegen) processContractCall(f *funcScope, args []ast.Expr) {
	var u util.Uint160

	// For stdlib calls it is `interop.Hash160(constHash)`
	// so we can determine hash at compile-time.
	ce, ok := args[0].(*ast.CallExpr)
	if ok && len(ce.Args) == 1 {
		// Ensure this is a type conversion, not a simple invoke.
		se, ok := ce.Fun.(*ast.SelectorExpr)
		if ok {
			name, _ := c.getFuncNameFromSelector(se)
			if _, ok := c.funcs[name]; !ok {
				value := c.typeAndValueOf(ce.Args[0]).Value
				if value != nil {
					s := constant.StringVal(value)
					copy(u[:], s) // constant must be big-endian
				}
			}
		}
	}

	value := c.typeAndValueOf(args[1]).Value
	if value == nil {
		return
	}

	method := constant.StringVal(value)

	value = c.typeAndValueOf(args[2]).Value
	if value == nil {
		return
	}

	flag, _ := constant.Uint64Val(value)
	c.appendInvokedContract(u, method, flag)
}

func (c *codegen) appendInvokedContract(u util.Uint160, method string, flag uint64) {
	currLst := c.invokedContracts[u]
	for _, m := range currLst {
		if m == method {
			return
		}
	}

	if flag&uint64(callflag.WriteStates|callflag.AllowNotify) != 0 {
		c.invokedContracts[u] = append(currLst, method)
	}
}

// hasCalls returns true if expression contains any calls.
// We uses this as a rough heuristic to determine if expression calculation
// has any side-effects.
func (c *codegen) hasCalls(expr ast.Expr) bool {
	var has bool
	ast.Inspect(expr, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !has && ok {
			isFunc := true
			fun, ok := ce.Fun.(*ast.Ident)
			if ok {
				_, isFunc = c.getFuncFromIdent(fun)
			} else {
				var sel *ast.SelectorExpr
				sel, ok = ce.Fun.(*ast.SelectorExpr)
				if ok {
					name, _ := c.getFuncNameFromSelector(sel)
					_, isFunc = c.funcs[name]
					fun = sel.Sel
				}
			}
			has = isFunc || fun.Obj != nil && (fun.Obj.Kind == ast.Var || fun.Obj.Kind == ast.Fun)
		}
		return !has
	})
	return has
}
