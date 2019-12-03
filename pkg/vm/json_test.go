package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
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
		// FIXME remove when NEO 3.0 https://github.com/nspcc-dev/neo-go/issues/477
		ScriptTable []map[string]vmUTScript
	}

	vmUTExecutionContextState struct {
		Instruction        string          `json:"nextInstruction"`
		InstructionPointer int             `json:"instructionPointer"`
		AStack             []vmUTStackItem `json:"altStack"`
		EStack             []vmUTStackItem `json:"evaluationStack"`
	}

	vmUTExecutionEngineState struct {
		State           vmUTState                   `json:"state"`
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

	vmUTState State

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
	vmExecute  vmUTActionType = "Execute"
	vmStepInto vmUTActionType = "StepInto"
	vmStepOut  vmUTActionType = "StepOut"
	vmStepOver vmUTActionType = "StepOver"

	typeArray     vmUTStackItemType = "Array"
	typeBoolean   vmUTStackItemType = "Boolean"
	typeByteArray vmUTStackItemType = "ByteArray"
	typeInteger   vmUTStackItemType = "Integer"
	typeInterop   vmUTStackItemType = "Interop"
	typeMap       vmUTStackItemType = "Map"
	typeString    vmUTStackItemType = "String"
	typeStruct    vmUTStackItemType = "Struct"

	testsDir = "neo-vm/tests/neo-vm.Tests/Tests/"
)

func TestUT(t *testing.T) {
	err := filepath.Walk(testsDir, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		testFile(t, path)
		return nil
	})

	require.NoError(t, err)
}

func testFile(t *testing.T, filename string) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	// FIXME remove when NEO 3.0 https://github.com/nspcc-dev/neo-go/issues/477
	if len(data) > 2 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		data = data[3:]
	}

	ut := new(vmUT)
	if err = json.Unmarshal(data, ut); err != nil {
		t.Fatal(err)
	}

	t.Run(ut.Category+":"+ut.Name, func(t *testing.T) {
		for i := range ut.Tests {
			test := ut.Tests[i]
			t.Run(ut.Tests[i].Name, func(t *testing.T) {
				prog := []byte(test.Script)
				vm := load(prog)
				vm.state = breakState

				// FIXME remove when NEO 3.0 https://github.com/nspcc-dev/neo-go/issues/477
				vm.getScript = getScript(test.ScriptTable)

				// FIXME in NEO 3.0 it is []byte{0x77, 0x77, 0x77, 0x77} https://github.com/nspcc-dev/neo-go/issues/477
				vm.RegisterInteropFunc("Test.ExecutionEngine.GetScriptContainer", InteropFunc(func(v *VM) error {
					v.estack.Push(&Element{value: (*InteropItem)(nil)})
					return nil
				}), 0)
				vm.RegisterInteropFunc("System.ExecutionEngine.GetScriptContainer", InteropFunc(func(v *VM) error {
					v.estack.Push(&Element{value: (*InteropItem)(nil)})
					return nil
				}), 0)

				for i := range test.Steps {
					execStep(t, vm, test.Steps[i])
					result := test.Steps[i].Result
					require.Equal(t, State(result.State), vm.state)
					if result.State == vmUTState(faultState) { // do not compare stacks on fault
						continue
					}

					if len(result.InvocationStack) > 0 {
						for i, s := range result.InvocationStack {
							ctx := vm.istack.Peek(i).Value().(*Context)
							if ctx.nextip < len(ctx.prog) {
								require.Equal(t, s.InstructionPointer, ctx.nextip)
								require.Equal(t, s.Instruction, opcode.Opcode(ctx.prog[ctx.nextip]).String())
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

func getScript(scripts []map[string]vmUTScript) func(util.Uint160) []byte {
	store := make(map[util.Uint160][]byte)
	for i := range scripts {
		for _, v := range scripts[i] {
			store[hash.Hash160(v)] = []byte(v)
		}
	}

	return func(a util.Uint160) []byte { return store[a] }
}

func compareItems(t *testing.T, a, b StackItem) {
	switch si := a.(type) {
	case *BigIntegerItem:
		val := si.value.Int64()
		switch ac := b.(type) {
		case *BigIntegerItem:
			require.Equal(t, val, ac.value.Int64())
		case *ByteArrayItem:
			require.Equal(t, val, new(big.Int).SetBytes(util.ArrayReverse(ac.value)).Int64())
		case *BoolItem:
			if ac.value {
				require.Equal(t, val, int64(1))
			} else {
				require.Equal(t, val, int64(0))
			}
		default:
			require.Fail(t, "wrong type")
		}
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
	switch v.Type {
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
			var item vmUTStackItem
			_ = json.Unmarshal([]byte(`"`+k+`"`), &item)
			result.Add(item.toStackItem(), v.toStackItem())
		}
		return result
	case typeInterop:
		panic("not implemented")
	case typeByteArray:
		return &ByteArrayItem{
			v.Value.([]byte),
		}
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
		panic("invalid type")
	}
}

func execStep(t *testing.T, v *VM, step vmUTStep) {
	for i, a := range step.Actions {
		var err error
		switch a {
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

func (v *vmUTState) UnmarshalJSON(data []byte) error {
	switch s := string(data); s {
	case `"Break"`:
		*v = vmUTState(breakState)
	case `"Fault"`:
		*v = vmUTState(faultState)
	case `"Halt"`:
		*v = vmUTState(haltState)
	default:
		panic(fmt.Sprintf("invalid state: %s", s))
	}
	return nil
}

func (v *vmUTScript) UnmarshalJSON(data []byte) error {
	b, err := decodeBytes(data)
	if err != nil {
		return err
	}

	*v = vmUTScript(b)
	return nil
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

	switch si.Type {
	case typeArray, typeStruct:
		var a []vmUTStackItem
		if err := json.Unmarshal(si.Value, &a); err != nil {
			return err
		}
		v.Value = a
	case typeInteger:
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
		v.Value = num
	case typeBoolean:
		var b bool
		if err := json.Unmarshal(si.Value, &b); err != nil {
			return err
		}
		v.Value = b
	case typeByteArray:
		b, err := decodeBytes(si.Value)
		if err != nil {
			return err
		}
		v.Value = b
	case typeInterop:
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

	hdata := data[3 : len(data)-1]
	if b, err := hex.DecodeString(string(hdata)); err == nil {
		return b, nil
	}

	data = data[1 : len(data)-1]
	r := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	return ioutil.ReadAll(r)
}
