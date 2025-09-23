package compiler_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/gas"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/notary"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/policy"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/roles"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

func TestContractHashes(t *testing.T) {
	cs := native.NewContracts(native.NewDefaultContracts(config.ProtocolConfiguration{}))
	require.Equalf(t, []byte(neo.Hash), cs.NEO().Metadata().Hash.BytesBE(), "%q", string(cs.NEO().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(gas.Hash), cs.GAS().Metadata().Hash.BytesBE(), "%q", string(cs.GAS().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(oracle.Hash), cs.Oracle().Metadata().Hash.BytesBE(), "%q", string(cs.Oracle().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(roles.Hash), cs.Designate().Metadata().Hash.BytesBE(), "%q", string(cs.Designate().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(policy.Hash), cs.Policy().Metadata().Hash.BytesBE(), "%q", string(cs.Policy().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(ledger.Hash), cs.ByName(nativenames.Ledger).Metadata().Hash.BytesBE(), "%q", string(cs.ByName(nativenames.Ledger).Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(management.Hash), cs.Management().Metadata().Hash.BytesBE(), "%q", string(cs.Management().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(notary.Hash), cs.Notary().Metadata().Hash.BytesBE(), "%q", string(cs.Notary().Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(crypto.Hash), cs.ByName(nativenames.CryptoLib).Metadata().Hash.BytesBE(), "%q", string(cs.ByName(nativenames.CryptoLib).Metadata().Hash.BytesBE()))
	require.Equalf(t, []byte(std.Hash), cs.ByName(nativenames.StdLib).Metadata().Hash.BytesBE(), "%q", string(cs.ByName(nativenames.StdLib).Metadata().Hash.BytesBE()))
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
	require.EqualValues(t, noderoles.Oracle, roles.Oracle)
	require.EqualValues(t, noderoles.StateValidator, roles.StateValidator)
	require.EqualValues(t, noderoles.NeoFSAlphabet, roles.NeoFSAlphabet)
	require.EqualValues(t, noderoles.P2PNotary, roles.P2PNotary)
}

func TestCryptoLibNamedCurve(t *testing.T) {
	require.EqualValues(t, native.Secp256k1Sha256, crypto.Secp256k1Sha256)
	require.EqualValues(t, native.Secp256r1Sha256, crypto.Secp256r1Sha256)
	require.EqualValues(t, native.Secp256k1Keccak256, crypto.Secp256k1Keccak256)
	require.EqualValues(t, native.Secp256r1Keccak256, crypto.Secp256r1Keccak256)
}

func TestOracleContractValues(t *testing.T) {
	require.EqualValues(t, oracle.Success, transaction.Success)
	require.EqualValues(t, oracle.ProtocolNotSupported, transaction.ProtocolNotSupported)
	require.EqualValues(t, oracle.ConsensusUnreachable, transaction.ConsensusUnreachable)
	require.EqualValues(t, oracle.NotFound, transaction.NotFound)
	require.EqualValues(t, oracle.Timeout, transaction.Timeout)
	require.EqualValues(t, oracle.Forbidden, transaction.Forbidden)
	require.EqualValues(t, oracle.ResponseTooLarge, transaction.ResponseTooLarge)
	require.EqualValues(t, oracle.InsufficientFunds, transaction.InsufficientFunds)
	require.EqualValues(t, oracle.Error, transaction.Error)

	require.EqualValues(t, oracle.MinimumResponseGas, native.MinimumResponseGas)
}

func TestLedgerTransactionWitnessScope(t *testing.T) {
	require.EqualValues(t, ledger.None, transaction.None)
	require.EqualValues(t, ledger.CalledByEntry, transaction.CalledByEntry)
	require.EqualValues(t, ledger.CustomContracts, transaction.CustomContracts)
	require.EqualValues(t, ledger.CustomGroups, transaction.CustomGroups)
	require.EqualValues(t, ledger.Rules, transaction.Rules)
	require.EqualValues(t, ledger.Global, transaction.Global)
}

func TestLedgerTransactionWitnessAction(t *testing.T) {
	require.EqualValues(t, ledger.WitnessAllow, transaction.WitnessAllow)
	require.EqualValues(t, ledger.WitnessDeny, transaction.WitnessDeny)
}

func TestLedgerTransactionWitnessCondition(t *testing.T) {
	require.EqualValues(t, ledger.WitnessBoolean, transaction.WitnessBoolean)
	require.EqualValues(t, ledger.WitnessNot, transaction.WitnessNot)
	require.EqualValues(t, ledger.WitnessAnd, transaction.WitnessAnd)
	require.EqualValues(t, ledger.WitnessOr, transaction.WitnessOr)
	require.EqualValues(t, ledger.WitnessScriptHash, transaction.WitnessScriptHash)
	require.EqualValues(t, ledger.WitnessGroup, transaction.WitnessGroup)
	require.EqualValues(t, ledger.WitnessCalledByEntry, transaction.WitnessCalledByEntry)
	require.EqualValues(t, ledger.WitnessCalledByContract, transaction.WitnessCalledByContract)
	require.EqualValues(t, ledger.WitnessCalledByGroup, transaction.WitnessCalledByGroup)
}

func TestLedgerVMStates(t *testing.T) {
	require.EqualValues(t, ledger.NoneState, vmstate.None)
	require.EqualValues(t, ledger.HaltState, vmstate.Halt)
	require.EqualValues(t, ledger.FaultState, vmstate.Fault)
	require.EqualValues(t, ledger.BreakState, vmstate.Break)
}

func TestPolicyAttributeType(t *testing.T) {
	require.EqualValues(t, policy.HighPriorityT, transaction.HighPriority)
	require.EqualValues(t, policy.OracleResponseT, transaction.OracleResponseT)
	require.EqualValues(t, policy.NotValidBeforeT, transaction.NotValidBeforeT)
	require.EqualValues(t, policy.ConflictsT, transaction.ConflictsT)
	require.EqualValues(t, policy.NotaryAssistedT, transaction.NotaryAssistedT)
}

func TestStorageLimits(t *testing.T) {
	require.EqualValues(t, storage.MaxKeyLen, limits.MaxStorageKeyLen)
	require.EqualValues(t, storage.MaxValueLen, limits.MaxStorageValueLen)
}

type nativeTestCase struct {
	method string
	params []string
}

// Here we test that corresponding method does exist, is invoked and correct value is returned.
func TestNativeHelpersCompile(t *testing.T) {
	cs := native.NewContracts(native.NewDefaultContracts(config.ProtocolConfiguration{}))
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
	runNativeTestCases(t, *cs.NEO().Metadata(), "neo", append([]nativeTestCase{
		{"getCandidates", nil},
		{"getAllCandidates", nil},
		{"getCandidateVote", []string{pub}},
		{"getCommittee", nil},
		{"getCommitteeAddress", nil},
		{"getGasPerBlock", nil},
		{"getNextBlockValidators", nil},
		{"getRegisterPrice", nil},
		{"registerCandidate", []string{pub}},
		{"setGasPerBlock", []string{"1"}},
		{"setRegisterPrice", []string{"10"}},
		{"vote", []string{u160, pub}},
		{"unclaimedGas", []string{u160, "123"}},
		{"unregisterCandidate", []string{pub}},
		{"getAccountState", []string{u160}},
	}, nep17TestCases...))
	runNativeTestCases(t, *cs.GAS().Metadata(), "gas", nep17TestCases)
	runNativeTestCases(t, *cs.ByName(nativenames.Oracle).Metadata(), "oracle", []nativeTestCase{
		{"getPrice", nil},
		{"request", []string{`"url"`, "nil", `"callback"`, "nil", "123"}},
		{"setPrice", []string{"10"}},
	})
	runNativeTestCases(t, *cs.Designate().Metadata(), "roles", []nativeTestCase{
		{"designateAsRole", []string{"1", "[]interop.PublicKey{}"}},
		{"getDesignatedByRole", []string{"1", "1000"}},
	})
	runNativeTestCases(t, *cs.Policy().Metadata(), "policy", []nativeTestCase{
		{"blockAccount", []string{u160}},
		{"getExecFeeFactor", nil},
		{"getFeePerByte", nil},
		{"getStoragePrice", nil},
		{"isBlocked", []string{u160}},
		{"setExecFeeFactor", []string{"42"}},
		{"setFeePerByte", []string{"42"}},
		{"setStoragePrice", []string{"42"}},
		{"unblockAccount", []string{u160}},
		{"getAttributeFee", []string{"1"}},
		{"setAttributeFee", []string{"1", "123"}},
		{"getMaxValidUntilBlockIncrement", nil},
		{"setMaxValidUntilBlockIncrement", []string{"10"}},
		{"getMillisecondsPerBlock", nil},
		{"setMillisecondsPerBlock", []string{"10"}},
		{"getBlockedAccounts", nil},
	})
	runNativeTestCases(t, *cs.ByName(nativenames.Ledger).Metadata(), "ledger", []nativeTestCase{
		{"currentHash", nil},
		{"currentIndex", nil},
		{"getBlock", []string{"1"}},
		{"getTransaction", []string{u256}},
		{"getTransactionFromBlock", []string{u256, "1"}},
		{"getTransactionHeight", []string{u256}},
		{"getTransactionSigners", []string{u256}},
		{"getTransactionVMState", []string{u256}},
	})
	runNativeTestCases(t, *cs.Notary().Metadata(), "notary", []nativeTestCase{
		{"lockDepositUntil", []string{u160, "123"}},
		{"withdraw", []string{u160, u160}},
		{"balanceOf", []string{u160}},
		{"expirationOf", []string{u160}},
		{"getMaxNotValidBeforeDelta", nil},
		{"setMaxNotValidBeforeDelta", []string{"42"}},
	})
	runNativeTestCases(t, *cs.Management().Metadata(), "management", []nativeTestCase{
		{"deploy", []string{"nil", "nil"}},
		{"deployWithData", []string{"nil", "nil", "123"}},
		{"destroy", nil},
		{"getContract", []string{u160}},
		{"getContractById", []string{"1"}},
		{"getContractHashes", nil},
		{"getMinimumDeploymentFee", nil},
		{"hasMethod", []string{u160, `"method"`, "0"}},
		{"setMinimumDeploymentFee", []string{"42"}},
		{"update", []string{"nil", "nil"}},
		{"updateWithData", []string{"nil", "nil", "123"}},
		{"isContract", []string{u160}},
	})
	runNativeTestCases(t, *cs.ByName(nativenames.CryptoLib).Metadata(), "crypto", []nativeTestCase{
		{"sha256", []string{"[]byte{1, 2, 3}"}},
		{"ripemd160", []string{"[]byte{1, 2, 3}"}},
		{"murmur32", []string{"[]byte{1, 2, 3}", "123"}},
		{"verifyWithECDsa", []string{"[]byte{1, 2, 3}", pub, sig, "crypto.Secp256k1Sha256"}},
		{"verifyWithEd25519", []string{"[]byte{1, 2, 3}", "[]byte{1, 2, 3}", "[]byte{1, 2, 3}"}},
		{"recoverSecp256K1", []string{"[]byte{1, 2, 3}", "[]byte{1, 2, 3}"}},
		{"bls12381Serialize", []string{"crypto.Bls12381Point{}"}},
		{"bls12381Deserialize", []string{"[]byte{1, 2, 3}"}},
		{"bls12381Equal", []string{"crypto.Bls12381Point{}", "crypto.Bls12381Point{}"}},
		{"bls12381Add", []string{"crypto.Bls12381Point{}", "crypto.Bls12381Point{}"}},
		{"bls12381Mul", []string{"crypto.Bls12381Point{}", "[]byte{1, 2, 3}", "true"}},
		{"bls12381Pairing", []string{"crypto.Bls12381Point{}", "crypto.Bls12381Point{}"}},
		{"keccak256", []string{"[]byte{1, 2, 3}"}},
	})
	runNativeTestCases(t, *cs.ByName(nativenames.StdLib).Metadata(), "std", []nativeTestCase{
		{"serialize", []string{"[]byte{1, 2, 3}"}},
		{"deserialize", []string{"[]byte{1, 2, 3}"}},
		{"jsonSerialize", []string{"[]byte{1, 2, 3}"}},
		{"jsonDeserialize", []string{"[]byte{1, 2, 3}"}},
		{"base64Encode", []string{"[]byte{1, 2, 3}"}},
		{"base64Decode", []string{"[]byte{1, 2, 3}"}},
		{"base58Encode", []string{"[]byte{1, 2, 3}"}},
		{"base58Decode", []string{"[]byte{1, 2, 3}"}},
		{"base58CheckEncode", []string{"[]byte{1, 2, 3}"}},
		{"base58CheckDecode", []string{"[]byte{1, 2, 3}"}},
		{"itoa", []string{"4", "10"}},
		{"itoa10", []string{"4"}},
		{"atoi", []string{`"4"`, "10"}},
		{"atoi10", []string{`"4"`}},
		{"memoryCompare", []string{"[]byte{1}", "[]byte{2}"}},
		{"memorySearch", []string{"[]byte{1}", "[]byte{2}"}},
		{"memorySearchIndex", []string{"[]byte{1}", "[]byte{2}", "3"}},
		{"memorySearchLastIndex", []string{"[]byte{1}", "[]byte{2}", "3"}},
		{"stringSplit", []string{`"a,b"`, `","`}},
		{"stringSplitNonEmpty", []string{`"a,b"`, `","`}},
		{"hexEncode", []string{"[]byte{0, 1, 2, 3}"}},
		{"hexDecode", []string{`"00010203"`}},
	})
}

func runNativeTestCases(t *testing.T, ctr interop.ContractMD, name string, nativeTestCases []nativeTestCase) {
	srcBuilder := bytes.NewBuffer([]byte(`package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/native/` + name + `"
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		var _ interop.Hash160
	`))
	for i, tc := range nativeTestCases {
		addNativeTestCase(t, srcBuilder, ctr, i, name, tc.method, tc.params...)
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	t.Run(ctr.Name, func(t *testing.T) {
		for i, tc := range nativeTestCases {
			t.Run(tc.method, func(t *testing.T) {
				runNativeTestCase(t, ne, di, ctr, i, tc.method, tc.params...)
			})
		}
	})
}

func getMethod(t *testing.T, ctr interop.ContractMD, name string, params []string) interop.HFSpecificMethodAndPrice {
	paramLen := len(params)

	switch {
	case name == "itoa10" || name == "atoi10":
		name = name[:4]
	case strings.HasPrefix(name, "memorySearch"):
		if strings.HasSuffix(name, "LastIndex") {
			paramLen++ // true should be appended inside of an interop
		}
		name = "memorySearch"
	case strings.HasPrefix(name, "stringSplit"):
		if strings.HasSuffix(name, "NonEmpty") {
			paramLen++ // true should be appended inside of an interop
		}
		name = "stringSplit"
	default:
		name = strings.TrimSuffix(name, "WithData")
	}

	latestHF := config.HFLatestKnown
	cMD := ctr.HFSpecificContractMD(&latestHF)
	md, ok := cMD.GetMethod(name, paramLen)
	require.True(t, ok, cMD.Manifest.Name, name, paramLen)
	return md
}

func addNativeTestCase(t *testing.T, srcBuilder *bytes.Buffer, ctr interop.ContractMD, i int, name, method string, params ...string) {
	md := getMethod(t, ctr, method, params)
	isVoid := md.MD.ReturnType == smartcontract.VoidType
	srcBuilder.WriteString("func F" + strconv.Itoa(i) + "() ")
	if !isVoid {
		srcBuilder.WriteString("any { return ")
	} else {
		srcBuilder.WriteString("{ ")
	}
	methodUpper := strings.ToUpper(method[:1]) + method[1:] // ASCII only
	methodUpper = strings.ReplaceAll(methodUpper, "Gas", "GAS")
	methodUpper = strings.ReplaceAll(methodUpper, "Json", "JSON")
	methodUpper = strings.ReplaceAll(methodUpper, "Id", "ID")
	srcBuilder.WriteString(name)
	srcBuilder.WriteRune('.')
	srcBuilder.WriteString(methodUpper)
	srcBuilder.WriteRune('(')
	srcBuilder.WriteString(strings.Join(params, ", "))
	srcBuilder.WriteString(") }\n")
}

func runNativeTestCase(t *testing.T, b *nef.File, di *compiler.DebugInfo, ctr interop.ContractMD, i int, method string, params ...string) {
	md := getMethod(t, ctr, method, params)
	result := getTestStackItem(md.MD.ReturnType)
	isVoid := md.MD.ReturnType == smartcontract.VoidType

	v := vm.New()
	v.LoadToken = func(id int32) error {
		t := b.Tokens[id]
		if t.Hash != ctr.Hash {
			return fmt.Errorf("wrong hash %s", t.Hash.StringLE())
		}
		if t.Method != md.MD.Name {
			return fmt.Errorf("wrong name %s", t.Method)
		}
		if int(t.ParamCount) != len(md.MD.Parameters) {
			return fmt.Errorf("wrong number of parameters %v", t.ParamCount)
		}
		if t.HasReturn != !isVoid {
			return fmt.Errorf("wrong hasReturn %v", t.HasReturn)
		}
		if t.CallFlag != md.RequiredFlags {
			return fmt.Errorf("wrong flags %v", t.CallFlag)
		}
		for range t.ParamCount {
			_ = v.Estack().Pop()
		}
		if v.Estack().Len() != 0 {
			return errors.New("excessive parameters on the stack")
		}
		if !isVoid {
			v.Estack().PushVal(result)
		}
		return nil
	}
	invokeMethod(t, fmt.Sprintf("F%d", i), b.Script, v, di)
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
