package vm

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"reflect"
	"text/tabwriter"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Mode configures behaviour of the VM.
type Mode uint

// Available VM Modes.
var (
	ModeMute Mode = 1 << 0
)

// VM represents the virtual machine.
type VM struct {
	state State

	// registered interop hooks.
	interop map[string]InteropFunc

	// scripts loaded in memory.
	scripts map[util.Uint160][]byte

	istack *Stack // invocation stack.
	estack *Stack // execution stack.
	astack *Stack // alt stack.

	// Mute all output after execution.
	mute bool
}

// New returns a new VM object ready to load .avm bytecode scripts.
func New(mode Mode) *VM {
	vm := &VM{
		interop: make(map[string]InteropFunc),
		scripts: make(map[util.Uint160][]byte),
		state:   haltState,
		istack:  NewStack("invocation"),
		estack:  NewStack("evaluation"),
		astack:  NewStack("alt"),
	}
	if mode == ModeMute {
		vm.mute = true
	}

	// Register native interop hooks.
	vm.RegisterInteropFunc("Neo.Runtime.Log", runtimeLog)
	vm.RegisterInteropFunc("Neo.Runtime.Notify", runtimeNotify)

	return vm
}

// RegisterInteropFunc will register the given InteropFunc to the VM.
func (v *VM) RegisterInteropFunc(name string, f InteropFunc) {
	v.interop[name] = f
}

// Estack will return the evaluation stack so interop hooks can utilize this.
func (v *VM) Estack() *Stack {
	return v.estack
}

// Astack will return the alt stack so interop hooks can utilize this.
func (v *VM) Astack() *Stack {
	return v.astack
}

// Istack will return the invocation stack so interop hooks can utilize this.
func (v *VM) Istack() *Stack {
	return v.istack
}

// LoadArgs will load in the arguments used in the Mian entry point.
func (v *VM) LoadArgs(method []byte, args []StackItem) {
	if len(args) > 0 {
		v.estack.PushVal(args)
	}
	if method != nil {
		v.estack.PushVal(method)
	}
}

// PrintOps will print the opcodes of the current loaded program to stdout.
func (v *VM) PrintOps() {
	prog := v.Context().Program()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "INDEX\tOPCODE\tDESC\t")
	cursor := ""
	ip, _ := v.Context().CurrInstr()
	for i := 0; i < len(prog); i++ {
		if i == ip {
			cursor = "<<"
		} else {
			cursor = ""
		}
		fmt.Fprintf(w, "%d\t0x%2x\t%s\t%s\n", i, prog[i], Instruction(prog[i]).String(), cursor)

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

// LoadFile will load a program from the given path, ready to execute it.
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
	// clear all stacks, it could be a reload.
	v.istack.Clear()
	v.estack.Clear()
	v.astack.Clear()
	v.istack.PushVal(NewContext(prog))
}

// LoadScript will load a script from the internal script table. It
// will immediately push a new context created from this script to
// the invocation stack and starts executing it.
func (v *VM) LoadScript(b []byte) {
	ctx := NewContext(b)
	v.istack.PushVal(ctx)
}

// Context returns the current executed context. Nil if there is no context,
// which implies no program is loaded.
func (v *VM) Context() *Context {
	if v.istack.Len() == 0 {
		return nil
	}
	return v.istack.Peek(0).value.Value().(*Context)
}

// PopResult is used to pop the first item of the evaluation stack. This allows
// us to test compiler and vm in a bi-directional way.
func (v *VM) PopResult() interface{} {
	return v.estack.Pop().value.Value()
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
	return buildStackOutput(s)
}

// Ready return true if the VM ready to execute the loaded program.
// Will return false if no program is loaded.
func (v *VM) Ready() bool {
	return v.istack.Len() > 0
}

// Run starts the execution of the loaded program.
func (v *VM) Run() {
	if !v.Ready() {
		fmt.Println("no program loaded")
		return
	}

	v.state = noneState
	for {
		switch {
		case v.state.HasFlag(haltState):
			if !v.mute {
				fmt.Println(v.Stack("estack"))
			}
			return
		case v.state.HasFlag(breakState):
			ctx := v.Context()
			i, op := ctx.CurrInstr()
			fmt.Printf("at breakpoint %d (%s)\n", i, op.String())
			return
		case v.state.HasFlag(faultState):
			fmt.Println("FAULT")
			return
		case v.state == noneState:
			v.Step()
		}
	}
}

// Step 1 instruction in the program.
func (v *VM) Step() {
	ctx := v.Context()
	op := ctx.Next()
	v.execute(ctx, op)

	// re-peek the context as it could been changed during execution.
	cctx := v.Context()
	if cctx != nil && cctx.atBreakPoint() {
		v.state = breakState
	}
}

// execute performs an instruction cycle in the VM. Acting on the instruction (opcode).
func (v *VM) execute(ctx *Context, op Instruction) {
	// Instead of polluting the whole VM logic with error handling, we will recover
	// each panic at a central point, putting the VM in a fault state.
	defer func() {
		if err := recover(); err != nil {
			log.Printf("error encountered at instruction %d (%s)", ctx.ip, op)
			log.Println(err)
			v.state = faultState
		}
	}()

	if op >= PUSHBYTES1 && op <= PUSHBYTES75 {
		b := ctx.readBytes(int(op))
		if b == nil {
			panic("failed to read instruction parameter")
		}
		v.estack.PushVal(b)
		return
	}

	switch op {
	case PUSHM1, PUSH1, PUSH2, PUSH3, PUSH4, PUSH5,
		PUSH6, PUSH7, PUSH8, PUSH9, PUSH10, PUSH11,
		PUSH12, PUSH13, PUSH14, PUSH15, PUSH16:
		val := int(op) - int(PUSH1) + 1
		v.estack.PushVal(val)

	case PUSH0:
		v.estack.PushVal([]byte{})

	case PUSHDATA1:
		n := ctx.readByte()
		b := ctx.readBytes(int(n))
		if b == nil {
			panic("failed to read instruction parameter")
		}
		v.estack.PushVal(b)

	case PUSHDATA2:
		n := ctx.readUint16()
		b := ctx.readBytes(int(n))
		if b == nil {
			panic("failed to read instruction parameter")
		}
		v.estack.PushVal(b)

	case PUSHDATA4:
		n := ctx.readUint32()
		b := ctx.readBytes(int(n))
		if b == nil {
			panic("failed to read instruction parameter")
		}
		v.estack.PushVal(b)

	// Stack operations.
	case TOALTSTACK:
		v.astack.Push(v.estack.Pop())

	case FROMALTSTACK:
		v.estack.Push(v.astack.Pop())

	case DUPFROMALTSTACK:
		v.estack.Push(v.astack.Dup(0))

	case DUP:
		v.estack.Push(v.estack.Dup(0))

	case SWAP:
		a := v.estack.Pop()
		b := v.estack.Pop()
		v.estack.Push(a)
		v.estack.Push(b)

	case TUCK:
		a := v.estack.Dup(0)
		if a == nil {
			panic("no top-level element found")
		}
		if v.estack.Len() < 2 {
			panic("can't TUCK with a one-element stack")
		}
		v.estack.InsertAt(a, 2)
	case CAT:
		b := v.estack.Pop().Bytes()
		a := v.estack.Pop().Bytes()
		ab := append(a, b...)
		v.estack.PushVal(ab)
	case SUBSTR:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		o := int(v.estack.Pop().BigInt().Int64())
		if o < 0 {
			panic("negative index")
		}
		s := v.estack.Pop().Bytes()
		if o > len(s) {
			panic("invalid offset")
		}
		last := l + o
		if last > len(s) {
			last = len(s)
		}
		v.estack.PushVal(s[o:last])
	case LEFT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		if t := len(s); l > t {
			l = t
		}
		v.estack.PushVal(s[:l])
	case RIGHT:
		l := int(v.estack.Pop().BigInt().Int64())
		if l < 0 {
			panic("negative length")
		}
		s := v.estack.Pop().Bytes()
		v.estack.PushVal(s[len(s)-l:])
	case XDROP:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("invalid length")
		}
		e := v.estack.RemoveAt(n)
		if e == nil {
			panic("bad index")
		}

	case XSWAP:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("XSWAP: invalid length")
		}

		// Swap values of elements instead of reordering stack elements.
		if n > 0 {
			a := v.estack.Peek(n)
			b := v.estack.Peek(0)
			aval := a.value
			bval := b.value
			a.value = bval
			b.value = aval
		}

	case XTUCK:
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

	case ROT:
		e := v.estack.RemoveAt(2)
		if e == nil {
			panic("no top-level element found")
		}
		v.estack.Push(e)

	case DEPTH:
		v.estack.PushVal(v.estack.Len())

	case NIP:
		elem := v.estack.RemoveAt(1)
		if elem == nil {
			panic("no second element found")
		}

	case OVER:
		a := v.estack.Peek(1)
		if a == nil {
			panic("no second element found")
		}
		v.estack.Push(a)

	case PICK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("negative stack item returned")
		}
		a := v.estack.Peek(n)
		if a == nil {
			panic("no nth element found")
		}
		v.estack.Push(a)

	case ROLL:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 {
			panic("negative stack item returned")
		}
		if n > 0 {
			e := v.estack.RemoveAt(n)
			if e == nil {
				panic("bad index")
			}
			v.estack.Push(e)
		}

	case DROP:
		if v.estack.Len() < 1 {
			panic("stack is too small")
		}
		v.estack.Pop()

	case EQUAL:
		b := v.estack.Pop()
		if b == nil {
			panic("no top-level element found")
		}
		a := v.estack.Pop()
		if a == nil {
			panic("no second-to-the-top element found")
		}
		if ta, ok := a.value.(*ArrayItem); ok {
			if tb, ok := b.value.(*ArrayItem); ok {
				v.estack.PushVal(ta == tb)
				break
			}
		}
		v.estack.PushVal(reflect.DeepEqual(a, b))

	// Bit operations.
	case INVERT:
		// inplace
		a := v.estack.Peek(0).BigInt()
		a.Not(a)

	case AND:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).And(b, a))

	case OR:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Or(b, a))

	case XOR:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Xor(b, a))

	// Numeric operations.
	case ADD:
		a := v.estack.Pop().BigInt()
		b := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Add(a, b))

	case SUB:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Sub(a, b))

	case DIV:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Div(a, b))

	case MUL:
		a := v.estack.Pop().BigInt()
		b := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Mul(a, b))

	case MOD:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Mod(a, b))

	case SHL:
		b := v.estack.Pop().BigInt()
		if b.Int64() == 0 {
			return
		}
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Lsh(a, uint(b.Int64())))

	case SHR:
		b := v.estack.Pop().BigInt()
		if b.Int64() == 0 {
			return
		}
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Rsh(a, uint(b.Int64())))

	case BOOLAND:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a && b)

	case BOOLOR:
		b := v.estack.Pop().Bool()
		a := v.estack.Pop().Bool()
		v.estack.PushVal(a || b)

	case NUMEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == 0)

	case NUMNOTEQUAL:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) != 0)

	case LT:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == -1)

	case GT:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) == 1)

	case LTE:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) <= 0)

	case GTE:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(b) >= 0)

	case MIN:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		val := a
		if a.Cmp(b) == 1 {
			val = b
		}
		v.estack.PushVal(val)

	case MAX:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		val := a
		if a.Cmp(b) == -1 {
			val = b
		}
		v.estack.PushVal(val)

	case WITHIN:
		b := v.estack.Pop().BigInt()
		a := v.estack.Pop().BigInt()
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(a.Cmp(x) <= 0 && x.Cmp(b) == -1)

	case INC:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Add(x, big.NewInt(1)))

	case DEC:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(new(big.Int).Sub(x, big.NewInt(1)))

	case SIGN:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Sign())

	case NEGATE:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Neg(x))

	case ABS:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Abs(x))

	case NOT:
		x := v.estack.Pop().Bool()
		v.estack.PushVal(!x)

	case NZ:
		x := v.estack.Pop().BigInt()
		v.estack.PushVal(x.Cmp(big.NewInt(0)) != 0)

	// Object operations.
	case NEWARRAY:
		item := v.estack.Pop()
		switch t := item.value.(type) {
		case *StructItem:
			v.estack.PushVal(&ArrayItem{t.value})
		case *ArrayItem:
			v.estack.PushVal(t)
		default:
			n := item.BigInt()
			items := makeArrayOfFalses(int(n.Int64()))
			v.estack.PushVal(&ArrayItem{items})
		}

	case NEWSTRUCT:
		item := v.estack.Pop()
		switch t := item.value.(type) {
		case *ArrayItem:
			v.estack.PushVal(&StructItem{t.value})
		case *StructItem:
			v.estack.PushVal(t)
		default:
			n := item.BigInt()
			items := makeArrayOfFalses(int(n.Int64()))
			v.estack.PushVal(&StructItem{items})
		}

	case APPEND:
		itemElem := v.estack.Pop()
		arrElem := v.estack.Pop()

		val := itemElem.value
		if t, ok := itemElem.value.(*StructItem); ok {
			val = t.Clone()
		}

		switch t := arrElem.value.(type) {
		case *ArrayItem:
			arr := t.Value().([]StackItem)
			arr = append(arr, val)
			t.value = arr
		case *StructItem:
			arr := t.Value().([]StackItem)
			arr = append(arr, val)
			t.value = arr
		default:
			panic("APPEND: not of underlying type Array")
		}

	case PACK:
		n := int(v.estack.Pop().BigInt().Int64())
		if n < 0 || n > v.estack.Len() {
			panic("OPACK: invalid length")
		}

		items := make([]StackItem, n)
		for i := 0; i < n; i++ {
			items[i] = v.estack.Pop().value
		}

		v.estack.PushVal(items)

	case UNPACK:
		a := v.estack.Pop().Array()
		l := len(a)
		for i := l - 1; i >= 0; i-- {
			v.estack.PushVal(a[i])
		}
		v.estack.PushVal(l)

	case PICKITEM:
		var (
			key   = v.estack.Pop()
			obj   = v.estack.Pop()
			index = int(key.BigInt().Int64())
		)

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *ArrayItem, *StructItem:
			arr := t.Value().([]StackItem)
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index]
			v.estack.PushVal(item)
		default:
			arr := obj.Bytes()
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index]
			v.estack.PushVal(int(item))
		}

	case SETITEM:
		var (
			item  = v.estack.Pop().value
			key   = v.estack.Pop()
			obj   = v.estack.Pop()
			index = int(key.BigInt().Int64())
		)

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *ArrayItem, *StructItem:
			arr := t.Value().([]StackItem)
			if index < 0 || index >= len(arr) {
				panic("SETITEM: invalid index")
			}
			arr[index] = item
		default:
			panic(fmt.Sprintf("SETITEM: invalid item type %s", t))
		}

	case REVERSE:
		a := v.estack.Pop().Array()
		if len(a) > 1 {
			for i, j := 0, len(a)-1; i <= j; i, j = i+1, j-1 {
				a[i], a[j] = a[j], a[i]
			}
		}
	case REMOVE:
		key := int(v.estack.Pop().BigInt().Int64())
		elem := v.estack.Pop()
		switch t := elem.value.(type) {
		case *ArrayItem:
			a := t.value
			if key < 0 || key >= len(a) {
				panic("REMOVE: invalid index")
			}
			a = append(a[:key], a[key+1:]...)
			t.value = a
		case *StructItem:
			a := t.value
			if key < 0 || key >= len(a) {
				panic("REMOVE: invalid index")
			}
			a = append(a[:key], a[key+1:]...)
			t.value = a
		default:
			panic("REMOVE: invalid type")
		}

	case ARRAYSIZE:
		elem := v.estack.Pop()
		// Cause there is no native (byte) item type here, hence we need to check
		// the type of the item for array size operations.
		switch t := elem.value.Value().(type) {
		case []StackItem:
			v.estack.PushVal(len(t))
		case []uint8:
			v.estack.PushVal(len(t))
		default:
			panic("ARRAYSIZE: item not of type []StackItem")
		}

	case SIZE:
		elem := v.estack.Pop()
		arr := elem.Bytes()
		v.estack.PushVal(len(arr))

	case JMP, JMPIF, JMPIFNOT:
		var (
			rOffset = int16(ctx.readUint16())
			offset  = ctx.ip + int(rOffset) - 3 // sizeOf(int16 + uint8)
		)
		if offset < 0 || offset > len(ctx.prog) {
			panic(fmt.Sprintf("JMP: invalid offset %d ip at %d", offset, ctx.ip))
		}
		cond := true
		if op > JMP {
			cond = v.estack.Pop().Bool()
			if op == JMPIFNOT {
				cond = !cond
			}
		}
		if cond {
			ctx.ip = offset
		}

	case CALL:
		v.istack.PushVal(ctx.Copy())
		ctx.ip += 2
		v.execute(v.Context(), JMP)

	case SYSCALL:
		api := ctx.readVarBytes()
		ifunc, ok := v.interop[string(api)]
		if !ok {
			panic(fmt.Sprintf("interop hook (%s) not registered", api))
		}
		if err := ifunc(v); err != nil {
			panic(fmt.Sprintf("failed to invoke syscall: %s", err))
		}

	case APPCALL, TAILCALL:
		if len(v.scripts) == 0 {
			panic("script table is empty")
		}

		hash, err := util.Uint160DecodeBytes(ctx.readBytes(20))
		if err != nil {
			panic(err)
		}

		script, ok := v.scripts[hash]
		if !ok {
			panic("could not find script")
		}

		if op == TAILCALL {
			_ = v.istack.Pop()
		}

		v.LoadScript(script)

	case RET:
		_ = v.istack.Pop()
		if v.istack.Len() == 0 {
			v.state = haltState
		}

	case CHECKSIG, VERIFY, CHECKMULTISIG, NEWMAP, HASKEY, KEYS, VALUES:
		panic("unimplemented")

	// Cryptographic operations.
	case SHA1:
		b := v.estack.Pop().Bytes()
		sha := sha1.New()
		sha.Write(b)
		v.estack.PushVal(sha.Sum(nil))

	case SHA256:
		b := v.estack.Pop().Bytes()
		v.estack.PushVal(hash.Sha256(b).Bytes())

	case HASH160:
		b := v.estack.Pop().Bytes()
		v.estack.PushVal(hash.Hash160(b).Bytes())

	case HASH256:
		b := v.estack.Pop().Bytes()
		v.estack.PushVal(hash.DoubleSha256(b).Bytes())

	case NOP:
		// unlucky ^^

	case THROW:
		panic("THROW")

	case THROWIFNOT:
		if !v.estack.Pop().Bool() {
			panic("THROWIFNOT")
		}

	default:
		panic(fmt.Sprintf("unknown opcode %s", op.String()))
	}
}

func makeArrayOfFalses(n int) []StackItem {
	items := make([]StackItem, n)
	for i := range items {
		items[i] = &BoolItem{false}
	}
	return items
}

func init() {
	log.SetPrefix("NEO-GO-VM > ")
	log.SetFlags(0)
}
