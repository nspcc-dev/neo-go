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
type Builder struct {
	bw *io.BufBinWriter
}

// NewBuilder creates a new Builder instance.
func NewBuilder() *Builder {
	return &Builder{bw: io.NewBufBinWriter()}
}

// InvokeMethod is the most generic contract method invoker, the code it produces
// packs all of the arguments given into an array and calls some method of the
// contract. The correctness of this invocation (number and type of parameters) is
// out of scope of this method, as well as return value, if contract's method returns
// something this value just remains on the execution stack.
func (b *Builder) InvokeMethod(contract util.Uint160, method string, params ...interface{}) {
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
func (b *Builder) InvokeWithAssert(contract util.Uint160, method string, params ...interface{}) {
	b.InvokeMethod(contract, method, params...)
	b.Assert()
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
