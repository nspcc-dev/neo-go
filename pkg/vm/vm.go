package vm

import (
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"

	"golang.org/x/crypto/ripemd160"
)

type VM struct {
	state State

	// interop layer.
	interop *InteropService

	istack *Stack // invocation stack.
	estack *Stack // execution stack.
	astack *Stack // alt stack.
}

// New returns a new VM object ready to load .avm bytecode scripts.
func New(svc *InteropService) *VM {
	if svc == nil {
		svc = NewInteropService()
	}
	return &VM{
		interop: svc,
		state:   haltState,
		istack:  NewStack("invocation"),
		estack:  NewStack("evaluation"),
		astack:  NewStack("alt"),
	}
}

func (v *VM) Load(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	v.istack.PushVal(NewContext(b))
	return nil
}

func (v *VM) context() *Context {
	return v.istack.Peek(0).value.Value().(*Context)
}

// Run starts the execution of the loaded program.
func (v *VM) Run() error {
	if v.istack.Len() == 0 {
		return errors.New("no program loaded")
	}

	v.state &= breakState
	for v.state&haltState == 0 && v.state&faultState == 0 && v.state&breakState == 0 {
		v.Step()
	}

	v.PrintState()

	return nil
}

// Step 1 instruction in the program.
func (v *VM) Step() {
	ctx := v.context()
	v.execute(ctx, ctx.Next())

	if ctx.ip == len(ctx.prog) {
		v.state = breakState
	}
}

// execute performs an instruction cycle in the VM. Acting on the instruction (opcode).
func (v *VM) execute(ctx *Context, op Opcode) {
	log.Printf("%d\t%s\n", ctx.ip, op)

	// Instead of poluting the whole VM logic with error handling, we will recover
	// each panic at a central point, putting the VM in a fault state.
	defer func() {
		if err := recover(); err != nil {
			log.Printf("error encountered at instruction %d (%s)", ctx.ip, op)
			log.Println(err)
			v.state |= faultState
		}
	}()

	if op >= Opushbytes1 && op <= Opushbytes75 {
		b := ctx.readBytes(int(op))
		v.estack.PushVal(b)
		return
	}

	switch op {
	case Opushm1, Opush1, Opush2, Opush3, Opush4, Opush5,
		Opush6, Opush7, Opush8, Opush9, Opush10, Opush11,
		Opush12, Opush13, Opush14, Opush15, Opush16:
		val := int(op) - int(Opush1) + 1
		v.estack.PushVal(val)

	case Opush0:
		v.estack.PushVal(0)

	case Opushdata1:
		n := ctx.readByte()
		b := ctx.readBytes(int(n))
		v.estack.PushVal(b)

	case Opushdata2:
		n := ctx.readUint16()
		b := ctx.readBytes(int(n))
		v.estack.PushVal(b)

	case Opushdata4:
		n := ctx.readUint32()
		b := ctx.readBytes(int(n))
		v.estack.PushVal(b)

	// Stack operations.

	case Otoaltstack:
		v.astack.Push(v.estack.Pop())

	case Ofromaltstack:
		v.estack.Push(v.astack.Pop())

	case Odupfromaltstack:
		v.estack.Push(v.astack.Dup(0))

	case Odup:
		v.estack.Push(v.estack.Dup(0))

	case Oswap:
		a := v.estack.Pop()
		b := v.estack.Pop()
		v.estack.Push(a)
		v.estack.Push(b)

	case Oroll:
		n := int(v.estack.Pop().GetBigInt().Int64())
		if n < 0 {
			panic("negative stack item returned")
		}
		if n > 0 {
			v.estack.Push(v.estack.RemoveAt(n - 1))
		}

	case Odrop:
		v.estack.Pop()

	case Oequal:
		panic("TODO EQUAL")

	// Bit operations.
	case Oand:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(b.And(b, a))

	case Oor:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(b.Or(b, a))

	case Oxor:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(b.Xor(b, a))

	// Numeric operations.
	case Oadd:
		a := v.estack.Pop().GetBigInt()
		b := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Add(a, b))

	case Osub:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Sub(a, b))

	case Odiv:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Div(a, b))

	case Omul:
		a := v.estack.Pop().GetBigInt()
		b := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Mul(a, b))

	case Omod:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Mod(a, b))

	case Oshl:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Lsh(a, uint(b.Int64())))

	case Oshr:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Rsh(a, uint(b.Int64())))

	case Obooland:
		b := v.estack.Pop().GetBool()
		a := v.estack.Pop().GetBool()
		v.estack.PushVal(a && b)

	case Oboolor:
		b := v.estack.Pop().GetBool()
		a := v.estack.Pop().GetBool()
		v.estack.PushVal(a || b)

	case Onumequal:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) == 0)

	case Onumnotequal:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) != 0)

	case Olt:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) == -1)

	case Ogt:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) == 1)

	case Olte:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) <= 0)

	case Ogte:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(b) >= 0)

	case Omin:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		val := a
		if a.Cmp(b) == 1 {
			val = b
		}
		v.estack.PushVal(val)

	case Omax:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		val := a
		if a.Cmp(b) == -1 {
			val = b
		}
		v.estack.PushVal(val)

	case Owithin:
		b := v.estack.Pop().GetBigInt()
		a := v.estack.Pop().GetBigInt()
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(a.Cmp(x) <= 0 && x.Cmp(b) == -1)

	case Oinc:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Add(x, big.NewInt(1)))

	case Odec:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Sub(x, big.NewInt(1)))

	case Osign:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Sign())

	case Onegate:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Neg(x))

	case Oabs:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Abs(x))

	case Onot:
		x := v.estack.Pop().GetBigInt()
		v.estack.PushVal(x.Not(x))

	case Onz:
		panic("todo NZ")
		// x := v.estack.Pop().GetBigInt()

	// Object operations.
	case Onewarray:
		n := v.estack.Pop().GetBigInt().Int64()
		items := make([]StackItem, n)
		v.estack.PushVal(&arrayItem{items})

	case Onewstruct:
		v.estack.Dump()
		n := v.estack.Pop().GetBigInt().Int64()
		items := make([]StackItem, n)
		v.estack.PushVal(&structItem{items})

	case Opack:
		n := int(v.estack.Pop().GetBigInt().Int64())
		if n < 0 || n > v.estack.Len() {
			panic("OPACK: invalid length")
		}

		items := make([]StackItem, n)
		for i := 0; i < n; i++ {
			items[i] = v.estack.Pop().value
		}

		v.estack.PushVal(items)

	case Opickitem:
		var (
			key   = v.estack.Pop()
			obj   = v.estack.Pop()
			index = int(key.GetBigInt().Int64())
		)

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *arrayItem, *structItem:
			arr := t.Value().([]StackItem)
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			item := arr[index]
			v.estack.PushVal(item)
		default:
			panic("PICKITEM: unknown type")
		}

	case Osetitem:
		var (
			obj   = v.estack.Pop()
			key   = v.estack.Pop()
			item  = v.estack.Pop().value
			index = int(key.GetBigInt().Int64())
		)

		switch t := obj.value.(type) {
		// Struct and Array items have their underlying value as []StackItem.
		case *arrayItem, *structItem:
			arr := t.Value().([]StackItem)
			if index < 0 || index >= len(arr) {
				panic("PICKITEM: invalid index")
			}
			arr[index] = item
		default:
			panic("SETITEM: unknown type")
		}

	case Oarraysize:
		elem := v.estack.Pop()
		arr, ok := elem.value.Value().([]StackItem)
		if !ok {
			panic("ARRAYSIZE: item not of type []StackItem")
		}
		v.estack.PushVal(len(arr))

	case Ojmp, Ojmpif, Ojmpifnot:
		rOffset := ctx.readUint16()
		offset := ctx.ip + int(rOffset) - 3 // sizeOf(uint16 + uint8)
		if offset < 0 || offset > len(ctx.prog) {
			panic("JMP: invalid offset")
		}

		cond := true
		if op > Ojmp {
		}
		if cond {
			log.Printf("jumping to %d\n", offset)
			ctx.ip = offset
		}

	case Ocall:
		v.istack.PushVal(ctx.Copy())
		ctx.ip += 2
		v.execute(v.context(), Ojmp)

	case Osyscall:
		api := ctx.readVarBytes()
		err := v.interop.Call(api, v)
		if err != nil {
			panic(fmt.Sprintf("failed to invoke syscall: %s", err))
		}

	case Oret:
		_ = v.istack.Pop()
		if v.istack.Len() == 0 {
			v.state |= haltState
		}

	// Cryptographic operations.
	case Osha1:
		b := v.estack.Pop().GetBytes()
		sha := sha1.New()
		sha.Write(b)
		v.estack.PushVal(sha.Sum(nil))

	case Osha256:
		b := v.estack.Pop().GetBytes()
		sha := sha256.New()
		sha.Write(b)
		v.estack.PushVal(sha.Sum(nil))

	case Ohash160:
		b := v.estack.Pop().GetBytes()
		sha := sha256.New()
		sha.Write(b)
		h := sha.Sum(nil)
		ripemd := ripemd160.New()
		ripemd.Write(h)
		v.estack.PushVal(ripemd.Sum(nil))

	case Ohash256:
		b := v.estack.Pop().GetBytes()
		sha := sha256.New()
		sha.Write(b)
		h := sha.Sum(nil)
		sha.Reset()
		sha.Write(h)
		v.estack.PushVal(sha.Sum(nil))

	case Ochecksig:

	case Ocheckmultisig:

	case Onop:
		// unlucky ^^

	case Othrow:
		panic("THROW")

	case Othrowifnot:
		if !v.estack.Pop().GetBool() {
			panic("THROWIFNOT")
		}

	default:
		panic(fmt.Sprintf("unknown opcode %s", op))
	}
}

func (v *VM) PrintState() {
	fmt.Println(v.state)
	v.estack.Dump()
	v.astack.Dump()
	v.istack.Dump()
}

func init() {
	log.SetPrefix("NEO-GO-VM > ")
	log.SetFlags(0)
}
