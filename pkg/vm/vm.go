package vm

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/pkg/errors"
)

type errorAtInstruct struct {
	ip  int
	op  opcode.Opcode
	err interface{}
}

func (e *errorAtInstruct) Error() string {
	return fmt.Sprintf("error encountered at instruction %d (%s): %s", e.ip, e.op, e.err)
}

func newError(ip int, op opcode.Opcode, err interface{}) *errorAtInstruct {
	return &errorAtInstruct{ip: ip, op: op, err: err}
}

// StateMessage is a vm state message which could be used as additional info for example by cli.
type StateMessage string

const (
	// MaxArraySize is the maximum array size allowed in the VM.
	MaxArraySize = 1024

	// MaxItemSize is the maximum item size allowed in the VM.
	MaxItemSize = 1024 * 1024

	// MaxInvocationStackSize is the maximum size of an invocation stack.
	MaxInvocationStackSize = 1024

	// MaxBigIntegerSizeBits is the maximum size of BigInt item in bits.
	MaxBigIntegerSizeBits = 32 * 8

	// MaxStackSize is the maximum number of items allowed to be
	// on all stacks at once.
	MaxStackSize = 2 * 1024

	maxSHLArg = MaxBigIntegerSizeBits
)

// VM represents the virtual machine.
type VM struct {
	state State

	// callbacks to get interops.
	getInterop []InteropGetterFunc

	// callback to get interop price
	getPrice func(*VM, opcode.Opcode, []byte) util.Fixed8

	// callback to get scripts.
	getScript func(util.Uint160) ([]byte, bool)

	istack *Stack // invocation stack.
	estack *Stack // execution stack.
	astack *Stack // alt stack.

	// Hash to verify in CHECKSIG/CHECKMULTISIG.
	checkhash []byte

	itemCount map[StackItem]int
	size      int

	gasConsumed util.Fixed8
	gasLimit    util.Fixed8

	// Public keys cache.
	keys map[string]*keys.PublicKey
}

// New returns a new VM object ready to load .avm bytecode scripts.
func New() *VM {
	vm := &VM{
		getInterop: make([]InteropGetterFunc, 0, 3), // 3 functions is typical for our default usage.
		getScript:  nil,
		state:      haltState,
		istack:     NewStack("invocation"),

		itemCount: make(map[StackItem]int),
		keys:      make(map[string]*keys.PublicKey),
	}

	vm.estack = vm.newItemStack("evaluation")
	vm.astack = vm.newItemStack("alt")

	vm.RegisterInteropGetter(getDefaultVMInterop)
	return vm
}

func (v *VM) newItemStack(n string) *Stack {
	s := NewStack(n)
	s.size = &v.size
	s.itemCount = v.itemCount

	return s
}

// RegisterInteropGetter registers the given InteropGetterFunc into VM. There
// can be many interop getters and they're probed in LIFO order wrt their
// registration time.
func (v *VM) RegisterInteropGetter(f InteropGetterFunc) {
	v.getInterop = append(v.getInterop, f)
}

// SetPriceGetter registers the given PriceGetterFunc in v.
// f accepts vm's Context, current instruction and instruction parameter.
func (v *VM) SetPriceGetter(f func(*VM, opcode.Opcode, []byte) util.Fixed8) {
	v.getPrice = f
}

// GasConsumed returns the amount of GAS consumed during execution.
func (v *VM) GasConsumed() util.Fixed8 {
	return v.gasConsumed
}

// SetGasLimit sets maximum amount of gas which v can spent.
// If max <= 0, no limit is imposed.
func (v *VM) SetGasLimit(max util.Fixed8) {
	v.gasLimit = max
}

// Estack returns the evaluation stack so interop hooks can utilize this.
func (v *VM) Estack() *Stack {
	return v.estack
}

// Astack returns the alt stack so interop hooks can utilize this.
func (v *VM) Astack() *Stack {
	return v.astack
}

// Istack returns the invocation stack so interop hooks can utilize this.
func (v *VM) Istack() *Stack {
	return v.istack
}

// SetPublicKeys sets internal key cache to the specified value (note
// that it doesn't copy them).
func (v *VM) SetPublicKeys(keys map[string]*keys.PublicKey) {
	v.keys = keys
}

// GetPublicKeys returns internal key cache (note that it doesn't copy it).
func (v *VM) GetPublicKeys() map[string]*keys.PublicKey {
	return v.keys
}

// LoadArgs loads in the arguments used in the Mian entry point.
func (v *VM) LoadArgs(method []byte, args []StackItem) {
	if len(args) > 0 {
		v.estack.PushVal(args)
	}
	if method != nil {
		v.estack.PushVal(method)
	}
}

// PrintOps prints the opcodes of the current loaded program to stdout.
func (v *VM) PrintOps() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "INDEX\tOPCODE\tPARAMETER\t")
	realctx := v.Context()
	ctx := realctx.Copy()
	ctx.ip = 0
	ctx.nextip = 0
	for {
		cursor := ""
		instr, parameter, err := ctx.Next()
		if ctx.ip == realctx.ip {
			cursor = "<<"
		}
		if err != nil {
			fmt.Fprintf(w, "%d\t%s\tERROR: %s\t%s\n", ctx.ip, instr, err, cursor)
			break
		}
		var desc = ""
		if parameter != nil {
			switch instr {
			case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.CALL:
				offset := int16(binary.LittleEndian.Uint16(parameter))
				desc = fmt.Sprintf("%d (%d/%x)", ctx.ip+int(offset), offset, parameter)
			case opcode.SYSCALL:
				desc = fmt.Sprintf("%q", parameter)
			case opcode.APPCALL, opcode.TAILCALL:
				desc = fmt.Sprintf("%x", parameter)
			default:
				if utf8.Valid(parameter) {
					desc = fmt.Sprintf("%x (%q)", parameter, parameter)
				} else {
					desc = fmt.Sprintf("%x", parameter)
				}
			}
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", ctx.ip, instr, desc, cursor)
		if ctx.nextip >= len(ctx.prog) {
			break
		}
	}
	w.Flush()
}

// AddBreakPoint adds a breakpoint to the current context.
func (v *VM) AddBreakPoint(n int) {
	ctx := v.Context()
	ctx.breakPoints = append(ctx.breakPoints, n)
}

// AddBreakPointRel adds a breakpoint relative to the current
// instruction pointer.
func (v *VM) AddBreakPointRel(n int) {
	ctx := v.Context()
	v.AddBreakPoint(ctx.ip + n)
}

// LoadFile loads a program from the given path, ready to execute it.
func (v *VM) LoadFile(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	v.Load(b)
	return nil
}

// Load initializes the VM with the program given.
func (v *VM) Load(prog []byte) {
	// Clear all stacks and state, it could be a reload.
	v.istack.Clear()
	v.estack.Clear()
	v.astack.Clear()
	v.state = noneState
	v.gasConsumed = 0
	v.LoadScript(prog)
}

// LoadScript loads a script from the internal script table. It
// will immediately push a new context created from this script to
// the invocation stack and starts executing it.
func (v *VM) LoadScript(b []byte) {
	ctx := NewContext(b)
	ctx.estack = v.estack
	ctx.astack = v.astack
	v.istack.PushVal(ctx)
}

// loadScriptWithHash if similar to the LoadScript method, but it also loads
// given script hash directly into the Context to avoid its recalculations. It's
// up to user of this function to make sure the script and hash match each other.
func (v *VM) loadScriptWithHash(b []byte, hash util.Uint160, hasDynamicInvoke bool) {
	v.LoadScript(b)
	ctx := v.Context()
	ctx.scriptHash = hash
	ctx.hasDynamicInvoke = hasDynamicInvoke
}

// Context returns the current executed context. Nil if there is no context,
// which implies no program is loaded.
func (v *VM) Context() *Context {
	if v.istack.Len() == 0 {
		return nil
	}
	return v.istack.Peek(0).Value().(*Context)
}

// PopResult is used to pop the first item of the evaluation stack. This allows
// us to test compiler and vm in a bi-directional way.
func (v *VM) PopResult() interface{} {
	e := v.estack.Pop()
	if e != nil {
		return e.Value()
	}
	return nil
}

// Stack returns json formatted representation of the given stack.
func (v *VM) Stack(n string) string {
	var s *Stack
	if n == "astack" {
		s = v.astack
	}
	if n == "istack" {
		s = v.istack
	}
	if n == "estack" {
		s = v.estack
	}
	b, _ := json.MarshalIndent(s.ToContractParameters(), "", "    ")
	return string(b)
}

// State returns string representation of the state for the VM.
func (v *VM) State() string {
	return v.state.String()
}

// Ready returns true if the VM ready to execute the loaded program.
// Will return false if no program is loaded.
func (v *VM) Ready() bool {
	return v.istack.Len() > 0
}

// Run starts the execution of the loaded program.
func (v *VM) Run() error {
	if !v.Ready() {
		v.state = faultState
		return errors.New("no program loaded")
	}

	if v.state.HasFlag(faultState) {
		// VM already ran something and failed, in general its state is
		// undefined in this case so we can't run anything.
		return errors.New("VM has failed")
	}
	// haltState (the default) or breakState are safe to continue.
	v.state = noneState
	for {
		// check for breakpoint before executing the next instruction
		ctx := v.Context()
		if ctx != nil && ctx.atBreakPoint() {
			v.state |= breakState
		}
		switch {
		case v.state.HasFlag(faultState):
			// Should be caught and reported already by the v.Step(),
			// but we're checking here anyway just in case.
			return errors.New("VM has failed")
		case v.state.HasFlag(haltState), v.state.HasFlag(breakState):
			// Normal exit from this loop.
			return nil
		case v.state == noneState:
			if err := v.Step(); err != nil {
				return err
			}
		default:
			v.state = faultState
			return errors.New("unknown state")
		}
	}
}

// Step 1 instruction in the program.
func (v *VM) Step() error {
	ctx := v.Context()
	op, param, err := ctx.Next()
	if err != nil {
		v.state = faultState
		return newError(ctx.ip, op, err)
	}
	return v.execute(ctx, op, param)
}

// StepInto behaves the same as “step over” in case if the line does not contain a function. Otherwise
// the debugger will enter the called function and continue line-by-line debugging there.
func (v *VM) StepInto() error {
	ctx := v.Context()

	if ctx == nil {
		v.state |= haltState
	}

	if v.HasStopped() {
		return nil
	}

	if ctx != nil && ctx.prog != nil {
		op, param, err := ctx.Next()
		if err != nil {
			v.state = faultState
			return newError(ctx.ip, op, err)
		}
		vErr := v.execute(ctx, op, param)
		if vErr != nil {
			return vErr
		}
	}

	cctx := v.Context()
	if cctx != nil && cctx.atBreakPoint() {
		v.state = breakState
	}
	return nil
}

// StepOut takes the debugger to the line where the current function was called.
func (v *VM) StepOut() error {
	var err error
	if v.state == breakState {
		v.state = noneState
	} else {
		v.state = breakState
	}

	expSize := v.istack.len
	for v.state == noneState && v.istack.len >= expSize {
		err = v.StepInto()
	}
	return err
}

// StepOver takes the debugger to the line that will step over a given line.
// If the line contains a function the function will be executed and the result returned without debugging each line.
func (v *VM) StepOver() error {
	var err error
	if v.HasStopped() {
		return err
	}

	if v.state == breakState {
		v.state = noneState
	} else {
		v.state = breakState
	}

	expSize := v.istack.len
	for {
		err = v.StepInto()
		if !(v.state == noneState && v.istack.len > expSize) {
			break
		}
	}

	if v.state == noneState {
		v.state = breakState
	}

	return err
}

// HasFailed returns whether VM is in the failed state now. Usually used to
// check status after Run.
func (v *VM) HasFailed() bool {
	return v.state.HasFlag(faultState)
}

// HasStopped returns whether VM is in Halt or Failed state.
func (v *VM) HasStopped() bool {
	return v.state.HasFlag(haltState) || v.state.HasFlag(faultState)
}

// HasHalted returns whether VM is in Halt state.
func (v *VM) HasHalted() bool {
	return v.state.HasFlag(haltState)
}

// AtBreakpoint returns whether VM is at breakpoint.
func (v *VM) AtBreakpoint() bool {
	return v.state.HasFlag(breakState)
}

// SetCheckedHash sets checked hash for CHECKSIG and CHECKMULTISIG instructions.
func (v *VM) SetCheckedHash(h []byte) {
	v.checkhash = make([]byte, len(h))
	copy(v.checkhash, h)
}

// SetScriptGetter sets the script getter for CALL instructions.
func (v *VM) SetScriptGetter(gs func(util.Uint160) ([]byte, bool)) {
	v.getScript = gs
}

// GetInteropID converts instruction parameter to an interop ID.
func GetInteropID(parameter []byte) uint32 {
	return binary.LittleEndian.Uint32(parameter)
}

// GetInteropByID returns interop function together with price.
// Registered callbacks are checked in LIFO order.
func (v *VM) GetInteropByID(id uint32) *InteropFuncPrice {
	for i := len(v.getInterop) - 1; i >= 0; i-- {
		if ifunc := v.getInterop[i](id); ifunc != nil {
			return ifunc
		}
	}

	return nil
}

// execute performs an instruction cycle in the VM. Acting on the instruction (opcode).
func (v *VM) execute(ctx *Context, op opcode.Opcode, parameter []byte) (err error) {
	// Instead of polluting the whole VM logic with error handling, we will recover
	// each panic at a central point, putting the VM in a fault state and setting error.
	defer func() {
		if errRecover := recover(); errRecover != nil {
			v.state = faultState
			err = newError(ctx.ip, op, errRecover)
		} else if v.size > MaxStackSize {
			v.state = faultState
			err = newError(ctx.ip, op, "stack is too big")
		}
	}()

	if v.getPrice != nil && ctx.ip < len(ctx.prog) {
		v.gasConsumed += v.getPrice(v, op, parameter)
		if v.gasLimit > 0 && v.gasConsumed > v.gasLimit {
			panic("gas limit is exceeded")
		}
	}

	switch op {
	case opcode.APPCALL, opcode.TAILCALL:
		isZero := true
		for i := range parameter {
			if parameter[i] != 0 {
				isZero = false
				break
			}
		}
		if !isZero {
			break
		}

		parameter = v.estack.Pop().Bytes()
		if !ctx.hasDynamicInvoke {
			panic("contract is not allowed to make dynamic invocations")
		}
	}

	if op <= opcode.PUSHINT256 {
		v.estack.PushVal(emit.BytesToInt(parameter))
		return
	}

	switch op {
	case opcode.PUSHM1, opcode.PUSH1, opcode.PUSH2, opcode.PUSH3,
		opcode.PUSH4, opcode.PUSH5, opcode.PUSH6, opcode.PUSH7,
		opcode.PUSH8, opcode.PUSH9, opcode.PUSH10, opcode.PUSH11,
		opcode.PUSH12, opcode.PUSH13, opcode.PUSH14, opcode.PUSH15,
		opcode.PUSH16:
		val := int(op) - int(opcode.PUSH1) + 1
		v.estack.PushVal(val)

	case opcode.OLDPUSH1:
		// FIXME remove this after Issue transactions will be removed
		v.estack.PushVal(1)

	case opcode.PUSH0:
		v.estack.PushVal([]byte{})

	case opcode.PUSHDATA1, opcode.PUSHDATA2, opcode.PUSHDATA4:
		v.estack.PushVal(parameter)

	case opcode.PUSHNULL:
		v.estack.PushVal(NullItem{})

	case opcode.ISNULL:
		res := v.estack.Pop().value.Equals(NullItem{})
		v.estack.PushVal(res)

	case opcode.ISTYPE:
		res := v.estack.Pop().Item()
		v.estack.PushVal(res.Type() == StackItemType(parameter[0]))

	// Stack operations.
	case opcode.TOALTSTACK:
		v.astack.Push(v.estack.Pop())

	case opcode.FROMALTSTACK:
		v.estack.Push(v.astack.Pop())

	case opcode.DUPFROMALTSTACK:
		v.estack.Push(v.astack.Dup(0))

	case opcode.DUP:
		v.estack.Push(v.estack.Dup(0))

	case opcode.SWAP:
		err := v.estack.Swap(1, 0)
		if err != nil {
			panic(err.Error())
		}

	case opcode.TUCK:
		a := v.estack.Dup(0)
		if a == nil {
			panic("no top-level element found")
		}
		if v.estack.Len() < 2 {
			panic("can't TUCK with a one-element stack")
		}
		v.estack.InsertAt(a, 2)

	case opcode.CAT:
		b := v.estack.Pop().Bytes()
		a := v.estack.Pop().Bytes()
		if l := len(a) + len(b); l > MaxItemSize {
			panic(fmt.Sprintf("too big item: %d", l))
		}
		ab := append(a, b...)
		v.estack.PushVal(ab)

	case opcode.SUBSTR:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		o := int(v.estack.Pop().BigInt().Int64())
		if o < 0 {
			panic("negative index")
		}
		s := v.estack.Pop().Bytes()
		last := l + o
		if last > len(s) {
			panic("invalid offset")
		}
		v.estack.PushVal(s[o:last])

	case opcode.LEFT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		if t := len(s); l > t {
			l = t
		}
		v.estack.PushVal(s[:l])

	case opcode.RIGHT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		v.estack.PushVal(s[len(s)-l:])

	case opcode.XDROP:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("invalid length")
		}
		e := v.estack.RemoveAt(n)
		if e == nil {
			panic("bad index")
		}

	case opcode.XSWAP:
		n := int(v.estack.Pop().BigInt().Int64())
		err := v.estack.Swap(n, 0)
		if err != nil {
			panic(err.Error())
		}

	case opcode.XTUCK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n <= 0 {
			panic("XTUCK: invalid length")
		}
		a := v.estack.Dup(0)
		if a == nil {
			panic("no top-level element found")
		}
		if n > v.estack.Len() {
			panic("can't push to the position specified")
		}
		v.estack.InsertAt(a, n)

	case opcode.ROT:
		err := v.estack.Roll(2)
		if err != nil {
			panic(err.Error())
		}

	case opcode.DEPTH:
		v.estack.PushVal(v.estack.Len())

	case opcode.NIP:
		elem := v.estack.RemoveAt(1)
		if elem == nil {
			panic("no second element found")
		}

	case opcode.OVER:
		a := v.estack.Dup(1)
		if a == nil {
			panic("no second element found")
		}
		v.estack.Push(a)

	case opcode.PICK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("negative stack item returned")
		}
		a := v.estack.Dup(n)
		if a == nil {
			panic("no nth element found")
		}
		v.estack.Push(a)

	case opcode.ROLL:
		n := int(v.estack.Pop().BigInt().Int64())
		err := v.estack.Roll(n)
		if err != nil {
			panic(err.Error())
		}

	case opcode.DROP:
		if v.estack.Len() < 1 {
			panic("stack is too small")
		}
		v.estack.Pop()

	case opcode.EQUAL:
		b := v.estack.Pop()
		if b == nil {
			panic("no top-level element found")
		}
		a := v.estack.Pop()
		if a == nil {
			panic("no second-to-the-top element found")
		}
		v.estack.PushVal(a.value.Equals(b.value))

	// Bit operations.
	case opcode.INVERT:
		// inplace
		e := v.estack.Peek(0)
		i := e.BigInt()
		e.value = makeStackItem(i.Not(i))

	case opcode.AND:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).And(b, a))

	case opcode.OR:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Or(b, a))

	case opcode.XOR:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Xor(b, a))

	// Numeric operations.
	case opcode.ADD:
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)
		b := v.estack.Pop().BigInt()
		v.checkBigIntSize(b)

		c := new(big.Int).Add(a, b)
		v.checkBigIntSize(c)
		v.estack.PushVal(c)

	case opcode.SUB:
		b := v.estack.Pop().BigInt()
		v.checkBigIntSize(b)
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)

		c := new(big.Int).Sub(a, b)
		v.checkBigIntSize(c)
		v.estack.PushVal(c)

	case opcode.DIV:
		b := v.estack.Pop().BigInt()
		v.checkBigIntSize(b)
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)

		v.estack.PushVal(new(big.Int).Quo(a, b))

	case opcode.MUL:
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)
		b := v.estack.Pop().BigInt()
		v.checkBigIntSize(b)

		c := new(big.Int).Mul(a, b)
		v.checkBigIntSize(c)
		v.estack.PushVal(c)

	case opcode.MOD:
		b := v.estack.Pop().BigInt()
		v.checkBigIntSize(b)
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)

		v.estack.PushVal(new(big.Int).Rem(a, b))

	case opcode.SHL, opcode.SHR:
		b := v.estack.Pop().BigInt().Int64()
		if b == 0 {
			return
		} else if b < 0 || b > maxSHLArg {
			panic(fmt.Sprintf("operand must be between %d and %d", 0, maxSHLArg))
		}
		a := v.estack.Pop().BigInt()
		v.checkBigIntSize(a)

		var item big.Int
		if op == opcode.SHL {
			item.Lsh(a, uint(b))
		} else {
			item.Rsh(a, uint(b))
		}

		v.checkBigIntSize(&item)
		v.estack.PushVal(&item)

	case opcode.BOOLAND:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a && b)

	case opcode.BOOLOR:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a || b)

	case opcode.NUMEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == 0)

	case opcode.NUMNOTEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) != 0)

	case opcode.LT:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == -1)

	case opcode.GT:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == 1)

	case opcode.LTE:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) <= 0)

	case opcode.GTE:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) >= 0)

	case opcode.MIN:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		val := a
		if a.Cmp(b) == 1 {
			val = b
		}
		v.estack.PushVal(val)

	case opcode.MAX:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		val := a
		if a.Cmp(b) == -1 {
			val = b
		}
		v.estack.PushVal(val)

	case opcode.WITHIN:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(x) <= 0 && x.Cmp(b) == -1)

	case opcode.INC:
		x := v.estack.Pop().BigInt()
		a := new(big.Int).Add(x, big.NewInt(1))
		v.checkBigIntSize(a)
		v.estack.PushVal(a)

	case opcode.DEC:
		x := v.estack.Pop().BigInt()
		a := new(big.Int).Sub(x, big.NewInt(1))
		v.checkBigIntSize(a)
		v.estack.PushVal(a)

	case opcode.SIGN:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Sign())

	case opcode.NEGATE:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Neg(x))

	case opcode.ABS:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Abs(x))

	case opcode.NOT:
		x := v.estack.Pop().Bool()
		v.estack.PushVal(!x)

	case opcode.NZ:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Cmp(big.NewInt(0)) != 0)

	// Object operations
	case opcode.NEWARRAY0:
		v.estack.PushVal(&ArrayItem{[]StackItem{}})

	case opcode.NEWARRAY:
		item := v.estack.Pop()
		switch t := item.value.(type) {
		case *StructItem:
			arr := make([]StackItem, len(t.value))
			copy(arr, t.value)
			v.estack.PushVal(&ArrayItem{arr})
		case *ArrayItem:
			v.estack.PushVal(t)
		default:
			n := item.BigInt().Int64()
			if n > MaxArraySize {
				panic("too long array")
			}
			items := makeArrayOfFalses(int(n))
			v.estack.PushVal(&ArrayItem{items})
		}

	case opcode.NEWSTRUCT0:
		v.estack.PushVal(&StructItem{[]StackItem{}})

	case opcode.NEWSTRUCT:
		item := v.estack.Pop()
		switch t := item.value.(type) {
		case *ArrayItem:
			arr := make([]StackItem, len(t.value))
			copy(arr, t.value)
			v.estack.PushVal(&StructItem{arr})
		case *StructItem:
			v.estack.PushVal(t)
		default:
			n := item.BigInt().Int64()
			if n > MaxArraySize {
				panic("too long struct")
			}
			items := makeArrayOfFalses(int(n))
			v.estack.PushVal(&StructItem{items})
		}

	case opcode.APPEND:
		itemElem := v.estack.Pop()
		arrElem := v.estack.Pop()

		val := cloneIfStruct(itemElem.value)

		switch t := arrElem.value.(type) {
		case *ArrayItem:
			arr := t.Value().([]StackItem)
			if len(arr) >= MaxArraySize {
				panic("too long array")
			}
			arr = append(arr, val)
			t.value = arr
		case *StructItem:
			arr := t.Value().([]StackItem)
			if len(arr) >= MaxArraySize {
				panic("too long struct")
			}
			arr = append(arr, val)
			t.value = arr
		default:
			panic("APPEND: not of underlying type Array")
		}

		v.estack.updateSizeAdd(val)

	case opcode.PACK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 || n > v.estack.Len() || n > MaxArraySize {
			panic("OPACK: invalid length")
		}

		items := make([]StackItem, n)
		for i := 0; i < n; i++ {
			items[i] = v.estack.Pop().value
		}

		v.estack.PushVal(items)

	case opcode.UNPACK:
		a := v.estack.Pop().Array()
		l := len(a)
		for i := l - 1; i >= 0; i-- {
			v.estack.PushVal(a[i])
		}
		v.estack.PushVal(l)

	case opcode.PICKITEM:
		key := v.estack.Pop()
		validateMapKey(key)

		obj := v.estack.Pop()
		index := int(key.BigInt().Int64())

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *ArrayItem, *StructItem:
			arr := t.Value().([]StackItem)
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index].Dup()
			v.estack.PushVal(item)
		case *MapItem:
			index := t.Index(key.Item())
			if index < 0 {
				panic("invalid key")
			}
			v.estack.Push(&Element{value: t.value[index].Value.Dup()})
		default:
			arr := obj.Bytes()
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index]
			v.estack.PushVal(int(item))
		}

	case opcode.SETITEM:
		item := v.estack.Pop().value
		key := v.estack.Pop()
		validateMapKey(key)

		obj := v.estack.Pop()

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *ArrayItem, *StructItem:
			arr := t.Value().([]StackItem)
			index := int(key.BigInt().Int64())
			if index < 0 || index >= len(arr) {
				panic("SETITEM: invalid index")
			}
			v.estack.updateSizeRemove(arr[index])
			arr[index] = item
			v.estack.updateSizeAdd(arr[index])
		case *MapItem:
			if t.Has(key.value) {
				v.estack.updateSizeRemove(item)
			} else if len(t.value) >= MaxArraySize {
				panic("too big map")
			}
			t.Add(key.value, item)
			v.estack.updateSizeAdd(item)

		default:
			panic(fmt.Sprintf("SETITEM: invalid item type %s", t))
		}

	case opcode.REVERSEITEMS:
		a := v.estack.Pop().Array()
		if len(a) > 1 {
			for i, j := 0, len(a)-1; i <= j; i, j = i+1, j-1 {
				a[i], a[j] = a[j], a[i]
			}
		}
	case opcode.REMOVE:
		key := v.estack.Pop()
		validateMapKey(key)

		elem := v.estack.Pop()
		switch t := elem.value.(type) {
		case *ArrayItem:
			a := t.value
			k := int(key.BigInt().Int64())
			if k < 0 || k >= len(a) {
				panic("REMOVE: invalid index")
			}
			v.estack.updateSizeRemove(a[k])
			a = append(a[:k], a[k+1:]...)
			t.value = a
		case *StructItem:
			a := t.value
			k := int(key.BigInt().Int64())
			if k < 0 || k >= len(a) {
				panic("REMOVE: invalid index")
			}
			v.estack.updateSizeRemove(a[k])
			a = append(a[:k], a[k+1:]...)
			t.value = a
		case *MapItem:
			index := t.Index(key.Item())
			// NEO 2.0 doesn't error on missing key.
			if index >= 0 {
				v.estack.updateSizeRemove(t.value[index].Value)
				t.Drop(index)
			}
		default:
			panic("REMOVE: invalid type")
		}

	case opcode.CLEARITEMS:
		elem := v.estack.Pop()
		switch t := elem.value.(type) {
		case *ArrayItem:
			for _, item := range t.value {
				v.estack.updateSizeRemove(item)
			}
			t.value = t.value[:0]
		case *StructItem:
			for _, item := range t.value {
				v.estack.updateSizeRemove(item)
			}
			t.value = t.value[:0]
		case *MapItem:
			for i := range t.value {
				v.estack.updateSizeRemove(t.value[i].Value)
			}
			t.value = t.value[:0]
		default:
			panic("CLEARITEMS: invalid type")
		}

	case opcode.SIZE:
		elem := v.estack.Pop()
		// Cause there is no native (byte) item type here, hence we need to check
		// the type of the item for array size operations.
		switch t := elem.Value().(type) {
		case []StackItem:
			v.estack.PushVal(len(t))
		case []MapElement:
			v.estack.PushVal(len(t))
		default:
			v.estack.PushVal(len(elem.Bytes()))
		}

	case opcode.JMP, opcode.JMPL, opcode.JMPIF, opcode.JMPIFL, opcode.JMPIFNOT, opcode.JMPIFNOTL,
		opcode.JMPEQ, opcode.JMPEQL, opcode.JMPNE, opcode.JMPNEL,
		opcode.JMPGT, opcode.JMPGTL, opcode.JMPGE, opcode.JMPGEL,
		opcode.JMPLT, opcode.JMPLTL, opcode.JMPLE, opcode.JMPLEL:
		offset := v.getJumpOffset(ctx, parameter, 0)
		cond := true
		switch op {
		case opcode.JMP, opcode.JMPL:
		case opcode.JMPIF, opcode.JMPIFL, opcode.JMPIFNOT, opcode.JMPIFNOTL:
			cond = v.estack.Pop().Bool() == (op == opcode.JMPIF || op == opcode.JMPIFL)
		default:
			b := v.estack.Pop().BigInt()
			a := v.estack.Pop().BigInt()
			cond = getJumpCondition(op, a, b)
		}

		v.jumpIf(ctx, offset, cond)

	case opcode.CALL, opcode.CALLL:
		v.checkInvocationStackSize()

		newCtx := ctx.Copy()
		newCtx.rvcount = -1
		v.istack.PushVal(newCtx)

		offset := v.getJumpOffset(newCtx, parameter, 0)
		v.jumpIf(newCtx, offset, true)

	case opcode.SYSCALL:
		interopID := GetInteropID(parameter)
		ifunc := v.GetInteropByID(interopID)

		if ifunc == nil {
			panic(fmt.Sprintf("interop hook (%q/0x%x) not registered", parameter, interopID))
		}
		if err := ifunc.Func(v); err != nil {
			panic(fmt.Sprintf("failed to invoke syscall: %s", err))
		}

	case opcode.APPCALL, opcode.TAILCALL:
		if v.getScript == nil {
			panic("no getScript callback is set up")
		}

		if op == opcode.APPCALL {
			v.checkInvocationStackSize()
		}

		hash, err := util.Uint160DecodeBytesBE(parameter)
		if err != nil {
			panic(err)
		}

		script, hasDynamicInvoke := v.getScript(hash)
		if script == nil {
			panic("could not find script")
		}

		if op == opcode.TAILCALL {
			_ = v.istack.Pop()
		}

		v.loadScriptWithHash(script, hash, hasDynamicInvoke)

	case opcode.RET:
		oldCtx := v.istack.Pop().Value().(*Context)
		rvcount := oldCtx.rvcount
		oldEstack := v.estack

		if rvcount > 0 && oldEstack.Len() < rvcount {
			panic("missing some return elements")
		}
		if v.istack.Len() == 0 {
			v.state = haltState
			break
		}

		newEstack := v.Context().estack
		if oldEstack != newEstack {
			if rvcount < 0 {
				rvcount = oldEstack.Len()
			}
			for i := rvcount; i > 0; i-- {
				elem := oldEstack.RemoveAt(i - 1)
				newEstack.Push(elem)
			}
			v.estack = newEstack
			v.astack = v.Context().astack
		}

	case opcode.NEWMAP:
		v.estack.Push(&Element{value: NewMapItem()})

	case opcode.KEYS:
		item := v.estack.Pop()
		if item == nil {
			panic("no argument")
		}

		m, ok := item.value.(*MapItem)
		if !ok {
			panic("not a Map")
		}

		arr := make([]StackItem, 0, len(m.value))
		for k := range m.value {
			arr = append(arr, m.value[k].Key.Dup())
		}
		v.estack.PushVal(arr)

	case opcode.VALUES:
		item := v.estack.Pop()
		if item == nil {
			panic("no argument")
		}

		var arr []StackItem
		switch t := item.value.(type) {
		case *ArrayItem, *StructItem:
			src := t.Value().([]StackItem)
			arr = make([]StackItem, len(src))
			for i := range src {
				arr[i] = cloneIfStruct(src[i])
			}
		case *MapItem:
			arr = make([]StackItem, 0, len(t.value))
			for k := range t.value {
				arr = append(arr, cloneIfStruct(t.value[k].Value))
			}
		default:
			panic("not a Map, Array or Struct")
		}

		v.estack.PushVal(arr)

	case opcode.HASKEY:
		key := v.estack.Pop()
		validateMapKey(key)

		c := v.estack.Pop()
		if c == nil {
			panic("no value found")
		}
		switch t := c.value.(type) {
		case *ArrayItem, *StructItem:
			index := key.BigInt().Int64()
			if index < 0 {
				panic("negative index")
			}
			v.estack.PushVal(index < int64(len(c.Array())))
		case *MapItem:
			v.estack.PushVal(t.Has(key.Item()))
		default:
			panic("wrong collection type")
		}

	// Cryptographic operations.
	case opcode.SHA1:
		b := v.estack.Pop().Bytes()
		sha := sha1.New()
		sha.Write(b)
		v.estack.PushVal(sha.Sum(nil))

	case opcode.SHA256:
		b := v.estack.Pop().Bytes()
		v.estack.PushVal(hash.Sha256(b).BytesBE())

	case opcode.NOP:
		// unlucky ^^

	case opcode.THROW:
		panic("THROW")

	case opcode.THROWIFNOT:
		if !v.estack.Pop().Bool() {
			panic("THROWIFNOT")
		}

	default:
		panic(fmt.Sprintf("unknown opcode %s", op.String()))
	}
	return
}

// getJumpCondition performs opcode specific comparison of a and b
func getJumpCondition(op opcode.Opcode, a, b *big.Int) bool {
	cmp := a.Cmp(b)
	switch op {
	case opcode.JMPEQ, opcode.JMPEQL:
		return cmp == 0
	case opcode.JMPNE, opcode.JMPNEL:
		return cmp != 0
	case opcode.JMPGT, opcode.JMPGTL:
		return cmp > 0
	case opcode.JMPGE, opcode.JMPGEL:
		return cmp >= 0
	case opcode.JMPLT, opcode.JMPLTL:
		return cmp < 0
	case opcode.JMPLE, opcode.JMPLEL:
		return cmp <= 0
	default:
		panic(fmt.Sprintf("invalid JMP* opcode: %s", op))
	}
}

// jumpIf performs jump to offset if cond is true.
func (v *VM) jumpIf(ctx *Context, offset int, cond bool) {
	if cond {
		ctx.nextip = offset
	}
}

// getJumpOffset returns instruction number in a current context
// to a which JMP should be performed.
// parameter should have length either 1 or 4 and
// is interpreted as little-endian.
func (v *VM) getJumpOffset(ctx *Context, parameter []byte, mod int) int {
	var rOffset int32
	switch l := len(parameter); l {
	case 1:
		rOffset = int32(int8(parameter[0]))
	case 4:
		rOffset = int32(binary.LittleEndian.Uint32(parameter))
	default:
		panic(fmt.Sprintf("invalid JMP* parameter length: %d", l))
	}
	offset := ctx.ip + int(rOffset) + mod
	if offset < 0 || offset > len(ctx.prog) {
		panic(fmt.Sprintf("JMP: invalid offset %d ip at %d", offset, ctx.ip))
	}

	return offset
}

// CheckMultisigPar checks if sigs contains sufficient valid signatures.
func CheckMultisigPar(v *VM, h []byte, pkeys [][]byte, sigs [][]byte) bool {
	if len(sigs) == 1 {
		return checkMultisig1(v, h, pkeys, sigs[0])
	}

	k1, k2 := 0, len(pkeys)-1
	s1, s2 := 0, len(sigs)-1

	type task struct {
		pub    *keys.PublicKey
		signum int
	}

	type verify struct {
		ok     bool
		signum int
	}

	worker := func(ch <-chan task, result chan verify) {
		for {
			t, ok := <-ch
			if !ok {
				return
			}

			result <- verify{
				signum: t.signum,
				ok:     t.pub.Verify(sigs[t.signum], h),
			}
		}
	}

	const workerCount = 3
	tasks := make(chan task, 2)
	results := make(chan verify, len(sigs))
	for i := 0; i < workerCount; i++ {
		go worker(tasks, results)
	}

	tasks <- task{pub: v.bytesToPublicKey(pkeys[k1]), signum: s1}
	tasks <- task{pub: v.bytesToPublicKey(pkeys[k2]), signum: s2}

	sigok := true
	taskCount := 2

loop:
	for r := range results {
		goingForward := true

		taskCount--
		if r.signum == s2 {
			goingForward = false
		}
		if k1+1 == k2 {
			sigok = r.ok && s1+1 == s2
			if taskCount != 0 && sigok {
				continue
			}
			break loop
		} else if r.ok {
			if s1+1 == s2 {
				if taskCount != 0 && sigok {
					continue
				}
				break loop
			}
			if goingForward {
				s1++
			} else {
				s2--
			}
		}

		var nextSig, nextKey int
		if goingForward {
			k1++
			nextSig = s1
			nextKey = k1
		} else {
			k2--
			nextSig = s2
			nextKey = k2
		}
		taskCount++
		tasks <- task{pub: v.bytesToPublicKey(pkeys[nextKey]), signum: nextSig}
	}

	close(tasks)

	return sigok
}

func checkMultisig1(v *VM, h []byte, pkeys [][]byte, sig []byte) bool {
	for i := range pkeys {
		pkey := v.bytesToPublicKey(pkeys[i])
		if pkey.Verify(sig, h) {
			return true
		}
	}

	return false
}

func cloneIfStruct(item StackItem) StackItem {
	switch it := item.(type) {
	case *StructItem:
		return it.Clone()
	default:
		return it
	}
}

func makeArrayOfFalses(n int) []StackItem {
	items := make([]StackItem, n)
	for i := range items {
		items[i] = &BoolItem{false}
	}
	return items
}

func validateMapKey(key *Element) {
	if key == nil {
		panic("no key found")
	}
	if !isValidMapKey(key.Item()) {
		panic("key can't be a collection")
	}
}

func (v *VM) checkInvocationStackSize() {
	if v.istack.len >= MaxInvocationStackSize {
		panic("invocation stack is too big")
	}
}

func (v *VM) checkBigIntSize(a *big.Int) {
	if a.BitLen() > MaxBigIntegerSizeBits {
		panic("big integer is too big")
	}
}

// bytesToPublicKey is a helper deserializing keys using cache and panicing on
// error.
func (v *VM) bytesToPublicKey(b []byte) *keys.PublicKey {
	var pkey *keys.PublicKey
	s := string(b)
	if v.keys[s] != nil {
		pkey = v.keys[s]
	} else {
		var err error
		pkey, err = keys.NewPublicKeyFromBytes(b)
		if err != nil {
			panic(err.Error())
		}
		v.keys[s] = pkey
	}
	return pkey
}
