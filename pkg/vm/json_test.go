package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

type (
	vmUT struct {
		Category string      `json:"category"`
		Name     string      `json:"name"`
		Tests    []vmUTEntry `json:"tests"`
	}

	vmUTActionType string

	vmUTEntry struct {
		Name   string
		Script vmUTScript
		Steps  []vmUTStep
	}

	vmUTExecutionContextState struct {
		Instruction        string          `json:"nextInstruction"`
		InstructionPointer int             `json:"instructionPointer"`
		AStack             []vmUTStackItem `json:"altStack"`
		EStack             []vmUTStackItem `json:"evaluationStack"`
	}

	vmUTExecutionEngineState struct {
		State           State                       `json:"state"`
		ResultStack     []vmUTStackItem             `json:"resultStack"`
		InvocationStack []vmUTExecutionContextState `json:"invocationStack"`
	}

	vmUTScript []byte

	vmUTStackItem struct {
		Type  vmUTStackItemType
		Value interface{}
	}

	vmUTStep struct {
		Actions []vmUTActionType         `json:"actions"`
		Result  vmUTExecutionEngineState `json:"result"`
	}

	vmUTStackItemType string
)

// stackItemAUX is used as an intermediate structure
// to conditionally unmarshal vmUTStackItem based
// on the value of Type field.
type stackItemAUX struct {
	Type  vmUTStackItemType `json:"type"`
	Value json.RawMessage   `json:"value"`
}

const (
	vmExecute  vmUTActionType = "execute"
	vmStepInto vmUTActionType = "stepinto"
	vmStepOut  vmUTActionType = "stepout"
	vmStepOver vmUTActionType = "stepover"

	typeArray      vmUTStackItemType = "array"
	typeBoolean    vmUTStackItemType = "boolean"
	typeBuffer     vmUTStackItemType = "buffer"
	typeByteString vmUTStackItemType = "bytestring"
	typeInteger    vmUTStackItemType = "integer"
	typeInterop    vmUTStackItemType = "interop"
	typeMap        vmUTStackItemType = "map"
	typeNull       vmUTStackItemType = "null"
	typePointer    vmUTStackItemType = "pointer"
	typeString     vmUTStackItemType = "string"
	typeStruct     vmUTStackItemType = "struct"

	testsDir = "testdata/neo-vm/tests/neo-vm.Tests/Tests/"
)

func TestUT(t *testing.T) {
	t.Skip()
	testsRan := false
	err := filepath.Walk(testsDir, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		testFile(t, path)
		testsRan = true
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, true, testsRan, "neo-vm tests should be available (check submodules)")
}

func getTestingInterop(id uint32) *InteropFuncPrice {
	if id == binary.LittleEndian.Uint32([]byte{0x77, 0x77, 0x77, 0x77}) {
		return &InteropFuncPrice{InteropFunc(func(v *VM) error {
			v.estack.Push(&Element{value: (*InteropItem)(nil)})
			return nil
		}), 0}
	}
	return nil
}

func testFile(t *testing.T, filename string) {
	data, err := ioutil.ReadFile(filename)
	require.NoError(t, err)

	ut := new(vmUT)
	require.NoError(t, json.Unmarshal(data, ut))

	t.Run(ut.Category+":"+ut.Name, func(t *testing.T) {
		for i := range ut.Tests {
			test := ut.Tests[i]
			t.Run(ut.Tests[i].Name, func(t *testing.T) {
				prog := []byte(test.Script)
				vm := load(prog)
				vm.state = breakState
				vm.RegisterInteropGetter(getTestingInterop)

				for i := range test.Steps {
					execStep(t, vm, test.Steps[i])
					result := test.Steps[i].Result
					require.Equal(t, result.State, vm.state)
					if result.State == faultState { // do not compare stacks on fault
						continue
					}

					if len(result.InvocationStack) > 0 {
						for i, s := range result.InvocationStack {
							ctx := vm.istack.Peek(i).Value().(*Context)
							if ctx.nextip < len(ctx.prog) {
								require.Equal(t, s.InstructionPointer, ctx.nextip)
								op, err := opcode.FromString(s.Instruction)
								require.NoError(t, err)
								require.Equal(t, op, opcode.Opcode(ctx.prog[ctx.nextip]))
							}
							compareStacks(t, s.EStack, vm.estack)
							compareStacks(t, s.AStack, vm.astack)
						}
					}

					if len(result.ResultStack) != 0 {
						compareStacks(t, result.ResultStack, vm.estack)
					}
				}
			})
		}
	})
}

func compareItems(t *testing.T, a, b StackItem) {
	switch si := a.(type) {
	case *BigIntegerItem:
		val := si.value.Int64()
		switch ac := b.(type) {
		case *BigIntegerItem:
			require.Equal(t, val, ac.value.Int64())
		case *ByteArrayItem:
			require.Equal(t, val, emit.BytesToInt(ac.value).Int64())
		case *BoolItem:
			if ac.value {
				require.Equal(t, val, int64(1))
			} else {
				require.Equal(t, val, int64(0))
			}
		default:
			require.Fail(t, "wrong type")
		}
	case *PointerItem:
		p, ok := b.(*PointerItem)
		require.True(t, ok)
		require.Equal(t, si.pos, p.pos) // there no script in test files
	default:
		require.Equal(t, a, b)
	}
}

func compareStacks(t *testing.T, expected []vmUTStackItem, actual *Stack) {
	if expected == nil {
		return
	}

	require.Equal(t, len(expected), actual.Len())
	for i, item := range expected {
		e := actual.Peek(i)
		require.NotNil(t, e)

		if item.Type == typeInterop {
			require.IsType(t, (*InteropItem)(nil), e.value)
			continue
		}
		compareItems(t, item.toStackItem(), e.value)
	}
}

func (v *vmUTStackItem) toStackItem() StackItem {
	switch v.Type.toLower() {
	case typeArray:
		items := v.Value.([]vmUTStackItem)
		result := make([]StackItem, len(items))
		for i := range items {
			result[i] = items[i].toStackItem()
		}
		return &ArrayItem{
			value: result,
		}
	case typeString:
		panic("not implemented")
	case typeMap:
		items := v.Value.(map[string]vmUTStackItem)
		result := NewMapItem()
		for k, v := range items {
			item := jsonStringToInteger(k)
			if item == nil {
				panic(fmt.Sprintf("can't unmarshal StackItem %s", k))
			}
			result.Add(item, v.toStackItem())
		}
		return result
	case typeInterop:
		panic("not implemented")
	case typeByteString:
		return &ByteArrayItem{
			v.Value.([]byte),
		}
	case typeBuffer:
		return &BufferItem{v.Value.([]byte)}
	case typePointer:
		return NewPointerItem(v.Value.(int), nil)
	case typeNull:
		return NullItem{}
	case typeBoolean:
		return &BoolItem{
			v.Value.(bool),
		}
	case typeInteger:
		return &BigIntegerItem{
			value: v.Value.(*big.Int),
		}
	case typeStruct:
		items := v.Value.([]vmUTStackItem)
		result := make([]StackItem, len(items))
		for i := range items {
			result[i] = items[i].toStackItem()
		}
		return &StructItem{
			value: result,
		}
	default:
		panic(fmt.Sprintf("invalid type: %s", v.Type))
	}
}

func execStep(t *testing.T, v *VM, step vmUTStep) {
	for i, a := range step.Actions {
		var err error
		switch a.toLower() {
		case vmExecute:
			err = v.Run()
		case vmStepInto:
			err = v.StepInto()
		case vmStepOut:
			err = v.StepOut()
		case vmStepOver:
			err = v.StepOver()
		default:
			panic(fmt.Sprintf("invalid action: %s", a))
		}

		// only the last action is allowed to fail
		if i+1 < len(step.Actions) {
			require.NoError(t, err)
		}
	}
}

func jsonStringToInteger(s string) StackItem {
	b, err := decodeHex(s)
	if err == nil {
		return NewBigIntegerItem(new(big.Int).SetBytes(b))
	}
	return nil
}

func (v vmUTStackItemType) toLower() vmUTStackItemType {
	return vmUTStackItemType(strings.ToLower(string(v)))
}

func (v *vmUTScript) UnmarshalJSON(data []byte) error {
	var ops []string
	if err := json.Unmarshal(data, &ops); err != nil {
		return err
	}

	var script []byte
	for i := range ops {
		if b, ok := decodeSingle(ops[i]); ok {
			script = append(script, b...)
		} else {
			const regex = `(?P<hex>(?:0x)?[0-9a-zA-Z]+)\*(?P<num>[0-9]+)`
			re := regexp.MustCompile(regex)
			ss := re.FindStringSubmatch(ops[i])
			if len(ss) != 3 {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			b, ok := decodeSingle(ss[1])
			if !ok {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			num, err := strconv.Atoi(ss[2])
			if err != nil {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			for i := 0; i < num; i++ {
				script = append(script, b...)
			}
		}
	}

	*v = script
	return nil
}

func decodeSingle(s string) ([]byte, bool) {
	if op, err := opcode.FromString(s); err == nil {
		return []byte{byte(op)}, true
	}
	b, err := decodeHex(s)
	return b, err == nil
}

func (v vmUTActionType) toLower() vmUTActionType {
	return vmUTActionType(strings.ToLower(string(v)))
}

func (v *vmUTActionType) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, (*string)(v))
}

func (v *vmUTStackItem) UnmarshalJSON(data []byte) error {
	var si stackItemAUX
	if err := json.Unmarshal(data, &si); err != nil {
		return err
	}

	v.Type = si.Type

	switch typ := si.Type.toLower(); typ {
	case typeArray, typeStruct:
		var a []vmUTStackItem
		if err := json.Unmarshal(si.Value, &a); err != nil {
			return err
		}
		v.Value = a
	case typeInteger, typePointer:
		num := new(big.Int)
		var a int64
		var s string
		if err := json.Unmarshal(si.Value, &a); err == nil {
			num.SetInt64(a)
		} else if err := json.Unmarshal(si.Value, &s); err == nil {
			num.SetString(s, 10)
		} else {
			panic(fmt.Sprintf("invalid integer: %v", si.Value))
		}
		if typ == typePointer {
			v.Value = int(num.Int64())
		} else {
			v.Value = num
		}
	case typeBoolean:
		var b bool
		if err := json.Unmarshal(si.Value, &b); err != nil {
			return err
		}
		v.Value = b
	case typeByteString, typeBuffer:
		b, err := decodeBytes(si.Value)
		if err != nil {
			return err
		}
		v.Value = b
	case typeInterop, typeNull:
		v.Value = nil
	case typeMap:
		var m map[string]vmUTStackItem
		if err := json.Unmarshal(si.Value, &m); err != nil {
			return err
		}
		v.Value = m
	case typeString:
		panic("not implemented")
	default:
		panic(fmt.Sprintf("unknown type: %s", si.Type))
	}
	return nil
}

// decodeBytes tries to decode bytes from string.
// It tries hex and base64 encodings.
func decodeBytes(data []byte) ([]byte, error) {
	if len(data) == 2 {
		return []byte{}, nil
	}

	data = data[1 : len(data)-1] // strip quotes
	if b, err := decodeHex(string(data)); err == nil {
		return b, nil
	}

	r := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	return ioutil.ReadAll(r)
}

func decodeHex(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return hex.DecodeString(s)
}
