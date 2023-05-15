package smartcontract

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// Builder is used to create arbitrary scripts from the set of methods it provides.
// Each method emits some set of opcodes performing an action and (in most cases)
// returning a result. These chunks of code can be composed together to perform
// several actions in the same script (and therefore in the same transaction), but
// the end result (in terms of state changes and/or resulting items) of the script
// totally depends on what it contains and that's the responsibility of the Builder
// user. Builder is mostly used to create transaction scripts (also known as
// "entry scripts"), so the set of methods it exposes is tailored to this model
// of use and any calls emitted don't limit flags in any way (always use
// callflag.All).
//
// When using this API keep in mind that the resulting script can't be larger than
// 64K (transaction.MaxScriptLength) to be used as a transaction entry script and
// it can't have more than 2048 elements on the stack. Technically, this limits
// the number of calls that can be made to a lesser value because invocations use
// the same stack too (the exact number depends on methods and parameters).
//
// This API is not (and won't be) suitable to create complex scripts that use
// returned values as parameters to other calls or perform loops or do any other
// things that can be done in NeoVM. This hardly can be expressed in an API like
// this, so if you need more than that and if you're ready to work with bare
// NeoVM instructions please refer to [emit] and [opcode] packages.
type Builder struct {
	bw *io.BufBinWriter
}

// NewBuilder creates a new Builder instance.
func NewBuilder() *Builder {
	return &Builder{bw: io.NewBufBinWriter()}
}

// InvokeMethod is the most generic contract method invoker, the code it produces
// packs all of the arguments given into an array and calls some method of the
// contract. It accepts as parameters everything that emit.Array accepts. The
// correctness of this invocation (number and type of parameters) is out of scope
// of this method, as well as return value, if contract's method returns something
// this value just remains on the execution stack.
func (b *Builder) InvokeMethod(contract util.Uint160, method string, params ...any) {
	emit.AppCall(b.bw.BinWriter, contract, method, callflag.All, params...)
}

// Assert emits an ASSERT opcode that expects a Boolean value to be on the stack,
// checks if it's true and aborts the transaction if it's not.
func (b *Builder) Assert() {
	emit.Opcodes(b.bw.BinWriter, opcode.ASSERT)
}

// InvokeWithAssert emits an invocation of the method (see InvokeMethod) with
// an ASSERT after the invocation. The presumption is that the method called
// returns a Boolean value signalling the success or failure of the operation.
// This pattern is pretty common, NEP-11 or NEP-17 'transfer' methods do exactly
// that as well as NEO's 'vote'. The ASSERT then allow to simplify transaction
// status checking, if action is successful then transaction is successful as
// well, if it went wrong than whole transaction fails (ends with vmstate.FAULT).
func (b *Builder) InvokeWithAssert(contract util.Uint160, method string, params ...any) {
	b.InvokeMethod(contract, method, params...)
	b.Assert()
}

// Len returns the current length of the script. It's useful to perform script
// length checks (wrt transaction.MaxScriptLength limit) while building the
// script.
func (b *Builder) Len() int {
	return b.bw.Len()
}

// Script return current script, you can't use Builder after invoking this method
// unless you Reset it.
func (b *Builder) Script() ([]byte, error) {
	err := b.bw.Err
	return b.bw.Bytes(), err
}

// Reset resets the Builder, allowing to reuse the same script buffer (but
// previous script will be overwritten there).
func (b *Builder) Reset() {
	b.bw.Reset()
}
