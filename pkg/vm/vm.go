package vm

import (
	"crypto/elliptic"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	// MaxInvocationStackSize is the maximum size of an invocation stack.
	MaxInvocationStackSize = 1024

	// MaxTryNestingDepth is the maximum level of TRY nesting allowed,
	// that is you can't have more exception handling contexts than this.
	MaxTryNestingDepth = 16

	// MaxStackSize is the maximum number of items allowed to be
	// on all stacks at once.
	MaxStackSize = 2 * 1024

	maxSHLArg = stackitem.MaxBigIntegerSizeBits
)

// SyscallHandler is a type for syscall handler.
type SyscallHandler = func(*VM, uint32) error

// VM represents the virtual machine.
type VM struct {
	state State

	// callback to get interop price
	getPrice func(opcode.Opcode, []byte) int64

	istack *Stack // invocation stack.
	estack *Stack // execution stack.

	uncaughtException stackitem.Item // exception being handled

	refs *refCounter

	gasConsumed int64
	GasLimit    int64

	// SyscallHandler handles SYSCALL opcode.
	SyscallHandler func(v *VM, id uint32) error

	// LoadToken handles CALLT opcode.
	LoadToken func(id int32) error

	trigger trigger.Type

	// Invocations is a script invocation counter.
	Invocations map[util.Uint160]int
}

// New returns a new VM object ready to load AVM bytecode scripts.
func New() *VM {
	return NewWithTrigger(trigger.Application)
}

// NewWithTrigger returns a new VM for executions triggered by t.
func NewWithTrigger(t trigger.Type) *VM {
	vm := &VM{
		state:   NoneState,
		istack:  NewStack("invocation"),
		refs:    newRefCounter(),
		trigger: t,

		SyscallHandler: defaultSyscallHandler,
		Invocations:    make(map[util.Uint160]int),
	}

	vm.estack = vm.newItemStack("evaluation")
	return vm
}

func (v *VM) newItemStack(n string) *Stack {
	s := NewStack(n)
	s.refs = v.refs

	return s
}

// SetPriceGetter registers the given PriceGetterFunc in v.
// f accepts vm's Context, current instruction and instruction parameter.
func (v *VM) SetPriceGetter(f func(opcode.Opcode, []byte) int64) {
	v.getPrice = f
}

// GasConsumed returns the amount of GAS consumed during execution.
func (v *VM) GasConsumed() int64 {
	return v.gasConsumed
}

// AddGas consumes specified amount of gas. It returns true iff gas limit wasn't exceeded.
func (v *VM) AddGas(gas int64) bool {
	v.gasConsumed += gas
	return v.GasLimit < 0 || v.gasConsumed <= v.GasLimit
}

// Estack returns the evaluation stack so interop hooks can utilize this.
func (v *VM) Estack() *Stack {
	return v.estack
}

// Istack returns the invocation stack so interop hooks can utilize this.
func (v *VM) Istack() *Stack {
	return v.istack
}

// LoadArgs loads in the arguments used in the Mian entry point.
func (v *VM) LoadArgs(method []byte, args []stackitem.Item) {
	if len(args) > 0 {
		v.estack.PushVal(args)
	}
	if method != nil {
		v.estack.PushVal(method)
	}
}

// PrintOps prints the opcodes of the current loaded program to stdout.
func (v *VM) PrintOps(out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)
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
			case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.CALL,
				opcode.JMPEQ, opcode.JMPNE,
				opcode.JMPGT, opcode.JMPGE, opcode.JMPLE, opcode.JMPLT,
				opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL, opcode.CALLL,
				opcode.JMPEQL, opcode.JMPNEL,
				opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLEL, opcode.JMPLTL,
				opcode.PUSHA, opcode.ENDTRY, opcode.ENDTRYL:
				desc = getOffsetDesc(ctx, parameter)
			case opcode.TRY, opcode.TRYL:
				catchP, finallyP := getTryParams(instr, parameter)
				desc = fmt.Sprintf("catch %s, finally %s",
					getOffsetDesc(ctx, catchP), getOffsetDesc(ctx, finallyP))
			case opcode.INITSSLOT:
				desc = fmt.Sprint(parameter[0])
			case opcode.CONVERT, opcode.ISTYPE:
				typ := stackitem.Type(parameter[0])
				desc = fmt.Sprintf("%s (%x)", typ, parameter[0])
			case opcode.INITSLOT:
				desc = fmt.Sprintf("%d local, %d arg", parameter[0], parameter[1])
			case opcode.SYSCALL:
				name, err := interopnames.FromID(GetInteropID(parameter))
				if err != nil {
					name = "not found"
				}
				desc = fmt.Sprintf("%s (%x)", name, parameter)
			case opcode.PUSHINT8, opcode.PUSHINT16, opcode.PUSHINT32,
				opcode.PUSHINT64, opcode.PUSHINT128, opcode.PUSHINT256:
				val := bigint.FromBytes(parameter)
				desc = fmt.Sprintf("%d (%x)", val, parameter)
			case opcode.LDLOC, opcode.STLOC, opcode.LDARG, opcode.STARG, opcode.LDSFLD, opcode.STSFLD:
				desc = fmt.Sprintf("%d (%x)", parameter[0], parameter)
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

func getOffsetDesc(ctx *Context, parameter []byte) string {
	offset, rOffset, err := calcJumpOffset(ctx, parameter)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	return fmt.Sprintf("%d (%d/%x)", offset, rOffset, parameter)
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
	v.AddBreakPoint(ctx.nextip + n)
}

// LoadFileWithFlags loads a program in NEF format from the given path, ready to execute it.
func (v *VM) LoadFileWithFlags(path string, f callflag.CallFlag) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	nef, err := nef.FileFromBytes(b)
	if err != nil {
		return err
	}
	v.Load(nef.Script)
	return nil
}

// Load initializes the VM with the program given.
func (v *VM) Load(prog []byte) {
	v.LoadWithFlags(prog, callflag.NoneFlag)
}

// LoadWithFlags initializes the VM with the program and flags given.
func (v *VM) LoadWithFlags(prog []byte, f callflag.CallFlag) {
	// Clear all stacks and state, it could be a reload.
	v.istack.Clear()
	v.estack.Clear()
	v.state = NoneState
	v.gasConsumed = 0
	v.LoadScriptWithFlags(prog, f)
}

// LoadScript loads a script from the internal script table. It
// will immediately push a new context created from this script to
// the invocation stack and starts executing it.
func (v *VM) LoadScript(b []byte) {
	v.LoadScriptWithFlags(b, callflag.NoneFlag)
}

// LoadScriptWithFlags loads script and sets call flag to f.
func (v *VM) LoadScriptWithFlags(b []byte, f callflag.CallFlag) {
	v.checkInvocationStackSize()
	ctx := NewContextWithParams(b, 0, -1, 0)
	v.estack = v.newItemStack("estack")
	ctx.estack = v.estack
	ctx.tryStack = NewStack("exception")
	ctx.callFlag = f
	ctx.static = newSlot(v.refs)
	ctx.callingScriptHash = v.GetCurrentScriptHash()
	v.istack.PushVal(ctx)
}

// LoadScriptWithHash if similar to the LoadScriptWithFlags method, but it also loads
// given script hash directly into the Context to avoid its recalculations and to make
// is possible to override it for deployed contracts with special hashes (the function
// assumes that it is used for deployed contracts setting context's parameters
// accordingly). It's up to user of this function to make sure the script and hash match
// each other.
func (v *VM) LoadScriptWithHash(b []byte, hash util.Uint160, f callflag.CallFlag) {
	shash := v.GetCurrentScriptHash()
	v.LoadScriptWithCallingHash(shash, b, hash, f, true, 0)
}

// LoadScriptWithCallingHash is similar to LoadScriptWithHash but sets calling hash explicitly.
// It should be used for calling from native contracts.
func (v *VM) LoadScriptWithCallingHash(caller util.Uint160, b []byte, hash util.Uint160,
	f callflag.CallFlag, hasReturn bool, paramCount uint16) {
	v.LoadScriptWithFlags(b, f)
	ctx := v.Context()
	ctx.scriptHash = hash
	ctx.callingScriptHash = caller
	if hasReturn {
		ctx.RetCount = 1
	} else {
		ctx.RetCount = 0
	}
	ctx.ParamCount = int(paramCount)
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
	if n == "istack" {
		s = v.istack
	}
	if n == "estack" {
		s = v.estack
	}
	b, _ := json.MarshalIndent(s, "", "    ")
	return string(b)
}

// State returns the state for the VM.
func (v *VM) State() State {
	return v.state
}

// Ready returns true if the VM ready to execute the loaded program.
// Will return false if no program is loaded.
func (v *VM) Ready() bool {
	return v.istack.Len() > 0
}

// Run starts the execution of the loaded program.
func (v *VM) Run() error {
	if !v.Ready() {
		v.state = FaultState
		return errors.New("no program loaded")
	}

	if v.state.HasFlag(FaultState) {
		// VM already ran something and failed, in general its state is
		// undefined in this case so we can't run anything.
		return errors.New("VM has failed")
	}
	// HaltState (the default) or BreakState are safe to continue.
	v.state = NoneState
	for {
		switch {
		case v.state.HasFlag(FaultState):
			// Should be caught and reported already by the v.Step(),
			// but we're checking here anyway just in case.
			return errors.New("VM has failed")
		case v.state.HasFlag(HaltState), v.state.HasFlag(BreakState):
			// Normal exit from this loop.
			return nil
		case v.state == NoneState:
			if err := v.Step(); err != nil {
				return err
			}
		default:
			v.state = FaultState
			return errors.New("unknown state")
		}
		// check for breakpoint before executing the next instruction
		ctx := v.Context()
		if ctx != nil && ctx.atBreakPoint() {
			v.state = BreakState
		}
	}
}

// Step 1 instruction in the program.
func (v *VM) Step() error {
	ctx := v.Context()
	op, param, err := ctx.Next()
	if err != nil {
		v.state = FaultState
		return newError(ctx.ip, op, err)
	}
	return v.execute(ctx, op, param)
}

// StepInto behaves the same as “step over” in case if the line does not contain a function. Otherwise
// the debugger will enter the called function and continue line-by-line debugging there.
func (v *VM) StepInto() error {
	ctx := v.Context()

	if ctx == nil {
		v.state = HaltState
	}

	if v.HasStopped() {
		return nil
	}

	if ctx != nil && ctx.prog != nil {
		op, param, err := ctx.Next()
		if err != nil {
			v.state = FaultState
			return newError(ctx.ip, op, err)
		}
		vErr := v.execute(ctx, op, param)
		if vErr != nil {
			return vErr
		}
	}

	cctx := v.Context()
	if cctx != nil && cctx.atBreakPoint() {
		v.state = BreakState
	}
	return nil
}

// StepOut takes the debugger to the line where the current function was called.
func (v *VM) StepOut() error {
	var err error
	if v.state == BreakState {
		v.state = NoneState
	}

	expSize := v.istack.len
	for v.state == NoneState && v.istack.len >= expSize {
		err = v.StepInto()
	}
	if v.state == NoneState {
		v.state = BreakState
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

	if v.state == BreakState {
		v.state = NoneState
	}

	expSize := v.istack.len
	for {
		err = v.StepInto()
		if !(v.state == NoneState && v.istack.len > expSize) {
			break
		}
	}

	if v.state == NoneState {
		v.state = BreakState
	}

	return err
}

// HasFailed returns whether VM is in the failed state now. Usually used to
// check status after Run.
func (v *VM) HasFailed() bool {
	return v.state.HasFlag(FaultState)
}

// HasStopped returns whether VM is in Halt or Failed state.
func (v *VM) HasStopped() bool {
	return v.state.HasFlag(HaltState) || v.state.HasFlag(FaultState)
}

// HasHalted returns whether VM is in Halt state.
func (v *VM) HasHalted() bool {
	return v.state.HasFlag(HaltState)
}

// AtBreakpoint returns whether VM is at breakpoint.
func (v *VM) AtBreakpoint() bool {
	return v.state.HasFlag(BreakState)
}

// GetInteropID converts instruction parameter to an interop ID.
func GetInteropID(parameter []byte) uint32 {
	return binary.LittleEndian.Uint32(parameter)
}

// execute performs an instruction cycle in the VM. Acting on the instruction (opcode).
func (v *VM) execute(ctx *Context, op opcode.Opcode, parameter []byte) (err error) {
	// Instead of polluting the whole VM logic with error handling, we will recover
	// each panic at a central point, putting the VM in a fault state and setting error.
	defer func() {
		if errRecover := recover(); errRecover != nil {
			v.state = FaultState
			err = newError(ctx.ip, op, errRecover)
		} else if v.refs.size > MaxStackSize {
			v.state = FaultState
			err = newError(ctx.ip, op, "stack is too big")
		}
	}()

	if v.getPrice != nil && ctx.ip < len(ctx.prog) {
		v.gasConsumed += v.getPrice(op, parameter)
		if v.GasLimit >= 0 && v.gasConsumed > v.GasLimit {
			panic("gas limit is exceeded")
		}
	}

	if op <= opcode.PUSHINT256 {
		v.estack.PushVal(bigint.FromBytes(parameter))
		return
	}

	switch op {
	case opcode.PUSHM1, opcode.PUSH0, opcode.PUSH1, opcode.PUSH2, opcode.PUSH3,
		opcode.PUSH4, opcode.PUSH5, opcode.PUSH6, opcode.PUSH7,
		opcode.PUSH8, opcode.PUSH9, opcode.PUSH10, opcode.PUSH11,
		opcode.PUSH12, opcode.PUSH13, opcode.PUSH14, opcode.PUSH15,
		opcode.PUSH16:
		val := int(op) - int(opcode.PUSH0)
		v.estack.PushVal(val)

	case opcode.PUSHDATA1, opcode.PUSHDATA2, opcode.PUSHDATA4:
		v.estack.PushVal(parameter)

	case opcode.PUSHA:
		n := getJumpOffset(ctx, parameter)
		ptr := stackitem.NewPointerWithHash(n, ctx.prog, ctx.ScriptHash())
		v.estack.PushVal(ptr)

	case opcode.PUSHNULL:
		v.estack.PushVal(stackitem.Null{})

	case opcode.ISNULL:
		_, ok := v.estack.Pop().value.(stackitem.Null)
		v.estack.PushVal(ok)

	case opcode.ISTYPE:
		res := v.estack.Pop().Item()
		v.estack.PushVal(res.Type() == stackitem.Type(parameter[0]))

	case opcode.CONVERT:
		typ := stackitem.Type(parameter[0])
		item := v.estack.Pop().Item()
		result, err := item.Convert(typ)
		if err != nil {
			panic(err)
		}
		v.estack.PushVal(result)

	case opcode.INITSSLOT:
		if parameter[0] == 0 {
			panic("zero argument")
		}
		ctx.static.init(int(parameter[0]))

	case opcode.INITSLOT:
		if ctx.local != nil || ctx.arguments != nil {
			panic("already initialized")
		}
		if parameter[0] == 0 && parameter[1] == 0 {
			panic("zero argument")
		}
		if parameter[0] > 0 {
			ctx.local = v.newSlot(int(parameter[0]))
		}
		if parameter[1] > 0 {
			sz := int(parameter[1])
			ctx.arguments = v.newSlot(sz)
			for i := 0; i < sz; i++ {
				ctx.arguments.Set(i, v.estack.Pop().Item())
			}
		}

	case opcode.LDSFLD0, opcode.LDSFLD1, opcode.LDSFLD2, opcode.LDSFLD3, opcode.LDSFLD4, opcode.LDSFLD5, opcode.LDSFLD6:
		item := ctx.static.Get(int(op - opcode.LDSFLD0))
		v.estack.PushVal(item)

	case opcode.LDSFLD:
		item := ctx.static.Get(int(parameter[0]))
		v.estack.PushVal(item)

	case opcode.STSFLD0, opcode.STSFLD1, opcode.STSFLD2, opcode.STSFLD3, opcode.STSFLD4, opcode.STSFLD5, opcode.STSFLD6:
		item := v.estack.Pop().Item()
		ctx.static.Set(int(op-opcode.STSFLD0), item)

	case opcode.STSFLD:
		item := v.estack.Pop().Item()
		ctx.static.Set(int(parameter[0]), item)

	case opcode.LDLOC0, opcode.LDLOC1, opcode.LDLOC2, opcode.LDLOC3, opcode.LDLOC4, opcode.LDLOC5, opcode.LDLOC6:
		item := ctx.local.Get(int(op - opcode.LDLOC0))
		v.estack.PushVal(item)

	case opcode.LDLOC:
		item := ctx.local.Get(int(parameter[0]))
		v.estack.PushVal(item)

	case opcode.STLOC0, opcode.STLOC1, opcode.STLOC2, opcode.STLOC3, opcode.STLOC4, opcode.STLOC5, opcode.STLOC6:
		item := v.estack.Pop().Item()
		ctx.local.Set(int(op-opcode.STLOC0), item)

	case opcode.STLOC:
		item := v.estack.Pop().Item()
		ctx.local.Set(int(parameter[0]), item)

	case opcode.LDARG0, opcode.LDARG1, opcode.LDARG2, opcode.LDARG3, opcode.LDARG4, opcode.LDARG5, opcode.LDARG6:
		item := ctx.arguments.Get(int(op - opcode.LDARG0))
		v.estack.PushVal(item)

	case opcode.LDARG:
		item := ctx.arguments.Get(int(parameter[0]))
		v.estack.PushVal(item)

	case opcode.STARG0, opcode.STARG1, opcode.STARG2, opcode.STARG3, opcode.STARG4, opcode.STARG5, opcode.STARG6:
		item := v.estack.Pop().Item()
		ctx.arguments.Set(int(op-opcode.STARG0), item)

	case opcode.STARG:
		item := v.estack.Pop().Item()
		ctx.arguments.Set(int(parameter[0]), item)

	case opcode.NEWBUFFER:
		n := toInt(v.estack.Pop().BigInt())
		if n < 0 || n > stackitem.MaxSize {
			panic("invalid size")
		}
		v.estack.PushVal(stackitem.NewBuffer(make([]byte, n)))

	case opcode.MEMCPY:
		n := toInt(v.estack.Pop().BigInt())
		if n < 0 {
			panic("invalid size")
		}
		si := toInt(v.estack.Pop().BigInt())
		if si < 0 {
			panic("invalid source index")
		}
		src := v.estack.Pop().Bytes()
		if sum := si + n; sum < 0 || sum > len(src) {
			panic("size is too big")
		}
		di := toInt(v.estack.Pop().BigInt())
		if di < 0 {
			panic("invalid destination index")
		}
		dst := v.estack.Pop().value.(*stackitem.Buffer).Value().([]byte)
		if sum := di + n; sum < 0 || sum > len(dst) {
			panic("size is too big")
		}
		copy(dst[di:], src[si:si+n])

	case opcode.CAT:
		b := v.estack.Pop().Bytes()
		a := v.estack.Pop().Bytes()
		l := len(a) + len(b)
		if l > stackitem.MaxSize {
			panic(fmt.Sprintf("too big item: %d", l))
		}
		ab := make([]byte, l)
		copy(ab, a)
		copy(ab[len(a):], b)
		v.estack.PushVal(stackitem.NewBuffer(ab))

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
		res := make([]byte, l)
		copy(res, s[o:last])
		v.estack.PushVal(stackitem.NewBuffer(res))

	case opcode.LEFT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		if t := len(s); l > t {
			panic("size is too big")
		}
		res := make([]byte, l)
		copy(res, s[:l])
		v.estack.PushVal(stackitem.NewBuffer(res))

	case opcode.RIGHT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		res := make([]byte, l)
		copy(res, s[len(s)-l:])
		v.estack.PushVal(stackitem.NewBuffer(res))

	case opcode.DEPTH:
		v.estack.PushVal(v.estack.Len())

	case opcode.DROP:
		if v.estack.Len() < 1 {
			panic("stack is too small")
		}
		v.estack.Pop()

	case opcode.NIP:
		elem := v.estack.RemoveAt(1)
		if elem == nil {
			panic("no second element found")
		}

	case opcode.XDROP:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("invalid length")
		}
		e := v.estack.RemoveAt(n)
		if e == nil {
			panic("bad index")
		}

	case opcode.CLEAR:
		v.estack.Clear()

	case opcode.DUP:
		v.estack.Push(v.estack.Dup(0))

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

	case opcode.TUCK:
		a := v.estack.Dup(0)
		if a == nil {
			panic("no top-level element found")
		}
		if v.estack.Len() < 2 {
			panic("can't TUCK with a one-element stack")
		}
		v.estack.InsertAt(a, 2)

	case opcode.SWAP:
		err := v.estack.Swap(1, 0)
		if err != nil {
			panic(err.Error())
		}

	case opcode.ROT:
		err := v.estack.Roll(2)
		if err != nil {
			panic(err.Error())
		}

	case opcode.ROLL:
		n := int(v.estack.Pop().BigInt().Int64())
		err := v.estack.Roll(n)
		if err != nil {
			panic(err.Error())
		}

	case opcode.REVERSE3, opcode.REVERSE4, opcode.REVERSEN:
		n := 3
		switch op {
		case opcode.REVERSE4:
			n = 4
		case opcode.REVERSEN:
			n = int(v.estack.Pop().BigInt().Int64())
		}
		if err := v.estack.ReverseTop(n); err != nil {
			panic(err.Error())
		}

	// Bit operations.
	case opcode.INVERT:
		// inplace
		e := v.estack.Peek(0)
		i := e.BigInt()
		e.value = stackitem.Make(new(big.Int).Not(i))

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

	case opcode.EQUAL, opcode.NOTEQUAL:
		b := v.estack.Pop()
		if b == nil {
			panic("no top-level element found")
		}
		a := v.estack.Pop()
		if a == nil {
			panic("no second-to-the-top element found")
		}
		v.estack.PushVal(a.value.Equals(b.value) == (op == opcode.EQUAL))

	// Numeric operations.
	case opcode.SIGN:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Sign())

	case opcode.ABS:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Abs(x))

	case opcode.NEGATE:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Neg(x))

	case opcode.INC:
		x := v.estack.Pop().BigInt()
		a := new(big.Int).Add(x, big.NewInt(1))
		v.estack.PushVal(a)

	case opcode.DEC:
		x := v.estack.Pop().BigInt()
		a := new(big.Int).Sub(x, big.NewInt(1))
		v.estack.PushVal(a)

	case opcode.ADD:
		a := v.estack.Pop().BigInt()
		b := v.estack.Pop().BigInt()

		c := new(big.Int).Add(a, b)
		v.estack.PushVal(c)

	case opcode.SUB:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()

		c := new(big.Int).Sub(a, b)
		v.estack.PushVal(c)

	case opcode.MUL:
		a := v.estack.Pop().BigInt()
		b := v.estack.Pop().BigInt()

		c := new(big.Int).Mul(a, b)
		v.estack.PushVal(c)

	case opcode.DIV:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()

		v.estack.PushVal(new(big.Int).Quo(a, b))

	case opcode.MOD:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()

		v.estack.PushVal(new(big.Int).Rem(a, b))

	case opcode.POW:
		exp := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		if ei := exp.Int64(); !exp.IsInt64() || ei > maxSHLArg || ei < 0 {
			panic("invalid exponent")
		}
		v.estack.PushVal(new(big.Int).Exp(a, exp, nil))

	case opcode.SQRT:
		a := v.estack.Pop().BigInt()
		if a.Sign() == -1 {
			panic("negative value")
		}

		v.estack.PushVal(new(big.Int).Sqrt(a))

	case opcode.SHL, opcode.SHR:
		b := v.estack.Pop().BigInt().Int64()
		if b == 0 {
			return
		} else if b < 0 || b > maxSHLArg {
			panic(fmt.Sprintf("operand must be between %d and %d", 0, maxSHLArg))
		}
		a := v.estack.Pop().BigInt()

		var item big.Int
		if op == opcode.SHL {
			item.Lsh(a, uint(b))
		} else {
			item.Rsh(a, uint(b))
		}

		v.estack.PushVal(&item)

	case opcode.NOT:
		x := v.estack.Pop().Bool()
		v.estack.PushVal(!x)

	case opcode.BOOLAND:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a && b)

	case opcode.BOOLOR:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a || b)

	case opcode.NZ:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Sign() != 0)

	case opcode.NUMEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == 0)

	case opcode.NUMNOTEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) != 0)

	case opcode.LT, opcode.LE, opcode.GT, opcode.GE:
		eb := v.estack.Pop()
		ea := v.estack.Pop()
		_, aNil := ea.Item().(stackitem.Null)
		_, bNil := eb.Item().(stackitem.Null)

		res := !aNil && !bNil
		if res {
			cmp := ea.BigInt().Cmp(eb.BigInt())
			switch op {
			case opcode.LT:
				res = cmp == -1
			case opcode.LE:
				res = cmp <= 0
			case opcode.GT:
				res = cmp == 1
			case opcode.GE:
				res = cmp >= 0
			}
		}
		v.estack.PushVal(res)

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

	// Object operations
	case opcode.NEWARRAY0:
		v.estack.PushVal(stackitem.NewArray([]stackitem.Item{}))

	case opcode.NEWARRAY, opcode.NEWARRAYT:
		item := v.estack.Pop()
		n := item.BigInt().Int64()
		if n > stackitem.MaxArraySize {
			panic("too long array")
		}
		typ := stackitem.AnyT
		if op == opcode.NEWARRAYT {
			typ = stackitem.Type(parameter[0])
		}
		items := makeArrayOfType(int(n), typ)
		v.estack.PushVal(stackitem.NewArray(items))

	case opcode.NEWSTRUCT0:
		v.estack.PushVal(stackitem.NewStruct([]stackitem.Item{}))

	case opcode.NEWSTRUCT:
		item := v.estack.Pop()
		n := item.BigInt().Int64()
		if n > stackitem.MaxArraySize {
			panic("too long struct")
		}
		items := makeArrayOfType(int(n), stackitem.AnyT)
		v.estack.PushVal(stackitem.NewStruct(items))

	case opcode.APPEND:
		itemElem := v.estack.Pop()
		arrElem := v.estack.Pop()

		val := cloneIfStruct(itemElem.value, MaxStackSize-v.refs.size)

		switch t := arrElem.value.(type) {
		case *stackitem.Array:
			if t.Len() >= stackitem.MaxArraySize {
				panic("too long array")
			}
			t.Append(val)
		case *stackitem.Struct:
			if t.Len() >= stackitem.MaxArraySize {
				panic("too long struct")
			}
			t.Append(val)
		default:
			panic("APPEND: not of underlying type Array")
		}

		v.refs.Add(val)

	case opcode.PACK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 || n > v.estack.Len() || n > stackitem.MaxArraySize {
			panic("OPACK: invalid length")
		}

		items := make([]stackitem.Item, n)
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
		// Struct and Array items have their underlying value as []Item.
		case *stackitem.Array, *stackitem.Struct:
			arr := t.Value().([]stackitem.Item)
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index].Dup()
			v.estack.PushVal(item)
		case *stackitem.Map:
			index := t.Index(key.Item())
			if index < 0 {
				panic("invalid key")
			}
			v.estack.Push(&Element{value: t.Value().([]stackitem.MapElement)[index].Value.Dup()})
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
		// Struct and Array items have their underlying value as []Item.
		case *stackitem.Array, *stackitem.Struct:
			arr := t.Value().([]stackitem.Item)
			index := int(key.BigInt().Int64())
			if index < 0 || index >= len(arr) {
				panic("SETITEM: invalid index")
			}
			v.refs.Remove(arr[index])
			arr[index] = item
			v.refs.Add(arr[index])
		case *stackitem.Map:
			if i := t.Index(key.value); i >= 0 {
				v.refs.Remove(t.Value().([]stackitem.MapElement)[i].Value)
			} else if t.Len() >= stackitem.MaxArraySize {
				panic("too big map")
			}
			t.Add(key.value, item)
			v.refs.Add(item)

		case *stackitem.Buffer:
			index := toInt(key.BigInt())
			if index < 0 || index >= t.Len() {
				panic("invalid index")
			}
			bi, err := item.TryInteger()
			b := toInt(bi)
			if err != nil || b < math.MinInt8 || b > math.MaxUint8 {
				panic("invalid value")
			}
			t.Value().([]byte)[index] = byte(b)

		default:
			panic(fmt.Sprintf("SETITEM: invalid item type %s", t))
		}

	case opcode.REVERSEITEMS:
		item := v.estack.Pop()
		switch t := item.value.(type) {
		case *stackitem.Array, *stackitem.Struct:
			a := t.Value().([]stackitem.Item)
			for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
				a[i], a[j] = a[j], a[i]
			}
		case *stackitem.Buffer:
			slice := t.Value().([]byte)
			for i, j := 0, t.Len()-1; i < j; i, j = i+1, j-1 {
				slice[i], slice[j] = slice[j], slice[i]
			}
		default:
			panic(fmt.Sprintf("invalid item type %s", t))
		}
	case opcode.REMOVE:
		key := v.estack.Pop()
		validateMapKey(key)

		elem := v.estack.Pop()
		switch t := elem.value.(type) {
		case *stackitem.Array:
			a := t.Value().([]stackitem.Item)
			k := int(key.BigInt().Int64())
			if k < 0 || k >= len(a) {
				panic("REMOVE: invalid index")
			}
			v.refs.Remove(a[k])
			t.Remove(k)
		case *stackitem.Struct:
			a := t.Value().([]stackitem.Item)
			k := int(key.BigInt().Int64())
			if k < 0 || k >= len(a) {
				panic("REMOVE: invalid index")
			}
			v.refs.Remove(a[k])
			t.Remove(k)
		case *stackitem.Map:
			index := t.Index(key.Item())
			// NEO 2.0 doesn't error on missing key.
			if index >= 0 {
				v.refs.Remove(t.Value().([]stackitem.MapElement)[index].Value)
				t.Drop(index)
			}
		default:
			panic("REMOVE: invalid type")
		}

	case opcode.CLEARITEMS:
		elem := v.estack.Pop()
		switch t := elem.value.(type) {
		case *stackitem.Array:
			for _, item := range t.Value().([]stackitem.Item) {
				v.refs.Remove(item)
			}
			t.Clear()
		case *stackitem.Struct:
			for _, item := range t.Value().([]stackitem.Item) {
				v.refs.Remove(item)
			}
			t.Clear()
		case *stackitem.Map:
			for i := range t.Value().([]stackitem.MapElement) {
				v.refs.Remove(t.Value().([]stackitem.MapElement)[i].Value)
			}
			t.Clear()
		default:
			panic("CLEARITEMS: invalid type")
		}

	case opcode.POPITEM:
		arr := v.estack.Pop().Item()
		elems := arr.Value().([]stackitem.Item)
		index := len(elems) - 1
		elem := elems[index]
		switch item := arr.(type) {
		case *stackitem.Array:
			item.Remove(index)
		case *stackitem.Struct:
			item.Remove(index)
		}
		v.refs.Remove(elem)
		v.estack.PushVal(elem)

	case opcode.SIZE:
		elem := v.estack.Pop()
		// Cause there is no native (byte) item type here, hence we need to check
		// the type of the item for array size operations.
		switch t := elem.Value().(type) {
		case []stackitem.Item:
			v.estack.PushVal(len(t))
		case []stackitem.MapElement:
			v.estack.PushVal(len(t))
		default:
			v.estack.PushVal(len(elem.Bytes()))
		}

	case opcode.JMP, opcode.JMPL, opcode.JMPIF, opcode.JMPIFL, opcode.JMPIFNOT, opcode.JMPIFNOTL,
		opcode.JMPEQ, opcode.JMPEQL, opcode.JMPNE, opcode.JMPNEL,
		opcode.JMPGT, opcode.JMPGTL, opcode.JMPGE, opcode.JMPGEL,
		opcode.JMPLT, opcode.JMPLTL, opcode.JMPLE, opcode.JMPLEL:
		offset := getJumpOffset(ctx, parameter)
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

		if cond {
			v.Jump(ctx, offset)
		}

	case opcode.CALL, opcode.CALLL:
		// Note: jump offset must be calculated regarding to new context,
		// but it is cloned and thus has the same script and instruction pointer.
		v.call(ctx, getJumpOffset(ctx, parameter))

	case opcode.CALLA:
		ptr := v.estack.Pop().Item().(*stackitem.Pointer)
		if ptr.ScriptHash() != ctx.ScriptHash() {
			panic("invalid script in pointer")
		}

		v.call(ctx, ptr.Position())

	case opcode.CALLT:
		id := int32(binary.LittleEndian.Uint16(parameter))
		if err := v.LoadToken(id); err != nil {
			panic(err)
		}

	case opcode.SYSCALL:
		interopID := GetInteropID(parameter)
		err := v.SyscallHandler(v, interopID)
		if err != nil {
			panic(fmt.Sprintf("failed to invoke syscall %d: %s", interopID, err))
		}

	case opcode.RET:
		oldCtx := v.istack.Pop().Value().(*Context)
		oldEstack := v.estack

		v.unloadContext(oldCtx)
		if v.istack.Len() == 0 {
			v.state = HaltState
			break
		}

		newEstack := v.Context().estack
		if oldEstack != newEstack {
			if oldCtx.RetCount >= 0 && oldEstack.Len() != oldCtx.RetCount {
				panic(fmt.Errorf("invalid return values count: expected %d, got %d",
					oldCtx.RetCount, oldEstack.Len()))
			}
			rvcount := oldEstack.Len()
			for i := rvcount; i > 0; i-- {
				elem := oldEstack.RemoveAt(i - 1)
				newEstack.Push(elem)
			}
			v.estack = newEstack
		}

	case opcode.NEWMAP:
		v.estack.Push(&Element{value: stackitem.NewMap()})

	case opcode.KEYS:
		item := v.estack.Pop()
		if item == nil {
			panic("no argument")
		}

		m, ok := item.value.(*stackitem.Map)
		if !ok {
			panic("not a Map")
		}

		arr := make([]stackitem.Item, 0, m.Len())
		for k := range m.Value().([]stackitem.MapElement) {
			arr = append(arr, m.Value().([]stackitem.MapElement)[k].Key.Dup())
		}
		v.estack.PushVal(arr)

	case opcode.VALUES:
		item := v.estack.Pop()
		if item == nil {
			panic("no argument")
		}

		var arr []stackitem.Item
		switch t := item.value.(type) {
		case *stackitem.Array, *stackitem.Struct:
			src := t.Value().([]stackitem.Item)
			arr = make([]stackitem.Item, len(src))
			for i := range src {
				arr[i] = cloneIfStruct(src[i], MaxStackSize-v.refs.size)
			}
		case *stackitem.Map:
			arr = make([]stackitem.Item, 0, t.Len())
			for k := range t.Value().([]stackitem.MapElement) {
				arr = append(arr, cloneIfStruct(t.Value().([]stackitem.MapElement)[k].Value, MaxStackSize-v.refs.size))
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
		case *stackitem.Array, *stackitem.Struct:
			index := key.BigInt().Int64()
			if index < 0 {
				panic("negative index")
			}
			v.estack.PushVal(index < int64(len(c.Array())))
		case *stackitem.Map:
			v.estack.PushVal(t.Has(key.Item()))
		case *stackitem.Buffer:
			index := key.BigInt().Int64()
			if index < 0 {
				panic("negative index")
			}
			v.estack.PushVal(index < int64(t.Len()))
		default:
			panic("wrong collection type")
		}

	case opcode.NOP:
		// unlucky ^^

	case opcode.THROW:
		v.throw(v.estack.Pop().Item())

	case opcode.ABORT:
		panic("ABORT")

	case opcode.ASSERT:
		if !v.estack.Pop().Bool() {
			panic("ASSERT failed")
		}

	case opcode.TRY, opcode.TRYL:
		catchP, finallyP := getTryParams(op, parameter)
		if ctx.tryStack.Len() >= MaxTryNestingDepth {
			panic("maximum TRY depth exceeded")
		}
		cOffset := getJumpOffset(ctx, catchP)
		fOffset := getJumpOffset(ctx, finallyP)
		if cOffset == ctx.ip && fOffset == ctx.ip {
			panic("invalid offset for TRY*")
		} else if cOffset == ctx.ip {
			cOffset = -1
		} else if fOffset == ctx.ip {
			fOffset = -1
		}
		eCtx := newExceptionHandlingContext(cOffset, fOffset)
		ctx.tryStack.PushVal(eCtx)

	case opcode.ENDTRY, opcode.ENDTRYL:
		eCtx := ctx.tryStack.Peek(0).Value().(*exceptionHandlingContext)
		if eCtx.State == eFinally {
			panic("invalid exception handling state during ENDTRY*")
		}
		eOffset := getJumpOffset(ctx, parameter)
		if eCtx.HasFinally() {
			eCtx.State = eFinally
			eCtx.EndOffset = eOffset
			eOffset = eCtx.FinallyOffset
		} else {
			ctx.tryStack.Pop()
		}
		v.Jump(ctx, eOffset)

	case opcode.ENDFINALLY:
		if v.uncaughtException != nil {
			v.handleException()
			return
		}
		eCtx := ctx.tryStack.Pop().Value().(*exceptionHandlingContext)
		v.Jump(ctx, eCtx.EndOffset)

	default:
		panic(fmt.Sprintf("unknown opcode %s", op.String()))
	}
	return
}

func (v *VM) unloadContext(ctx *Context) {
	if ctx.local != nil {
		ctx.local.Clear()
	}
	if ctx.arguments != nil {
		ctx.arguments.Clear()
	}
	currCtx := v.Context()
	if ctx.static != nil && currCtx != nil && ctx.static != currCtx.static {
		ctx.static.Clear()
	}
}

// getTryParams splits TRY(L) instruction parameter into offsets for catch and finally blocks.
func getTryParams(op opcode.Opcode, p []byte) ([]byte, []byte) {
	i := 1
	if op == opcode.TRYL {
		i = 4
	}
	return p[:i], p[i:]
}

// getJumpCondition performs opcode specific comparison of a and b.
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

func (v *VM) throw(item stackitem.Item) {
	v.uncaughtException = item
	v.handleException()
}

// Jump performs jump to the offset.
func (v *VM) Jump(ctx *Context, offset int) {
	ctx.nextip = offset
}

// Call calls method by offset. It is similar to Jump but also
// pushes new context to the invocation stack and increments
// invocation counter for the corresponding context script hash.
func (v *VM) Call(ctx *Context, offset int) {
	v.call(ctx, offset)
	v.Invocations[ctx.ScriptHash()]++
}

// call is an internal representation of Call, which does not
// affect the invocation counter and is only being used by vm
// package.
func (v *VM) call(ctx *Context, offset int) {
	v.checkInvocationStackSize()
	newCtx := ctx.Copy()
	newCtx.RetCount = -1
	newCtx.local = nil
	newCtx.arguments = nil
	newCtx.tryStack = NewStack("exception")
	newCtx.NEF = ctx.NEF
	v.istack.PushVal(newCtx)
	v.Jump(newCtx, offset)
}

// getJumpOffset returns instruction number in a current context
// to a which JMP should be performed.
// parameter should have length either 1 or 4 and
// is interpreted as little-endian.
func getJumpOffset(ctx *Context, parameter []byte) int {
	offset, _, err := calcJumpOffset(ctx, parameter)
	if err != nil {
		panic(err)
	}
	return offset
}

// calcJumpOffset returns absolute and relative offset of JMP/CALL/TRY instructions
// either in short (1-byte) or long (4-byte) form.
func calcJumpOffset(ctx *Context, parameter []byte) (int, int, error) {
	var rOffset int32
	switch l := len(parameter); l {
	case 1:
		rOffset = int32(int8(parameter[0]))
	case 4:
		rOffset = int32(binary.LittleEndian.Uint32(parameter))
	default:
		_, curr := ctx.CurrInstr()
		return 0, 0, fmt.Errorf("invalid %s parameter length: %d", curr, l)
	}
	offset := ctx.ip + int(rOffset)
	if offset < 0 || offset > len(ctx.prog) {
		return 0, 0, fmt.Errorf("invalid offset %d ip at %d", offset, ctx.ip)
	}

	return offset, int(rOffset), nil
}

func (v *VM) handleException() {
	pop := 0
	ictxv := v.istack.Peek(0)
	ictx := ictxv.Value().(*Context)
	for ictx != nil {
		e := ictx.tryStack.Peek(0)
		for e != nil {
			ectx := e.Value().(*exceptionHandlingContext)
			if ectx.State == eFinally || (ectx.State == eCatch && !ectx.HasFinally()) {
				ictx.tryStack.Pop()
				e = ictx.tryStack.Peek(0)
				continue
			}
			for i := 0; i < pop; i++ {
				ctx := v.istack.Pop().Value().(*Context)
				v.unloadContext(ctx)
			}
			if ectx.State == eTry && ectx.HasCatch() {
				ectx.State = eCatch
				v.estack.PushVal(v.uncaughtException)
				v.uncaughtException = nil
				v.Jump(ictx, ectx.CatchOffset)
			} else {
				ectx.State = eFinally
				v.Jump(ictx, ectx.FinallyOffset)
			}
			return
		}
		pop++
		ictxv = ictxv.Next()
		if ictxv == nil {
			break
		}
		ictx = ictxv.Value().(*Context)
	}
	throwUnhandledException(v.uncaughtException)
}

// throwUnhandledException gets exception message from the provided stackitem and panics.
func throwUnhandledException(item stackitem.Item) {
	msg := "unhandled exception"
	switch item.Type() {
	case stackitem.ArrayT:
		if arr := item.Value().([]stackitem.Item); len(arr) > 0 {
			data, err := arr[0].TryBytes()
			if err == nil {
				msg = fmt.Sprintf("%s: %q", msg, string(data))
			}
		}
	default:
		data, err := item.TryBytes()
		if err == nil {
			msg = fmt.Sprintf("%s: %q", msg, string(data))
		}
	}
	panic(msg)
}

// CheckMultisigPar checks if sigs contains sufficient valid signatures.
func CheckMultisigPar(v *VM, curve elliptic.Curve, h []byte, pkeys [][]byte, sigs [][]byte) bool {
	if len(sigs) == 1 {
		return checkMultisig1(v, curve, h, pkeys, sigs[0])
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

	tasks <- task{pub: bytesToPublicKey(pkeys[k1], curve), signum: s1}
	tasks <- task{pub: bytesToPublicKey(pkeys[k2], curve), signum: s2}

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
		tasks <- task{pub: bytesToPublicKey(pkeys[nextKey], curve), signum: nextSig}
	}

	close(tasks)

	return sigok
}

func checkMultisig1(v *VM, curve elliptic.Curve, h []byte, pkeys [][]byte, sig []byte) bool {
	for i := range pkeys {
		pkey := bytesToPublicKey(pkeys[i], curve)
		if pkey.Verify(sig, h) {
			return true
		}
	}

	return false
}

func cloneIfStruct(item stackitem.Item, limit int) stackitem.Item {
	switch it := item.(type) {
	case *stackitem.Struct:
		ret, err := it.Clone(limit)
		if err != nil {
			panic(err)
		}
		return ret
	default:
		return it
	}
}

func makeArrayOfType(n int, typ stackitem.Type) []stackitem.Item {
	if !typ.IsValid() {
		panic(fmt.Sprintf("invalid stack item type: %d", typ))
	}
	items := make([]stackitem.Item, n)
	for i := range items {
		switch typ {
		case stackitem.BooleanT:
			items[i] = stackitem.NewBool(false)
		case stackitem.IntegerT:
			items[i] = stackitem.NewBigInteger(big.NewInt(0))
		case stackitem.ByteArrayT:
			items[i] = stackitem.NewByteArray([]byte{})
		default:
			items[i] = stackitem.Null{}
		}
	}
	return items
}

func validateMapKey(key *Element) {
	if key == nil {
		panic("no key found")
	}
	if err := stackitem.IsValidMapKey(key.Item()); err != nil {
		panic(err)
	}
}

func (v *VM) checkInvocationStackSize() {
	if v.istack.len >= MaxInvocationStackSize {
		panic("invocation stack is too big")
	}
}

// bytesToPublicKey is a helper deserializing keys using cache and panicing on
// error.
func bytesToPublicKey(b []byte, curve elliptic.Curve) *keys.PublicKey {
	pkey, err := keys.NewPublicKeyFromBytes(b, curve)
	if err != nil {
		panic(err.Error())
	}
	return pkey
}

// GetCallingScriptHash implements ScriptHashGetter interface.
func (v *VM) GetCallingScriptHash() util.Uint160 {
	return v.Context().callingScriptHash
}

// GetEntryScriptHash implements ScriptHashGetter interface.
func (v *VM) GetEntryScriptHash() util.Uint160 {
	return v.getContextScriptHash(v.Istack().Len() - 1)
}

// GetCurrentScriptHash implements ScriptHashGetter interface.
func (v *VM) GetCurrentScriptHash() util.Uint160 {
	return v.getContextScriptHash(0)
}

// toInt converts an item to a 32-bit int.
func toInt(i *big.Int) int {
	if !i.IsInt64() {
		panic("not an int32")
	}
	n := i.Int64()
	if n < math.MinInt32 || n > math.MaxInt32 {
		panic("not an int32")
	}
	return int(n)
}
