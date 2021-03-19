package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/gas"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/nameservice"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/notary"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/policy"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/roles"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestContractHashes(t *testing.T) {
	cs := native.NewContracts(true, map[string][]uint32{})
	require.Equal(t, []byte(neo.Hash), cs.NEO.Hash.BytesBE())
	require.Equal(t, []byte(gas.Hash), cs.GAS.Hash.BytesBE())
	require.Equal(t, []byte(oracle.Hash), cs.Oracle.Hash.BytesBE())
	require.Equal(t, []byte(roles.Hash), cs.Designate.Hash.BytesBE())
	require.Equal(t, []byte(policy.Hash), cs.Policy.Hash.BytesBE())
	require.Equal(t, []byte(nameservice.Hash), cs.NameService.Hash.BytesBE())
	require.Equal(t, []byte(ledger.Hash), cs.Ledger.Hash.BytesBE())
	require.Equal(t, []byte(management.Hash), cs.Management.Hash.BytesBE())
	require.Equal(t, []byte(notary.Hash), cs.Notary.Hash.BytesBE())
	require.Equal(t, []byte(crypto.Hash), cs.Crypto.Hash.BytesBE())
	require.Equal(t, []byte(std.Hash), cs.Std.Hash.BytesBE())
}

// testPrintHash is a helper for updating contract hashes.
func testPrintHash(u util.Uint160) {
	fmt.Print(`"`)
	for _, b := range u.BytesBE() {
		fmt.Printf("\\x%02x", b)
	}
	fmt.Println(`"`)
}

func TestContractParameterTypes(t *testing.T) {
	require.EqualValues(t, management.AnyType, smartcontract.AnyType)
	require.EqualValues(t, management.BoolType, smartcontract.BoolType)
	require.EqualValues(t, management.IntegerType, smartcontract.IntegerType)
	require.EqualValues(t, management.ByteArrayType, smartcontract.ByteArrayType)
	require.EqualValues(t, management.StringType, smartcontract.StringType)
	require.EqualValues(t, management.Hash160Type, smartcontract.Hash160Type)
	require.EqualValues(t, management.Hash256Type, smartcontract.Hash256Type)
	require.EqualValues(t, management.PublicKeyType, smartcontract.PublicKeyType)
	require.EqualValues(t, management.SignatureType, smartcontract.SignatureType)
	require.EqualValues(t, management.ArrayType, smartcontract.ArrayType)
	require.EqualValues(t, management.MapType, smartcontract.MapType)
	require.EqualValues(t, management.InteropInterfaceType, smartcontract.InteropInterfaceType)
	require.EqualValues(t, management.VoidType, smartcontract.VoidType)
}

func TestRoleManagementRole(t *testing.T) {
	require.EqualValues(t, native.RoleOracle, roles.Oracle)
	require.EqualValues(t, native.RoleStateValidator, roles.StateValidator)
	require.EqualValues(t, native.RoleNeoFSAlphabet, roles.NeoFSAlphabet)
	require.EqualValues(t, native.RoleP2PNotary, roles.P2PNotary)
}

func TestNameServiceRecordType(t *testing.T) {
	require.EqualValues(t, native.RecordTypeA, nameservice.TypeA)
	require.EqualValues(t, native.RecordTypeCNAME, nameservice.TypeCNAME)
	require.EqualValues(t, native.RecordTypeTXT, nameservice.TypeTXT)
	require.EqualValues(t, native.RecordTypeAAAA, nameservice.TypeAAAA)
}

func TestCryptoLibNamedCurve(t *testing.T) {
	require.EqualValues(t, native.Secp256k1, crypto.Secp256k1)
	require.EqualValues(t, native.Secp256r1, crypto.Secp256r1)
}

type nativeTestCase struct {
	method string
	params []string
}

// Here we test that corresponding method does exist, is invoked and correct value is returned.
func TestNativeHelpersCompile(t *testing.T) {
	cs := native.NewContracts(true, map[string][]uint32{})
	u160 := `interop.Hash160("aaaaaaaaaaaaaaaaaaaa")`
	u256 := `interop.Hash256("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")`
	pub := `interop.PublicKey("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")`
	sig := `interop.Signature("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")`
	nep17TestCases := []nativeTestCase{
		{"balanceOf", []string{u160}},
		{"decimals", nil},
		{"symbol", nil},
		{"totalSupply", nil},
		{"transfer", []string{u160, u160, "123", "nil"}},
	}
	runNativeTestCases(t, cs.NEO.ContractMD, "neo", append([]nativeTestCase{
		{"getCandidates", nil},
		{"getCommittee", nil},
		{"getGasPerBlock", nil},
		{"getNextBlockValidators", nil},
		{"getRegisterPrice", nil},
		{"registerCandidate", []string{pub}},
		{"setGasPerBlock", []string{"1"}},
		{"setRegisterPrice", []string{"10"}},
		{"vote", []string{u160, pub}},
		{"unclaimedGas", []string{u160, "123"}},
		{"unregisterCandidate", []string{pub}},
	}, nep17TestCases...))
	runNativeTestCases(t, cs.GAS.ContractMD, "gas", nep17TestCases)
	runNativeTestCases(t, cs.Oracle.ContractMD, "oracle", []nativeTestCase{
		{"getPrice", nil},
		{"request", []string{`"url"`, "nil", `"callback"`, "nil", "123"}},
		{"setPrice", []string{"10"}},
	})
	runNativeTestCases(t, cs.Designate.ContractMD, "roles", []nativeTestCase{
		{"designateAsRole", []string{"1", "[]interop.PublicKey{}"}},
		{"getDesignatedByRole", []string{"1", "1000"}},
	})
	runNativeTestCases(t, cs.Policy.ContractMD, "policy", []nativeTestCase{
		{"blockAccount", []string{u160}},
		{"getExecFeeFactor", nil},
		{"getFeePerByte", nil},
		{"getStoragePrice", nil},
		{"isBlocked", []string{u160}},
		{"setExecFeeFactor", []string{"42"}},
		{"setFeePerByte", []string{"42"}},
		{"setStoragePrice", []string{"42"}},
		{"unblockAccount", []string{u160}},
	})
	runNativeTestCases(t, cs.NameService.ContractMD, "nameservice", []nativeTestCase{
		// nonfungible
		{"symbol", nil},
		{"decimals", nil},
		{"totalSupply", nil},
		{"ownerOf", []string{`"neo.com"`}},
		{"balanceOf", []string{u160}},
		{"properties", []string{`"neo.com"`}},
		{"tokens", nil},
		{"tokensOf", []string{u160}},
		{"transfer", []string{u160, `"neo.com"`}},

		// name service
		{"addRoot", []string{`"com"`}},
		{"deleteRecord", []string{`"neo.com"`, "nameservice.TypeA"}},
		{"isAvailable", []string{`"neo.com"`}},
		{"getPrice", nil},
		{"getRecord", []string{`"neo.com"`, "nameservice.TypeA"}},
		{"register", []string{`"neo.com"`, u160}},
		{"renew", []string{`"neo.com"`}},
		{"resolve", []string{`"neo.com"`, "nameservice.TypeA"}},
		{"setPrice", []string{"42"}},
		{"setAdmin", []string{`"neo.com"`, u160}},
		{"setRecord", []string{`"neo.com"`, "nameservice.TypeA", `"1.1.1.1"`}},
	})
	runNativeTestCases(t, cs.Ledger.ContractMD, "ledger", []nativeTestCase{
		{"currentHash", nil},
		{"currentIndex", nil},
		{"getBlock", []string{"1"}},
		{"getTransaction", []string{u256}},
		{"getTransactionFromBlock", []string{u256, "1"}},
		{"getTransactionHeight", []string{u256}},
	})
	runNativeTestCases(t, cs.Notary.ContractMD, "notary", []nativeTestCase{
		{"lockDepositUntil", []string{u160, "123"}},
		{"withdraw", []string{u160, u160}},
		{"balanceOf", []string{u160}},
		{"expirationOf", []string{u160}},
		{"getMaxNotValidBeforeDelta", nil},
		{"setMaxNotValidBeforeDelta", []string{"42"}},
	})
	runNativeTestCases(t, cs.Management.ContractMD, "management", []nativeTestCase{
		{"deploy", []string{"nil", "nil"}},
		{"deployWithData", []string{"nil", "nil", "123"}},
		{"destroy", nil},
		{"getContract", []string{u160}},
		{"getMinimumDeploymentFee", nil},
		{"setMinimumDeploymentFee", []string{"42"}},
		{"update", []string{"nil", "nil"}},
		{"updateWithData", []string{"nil", "nil", "123"}},
	})
	runNativeTestCases(t, cs.Crypto.ContractMD, "crypto", []nativeTestCase{
		{"sha256", []string{"[]byte{1, 2, 3}"}},
		{"ripemd160", []string{"[]byte{1, 2, 3}"}},
		{"verifyWithECDsa", []string{"[]byte{1, 2, 3}", pub, sig, "crypto.Secp256k1"}},
	})
	runNativeTestCases(t, cs.Std.ContractMD, "std", []nativeTestCase{
		{"serialize", []string{"[]byte{1, 2, 3}"}},
		{"deserialize", []string{"[]byte{1, 2, 3}"}},
		{"jsonSerialize", []string{"[]byte{1, 2, 3}"}},
		{"jsonDeserialize", []string{"[]byte{1, 2, 3}"}},
		{"base64Encode", []string{"[]byte{1, 2, 3}"}},
		{"base64Decode", []string{"[]byte{1, 2, 3}"}},
		{"base58Encode", []string{"[]byte{1, 2, 3}"}},
		{"base58Decode", []string{"[]byte{1, 2, 3}"}},
		{"itoa", []string{"4", "10"}},
		{"atoi", []string{`"4"`, "10"}},
	})
}

func runNativeTestCases(t *testing.T, ctr interop.ContractMD, name string, testCases []nativeTestCase) {
	t.Run(ctr.Name, func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.method, func(t *testing.T) {
				runNativeTestCase(t, ctr, name, tc.method, tc.params...)
			})
		}
	})
}

func runNativeTestCase(t *testing.T, ctr interop.ContractMD, name, method string, params ...string) {
	md, ok := ctr.GetMethod(strings.TrimSuffix(method, "WithData"), len(params))
	require.True(t, ok)

	isVoid := md.MD.ReturnType == smartcontract.VoidType
	srcTmpl := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/native/%s"
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		var _ interop.Hash160
	`
	if isVoid {
		srcTmpl += `func Main() { %s.%s(%s) }`
	} else {
		srcTmpl += `func Main() interface{} { return %s.%s(%s) }`
	}
	methodUpper := strings.ToUpper(method[:1]) + method[1:] // ASCII only
	methodUpper = strings.ReplaceAll(methodUpper, "Gas", "GAS")
	methodUpper = strings.ReplaceAll(methodUpper, "Json", "JSON")
	src := fmt.Sprintf(srcTmpl, name, name, methodUpper, strings.Join(params, ","))

	v, s := vmAndCompileInterop(t, src)
	id := interopnames.ToID([]byte(interopnames.SystemContractCall))
	result := getTestStackItem(md.MD.ReturnType)
	s.interops[id] = testContractCall(t, ctr.Hash, md, result)
	require.NoError(t, v.Run())
	if isVoid {
		require.Equal(t, 0, v.Estack().Len())
		return
	}
	require.Equal(t, 1, v.Estack().Len(), "stack contains unexpected items")
	require.Equal(t, result.Value(), v.Estack().Pop().Item().Value())
}

func getTestStackItem(typ smartcontract.ParamType) stackitem.Item {
	switch typ {
	case smartcontract.AnyType, smartcontract.VoidType:
		return stackitem.Null{}
	case smartcontract.BoolType:
		return stackitem.NewBool(true)
	case smartcontract.IntegerType:
		return stackitem.NewBigInteger(big.NewInt(42))
	case smartcontract.ByteArrayType, smartcontract.StringType, smartcontract.Hash160Type,
		smartcontract.Hash256Type, smartcontract.PublicKeyType, smartcontract.SignatureType:
		return stackitem.NewByteArray([]byte("result"))
	case smartcontract.ArrayType:
		return stackitem.NewArray([]stackitem.Item{stackitem.NewBool(true), stackitem.Null{}})
	case smartcontract.MapType:
		return stackitem.NewMapWithValue([]stackitem.MapElement{{
			Key:   stackitem.NewByteArray([]byte{1, 2, 3}),
			Value: stackitem.NewByteArray([]byte{5, 6, 7}),
		}})
	case smartcontract.InteropInterfaceType:
		return stackitem.NewInterop(42)
	default:
		panic("unexpected type")
	}
}

func testContractCall(t *testing.T, hash util.Uint160, md interop.MethodAndPrice, result stackitem.Item) func(*vm.VM) error {
	return func(v *vm.VM) error {
		h := v.Estack().Pop().Bytes()
		require.Equal(t, hash.BytesBE(), h)

		method := v.Estack().Pop().String()
		require.Equal(t, md.MD.Name, method)

		fs := callflag.CallFlag(int32(v.Estack().Pop().BigInt().Int64()))
		require.Equal(t, fs, md.RequiredFlags)

		args := v.Estack().Pop().Array()
		require.Equal(t, len(md.MD.Parameters), len(args))

		v.Estack().PushVal(result)
		return nil
	}
}
