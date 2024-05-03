package native_test

import (
	"math/big"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

// TestCryptoLib_KoblitzVerificationScript builds transaction with custom witness that contains
// the Koblitz tx signature bytes and Koblitz signature verification script.
// This test ensures that transaction signed by Koblitz key passes verification and can
// be successfully accepted to the chain.
func TestCryptoLib_KoblitzVerificationScript(t *testing.T) {
	check := func(
		t *testing.T,
		buildVerificationScript func(t *testing.T, pub *keys.PublicKey) []byte,
		constructMsg func(t *testing.T, magic uint32, tx hash.Hashable) []byte,
	) {
		c := newGasClient(t)
		gasInvoker := c.WithSigners(c.Committee)
		e := c.Executor

		// Consider the user that is able to sign txs only with Secp256k1 private key.
		// Let this user build, sign and push a GAS transfer transaction from its account
		// to some other account.
		pk, err := keys.NewSecp256k1PrivateKey()
		require.NoError(t, err)

		// Firstly, we need to build the N3 user's account address based on the user's public key.
		// The address itself is Hash160 from the verification script corresponding to the user's public key.
		// Since user's private key belongs to Koblitz curve, we can't use System.Crypto.CheckSig interop
		// in the verification script. Likely, we have a 'verifyWithECDsa' method in native CriptoLib contract
		// that is able to check Koblitz signature. So let's build custom verification script based on this call.
		// The script should call 'verifyWithECDsa' method of native CriptoLib contract with Koblitz curve identifier
		// and check the provided message signature against the user's Koblitz public key.
		vrfBytes := buildVerificationScript(t, pk.PublicKey())

		// Construct the user's account script hash. It's effectively a verification script hash.
		from := hash.Hash160(vrfBytes)

		// Supply this account with some initial balance so that the user is able to pay for his transactions.
		gasInvoker.Invoke(t, true, "transfer", c.Committee.ScriptHash(), from, 10000_0000_0000, nil)

		// Construct transaction that transfers 5 GAS from the user's account to some other account.
		to := util.Uint160{1, 2, 3}
		amount := 5
		tx := gasInvoker.PrepareInvokeNoSign(t, "transfer", from, to, amount, nil)
		tx.Signers = []transaction.Signer{
			{
				Account: from,
				Scopes:  transaction.CalledByEntry,
			},
		}
		neotest.AddNetworkFee(t, e.Chain, tx)
		neotest.AddSystemFee(e.Chain, tx, -1)

		// Add some more network fee to pay for the witness verification. This value may be calculated precisely,
		// but let's keep some inaccurate value for the test.
		tx.NetworkFee += 540_0000

		// This transaction (along with the network magic) should be signed by the user's Koblitz private key.
		msg := constructMsg(t, uint32(e.Chain.GetConfig().Magic), tx)

		// The user has to sign the hash of the message by his Koblitz key.
		// Please, note that this Keccak256 hash may easily be replaced by sha256 hash if needed.
		signature := pk.SignHash(native.Keccak256(msg))

		// Ensure that signature verification passes. This line here is just for testing purposes,
		// it won't be present in the real code.
		require.True(t, pk.PublicKey().Verify(signature, native.Keccak256(msg).BytesBE()))

		// Build invocation witness script for the user's account.
		invBytes := buildKoblitzInvocationScript(t, [][]byte{signature})

		// Construct witness for signer #0 (the user itself).
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript:   invBytes,
				VerificationScript: vrfBytes,
			},
		}

		// Add transaction to the chain. No error is expected on new block addition. Note, that this line performs
		// all those checks that are executed during transaction acceptance in the real network.
		e.AddNewBlock(t, tx)

		// Double-check: ensure funds have been transferred.
		e.CheckGASBalance(t, to, big.NewInt(int64(amount)))
	}

	// The proposed preferable witness verification script
	// (110 bytes, 2154270 GAS including Invocation script execution).
	// The user has to sign the keccak256([4-bytes-network-magic-LE, txHash-bytes-BE]).
	check(t, buildKoblitzVerificationScript, constructMessage)

	// Below presented some variations of verification scripts that were also considered, but
	// they are not as good as the first one.

	// The simplest witness verification script with low length and low execution cost
	// (98 bytes, 2092530 GAS including Invocation script execution).
	// The user has to sign the keccak256([var-bytes-network-magic, txHash-bytes-BE]).
	check(t, buildKoblitzVerificationScriptSimpleSingleHash, constructMessageNoHash)

	// Even more simple witness verification script with low length and low execution cost
	// (95 bytes, 2092320 GAS including Invocation script execution).
	// The user has to sign the keccak256([var-bytes-network-magic, txHash-bytes-BE]).
	// The difference is that network magic is a static value, thus, both verification script and
	// user address are network-specific.
	check(t, buildKoblitzVerificationScriptSimpleSingleHashStaticMagic, constructMessageNoHash)

	// More complicated verification script with higher length and higher execution cost
	// (136 bytes, 4120620 GAS including Invocation script execution).
	// The user has to sign the keccak256(sha256([var-bytes-network-magic, txHash-bytes-BE])).
	check(t, buildKoblitzVerificationScriptSimple, constructMessageSimple)

	// Witness verification script that follows the existing standard CheckSig account generation rules
	// and has larger length and higher execution cost.
	// (186 bytes, 5116020 GAS including Invocation script execution).
	// The user has to sign the keccak256(sha256([4-bytes-network-magic-LE, txHash-bytes-BE]))
	check(t, buildKoblitzVerificationScriptCompat, constructMessageCompat)
}

// buildKoblitzVerificationScript builds witness verification script for Koblitz public key.
// This method checks
//
//	keccak256([4-bytes-network-magic-LE, txHash-bytes-BE])
//
// instead of (comparing with N3)
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE]).
func buildKoblitzVerificationScript(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)                  // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())                    // emit the caller's public key.
	// Construct and push the signed message. The signed message is effectively the network-dependent transaction hash,
	// i.e. msg = [4-network-magic-bytes-LE, tx-hash-BE]
	// Firstly, retrieve network magic (it's uint32 wrapped into BigInteger and represented as Integer stackitem on stack).
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetNetwork) // push network magic (Integer stackitem), can have 0-5 bytes length serialized.
	// Convert network magic to 4-bytes-length LE byte array representation.
	emit.Int(vrf.BinWriter, 0x100000000)
	emit.Opcodes(vrf.BinWriter, opcode.ADD, // some new number that is 5 bytes at least when serialized, but first 4 bytes are intact network value (LE).
		opcode.PUSH4, opcode.LEFT) // cut the first 4 bytes out of a number that is at least 5 bytes long, the result is 4-bytes-length LE network representation.
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM)                // pick 0-th transaction item (the transaction hash).
	// Concatenate network magic and transaction hash.
	emit.Opcodes(vrf.BinWriter, opcode.CAT) // this instruction will convert network magic to bytes using BigInteger rules of conversion.
	// Continue construction of 'verifyWithECDsa' call.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.PACK)                         // pack arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.
	require.NoError(t, vrf.Err)

	return vrf.Bytes()
	// Here's an example of the resulting witness verification script (110 bytes length, always constant length, with constant length of signed data):
	// NEO-GO-VM > loadbase64 ABhQDCECoIi/qx5LS+3n1GJFcoYbQByyDDsU6QaHvYhiJypOYWZBxfug4AMAAAAAAQAAAJ4UjUEtUQgwEM6LFMAfDA92ZXJpZnlXaXRoRUNEc2EMFBv1dasRiWiEE2EKNaEohs3gtmxyQWJ9W1I=
	// READY: loaded 110 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHINT8     24 (18)    <<
	// 2        SWAP
	// 3        PUSHDATA1    02a088bfab1e4b4bede7d4624572861b401cb20c3b14e90687bd8862272a4e6166
	// 38       SYSCALL      System.Runtime.GetNetwork (c5fba0e0)
	// 43       PUSHINT64    4294967296 (0000000001000000)
	// 52       ADD
	// 53       PUSH4
	// 54       LEFT
	// 55       SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 60       PUSH0
	// 61       PICKITEM
	// 62       CAT
	// 63       PUSH4
	// 64       PACK
	// 65       PUSH15
	// 66       PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 83       PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 105      SYSCALL      System.Contract.Call (627d5b52)
}

// buildKoblitzVerificationScriptSimpleSingleHash builds witness verification script for Koblitz public key.
// This method differs from buildKoblitzVerificationScriptCompat in that it checks
//
//	keccak256([var-bytes-network-magic, txHash-bytes-BE])
//
// instead of (comparing with N3)
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE]).
func buildKoblitzVerificationScriptSimpleSingleHash(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	// vrf is witness verification script corresponding to the pk.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)                  // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())                    // emit the caller's public key.
	// Construct and push the signed message. The signed message is effectively the network-dependent transaction hash,
	// i.e. msg = [network-magic-bytes, tx.Hash()]
	// Firstly, retrieve network magic (it's uint32 wrapped into BigInteger and represented as Integer stackitem on stack).
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetNetwork) // push network magic.
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM)                // pick 0-th transaction item (the transaction hash).
	// Concatenate network magic and transaction hash.
	emit.Opcodes(vrf.BinWriter, opcode.CAT) // this instruction will convert network magic to bytes using BigInteger rules of conversion.
	// Continue construction of 'verifyWithECDsa' call.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.PACK)                         // pack arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.
	require.NoError(t, vrf.Err)

	return vrf.Bytes()
	// Here's an example of the resulting witness verification script (98 bytes length, always constant length, with variable length of signed data):
	// NEO-GO-VM > loadbase64 ABZQDCEDY9ekgSWnbN6m4JjJ8SjoKSDtQo5ftMrx1/gcFsrQwgVBxfug4EEtUQgwEM6LFMAfDA92ZXJpZnlXaXRoRUNEc2EMFBv1dasRiWiEE2EKNaEohs3gtmxyQWJ9W1I=
	// READY: loaded 98 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHINT8     24 (18)    <<
	// 2        SWAP
	// 3        PUSHDATA1    0363d7a48125a76cdea6e098c9f128e82920ed428e5fb4caf1d7f81c16cad0c205
	// 38       SYSCALL      System.Runtime.GetNetwork (c5fba0e0)
	// 43       SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 48       PUSH0
	// 49       PICKITEM
	// 50       CAT
	// 51       PUSH4
	// 52       PACK
	// 53       PUSH15
	// 54       PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 71       PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 93       SYSCALL      System.Contract.Call (627d5b52)
}

// buildKoblitzVerificationScriptSimpleSingleHashStaticMagic builds witness verification script for Koblitz public key.
// This method differs from buildKoblitzVerificationScriptCompat in that it checks
//
//	keccak256([var-bytes-network-magic, txHash-bytes-BE])
//
// instead of (comparing with N3)
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE]).
//
// and it uses static magic value (simple PUSHINT* + magic, or PUSHDATA1 + magicBytes is also possible)
// which results in network-specific verification script and, consequently, network-specific user address.
func buildKoblitzVerificationScriptSimpleSingleHashStaticMagic(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	// vrf is witness verification script corresponding to the pk.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)                  // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())                    // emit the caller's public key.
	// Construct and push the signed message. The signed message is effectively the network-dependent transaction hash,
	// i.e. msg = [network-magic-bytes, tx.Hash()]
	// Firstly, push static network magic (it's 42 for unit test chain).
	emit.Int(vrf.BinWriter, 42)
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM)                // pick 0-th transaction item (the transaction hash).
	// Concatenate network magic and transaction hash.
	emit.Opcodes(vrf.BinWriter, opcode.CAT) // this instruction will convert network magic to bytes using BigInteger rules of conversion.
	// Continue construction of 'verifyWithECDsa' call.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.PACK)                         // pack arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.
	require.NoError(t, vrf.Err)

	return vrf.Bytes()
	// Here's an example of the resulting witness verification script (95 bytes length, always constant length, with variable length of signed data):
	// NEO-GO-VM > loadbase64 ABZQDCECluEwgK3pKiq3IjOMKiSe6Ng6FPZJxoMhZkFl8GvREL0AKkEtUQgwEM6LFMAfDA92ZXJpZnlXaXRoRUNEc2EMFBv1dasRiWiEE2EKNaEohs3gtmxyQWJ9W1I=
	// READY: loaded 95 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHINT8     24 (18)    <<
	// 2        SWAP
	// 3        PUSHDATA1    0296e13080ade92a2ab722338c2a249ee8d83a14f649c68321664165f06bd110bd
	// 38       PUSHINT8     42 (2a)
	// 40       SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 45       PUSH0
	// 46       PICKITEM
	// 47       CAT
	// 48       PUSH4
	// 49       PACK
	// 50       PUSH15
	// 51       PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 68       PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 90       SYSCALL      System.Contract.Call (627d5b52)
}

// buildKoblitzVerificationScriptSimple builds witness verification script for Koblitz public key.
// This method differs from buildKoblitzVerificationScriptCompat in that it checks
//
//	keccak256(sha256([var-bytes-network-magic, txHash-bytes-BE]))
//
// instead of (comparing with N3)
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE]).
//
// It produces constant-length verification script (136 bytes) independently of the network parameters.
// However, the length of signed message is variable and depends on the network magic (since network
// magic Integer stackitem being converted to Buffer has the resulting byte slice length that depends on
// the magic).
func buildKoblitzVerificationScriptSimple(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	// vrf is witness verification script corresponding to the pk.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)                  // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())                    // emit the caller's public key.
	// Construct and push the signed message. The signed message is effectively the network-dependent transaction hash,
	// i.e. msg = Sha256([network-magic-bytes, tx.Hash()])
	// Firstly, retrieve network magic (it's uint32 wrapped into BigInteger and represented as Integer stackitem on stack).
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetNetwork) // push network magic.
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM,                // pick 0-th transaction item (the transaction hash).
		opcode.CAT,   // concatenate network magic and transaction hash; this instruction will convert network magic to bytes using BigInteger rules of conversion.
		opcode.PUSH1, // push 1 (the number of arguments of 'sha256' method of native CryptoLib).
		opcode.PACK)  // pack arguments for 'sha256' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "sha256", callflag.All) // emit the call to 'sha256' itself.
	// Continue construction of 'verifyWithECDsa' call.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.PACK)                         // pack arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.
	require.NoError(t, vrf.Err)

	return vrf.Bytes()
	// Here's an example of the resulting witness verification script (136 bytes length, always constant length, with variable length of signed data):
	// NEO-GO-VM 0 > loadbase64 ABZQDCEDp38Tevu0to16RQqloo/jNfgExYmoCElLS2JuuYcH831Bxfug4EEtUQgwEM6LEcAfDAZzaGEyNTYMFBv1dasRiWiEE2EKNaEohs3gtmxyQWJ9W1IUwB8MD3ZlcmlmeVdpdGhFQ0RzYQwUG/V1qxGJaIQTYQo1oSiGzeC2bHJBYn1bUg==
	// READY: loaded 136 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHINT8     24 (18)    <<
	// 2        SWAP
	// 3        PUSHDATA1    03a77f137afbb4b68d7a450aa5a28fe335f804c589a808494b4b626eb98707f37d
	// 38       SYSCALL      System.Runtime.GetNetwork (c5fba0e0)
	// 43       SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 48       PUSH0
	// 49       PICKITEM
	// 50       CAT
	// 51       PUSH1
	// 52       PACK
	// 53       PUSH15
	// 54       PUSHDATA1    736861323536 ("sha256")
	// 62       PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 84       SYSCALL      System.Contract.Call (627d5b52)
	// 89       PUSH4
	// 90       PACK
	// 91       PUSH15
	// 92       PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 109      PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 131      SYSCALL      System.Contract.Call (627d5b52)
}

// buildKoblitzVerificationScript builds custom verification script for the provided Koblitz public key.
// It checks that the following message is signed by the provided public key:
//
//	keccak256(sha256([4-bytes-network-magic-LE, txHash-bytes-BE]))
//
// It produces constant-length verification script (186 bytes) independently of the network parameters.
func buildKoblitzVerificationScriptCompat(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)                  // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())                    // emit the caller's public key.
	// Construct and push the signed message. The signed message is effectively the network-dependent transaction hash,
	// i.e. msg = Sha256([4-bytes-network-magic-LE, tx.Hash()])
	// Firstly, convert network magic (uint32) to LE buffer.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetNetwork) // push network magic.
	// First byte: n & 0xFF
	emit.Opcodes(vrf.BinWriter, opcode.DUP)
	emit.Int(vrf.BinWriter, 0xFF) // TODO: this can be optimize in order not to allocate 0xFF every time, but need to compare execution price.
	emit.Opcodes(vrf.BinWriter, opcode.AND,
		opcode.SWAP, // Swap with the original network n.
		opcode.PUSH8,
		opcode.SHR)
	// Second byte: n >> 8 & 0xFF
	emit.Opcodes(vrf.BinWriter, opcode.DUP)
	emit.Int(vrf.BinWriter, 0xFF)
	emit.Opcodes(vrf.BinWriter, opcode.AND,
		opcode.SWAP, // Swap with the n >> 8.
		opcode.PUSH8,
		opcode.SHR)
	// Third byte: n >> 16 & 0xFF
	emit.Opcodes(vrf.BinWriter, opcode.DUP)
	emit.Int(vrf.BinWriter, 0xFF)
	emit.Opcodes(vrf.BinWriter, opcode.AND,
		opcode.SWAP, // Swap with the n >> 16.
		opcode.PUSH8,
		opcode.SHR)
	// Fourth byte: n >> 24 & 0xFF
	emit.Int(vrf.BinWriter, 0xFF) // no DUP is needed since it's the last shift.
	emit.Opcodes(vrf.BinWriter, opcode.AND)
	// Put these 4 bytes into buffer.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.NEWBUFFER) // allocate new 4-bytes-length buffer.
	emit.Opcodes(vrf.BinWriter,
		// Set fourth byte.
		opcode.DUP, opcode.PUSH3,
		opcode.PUSH3, opcode.ROLL,
		opcode.SETITEM,
		// Set third byte.
		opcode.DUP, opcode.PUSH2,
		opcode.PUSH3, opcode.ROLL,
		opcode.SETITEM,
		// Set second byte.
		opcode.DUP, opcode.PUSH1,
		opcode.PUSH3, opcode.ROLL,
		opcode.SETITEM,
		// Set first byte.
		opcode.DUP, opcode.PUSH0,
		opcode.PUSH3, opcode.ROLL,
		opcode.SETITEM)
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM,                // pick 0-th transaction item (the transaction hash).
		opcode.CAT,   // concatenate network magic and transaction hash.
		opcode.PUSH1, // push 1 (the number of arguments of 'sha256' method of native CryptoLib).
		opcode.PACK)  // pack arguments for 'sha256' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "sha256", callflag.All) // emit the call to 'sha256' itself.
	// Continue construction of 'verifyWithECDsa' call.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH4, opcode.PACK)                         // pack arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, criptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.
	require.NoError(t, vrf.Err)

	return vrf.Bytes()
	// Here's an example of the resulting witness verification script (186 bytes length, always constant length, the length of signed data is also always constant):
	// NEO-GO-VM 0 > loadbase64 ABZQDCECYn75w2MePMuPvExbbEnjjM7eWnmvseGwcI+7lYp4AtdBxfug4EoB/wCRUBipSgH/AJFQGKlKAf8AkVAYqQH/AJEUiEoTE1LQShITUtBKERNS0EoQE1LQQS1RCDAQzosRwB8MBnNoYTI1NgwUG/V1qxGJaIQTYQo1oSiGzeC2bHJBYn1bUhTAHwwPdmVyaWZ5V2l0aEVDRHNhDBQb9XWrEYlohBNhCjWhKIbN4LZsckFifVtS
	// READY: loaded 186 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHINT8     24 (18)    <<
	// 2        SWAP
	// 3        PUSHDATA1    02627ef9c3631e3ccb8fbc4c5b6c49e38ccede5a79afb1e1b0708fbb958a7802d7
	// 38       SYSCALL      System.Runtime.GetNetwork (c5fba0e0)
	// 43       DUP
	// 44       PUSHINT16    255 (ff00)
	// 47       AND
	// 48       SWAP
	// 49       PUSH8
	// 50       SHR
	// 51       DUP
	// 52       PUSHINT16    255 (ff00)
	// 55       AND
	// 56       SWAP
	// 57       PUSH8
	// 58       SHR
	// 59       DUP
	// 60       PUSHINT16    255 (ff00)
	// 63       AND
	// 64       SWAP
	// 65       PUSH8
	// 66       SHR
	// 67       PUSHINT16    255 (ff00)
	// 70       AND
	// 71       PUSH4
	// 72       NEWBUFFER
	// 73       DUP
	// 74       PUSH3
	// 75       PUSH3
	// 76       ROLL
	// 77       SETITEM
	// 78       DUP
	// 79       PUSH2
	// 80       PUSH3
	// 81       ROLL
	// 82       SETITEM
	// 83       DUP
	// 84       PUSH1
	// 85       PUSH3
	// 86       ROLL
	// 87       SETITEM
	// 88       DUP
	// 89       PUSH0
	// 90       PUSH3
	// 91       ROLL
	// 92       SETITEM
	// 93       SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 98       PUSH0
	// 99       PICKITEM
	// 100      CAT
	// 101      PUSH1
	// 102      PACK
	// 103      PUSH15
	// 104      PUSHDATA1    736861323536 ("sha256")
	// 112      PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 134      SYSCALL      System.Contract.Call (627d5b52)
	// 139      PUSH4
	// 140      PACK
	// 141      PUSH15
	// 142      PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 159      PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 181      SYSCALL      System.Contract.Call (627d5b52)
}

// buildKoblitzInvocationScript builds witness invocation script for the transaction signatures. The signature
// itself may be produced by public key over any curve (not required Koblitz, the algorithm is the same).
// The signatures expected to be sorted by public key (if multiple signatures are provided).
func buildKoblitzInvocationScript(t *testing.T, signatures [][]byte) []byte {
	//Exactly like during standard
	// signature verification, the resulting script pushes Koblitz signature bytes onto stack.
	inv := io.NewBufBinWriter()
	for _, sig := range signatures {
		emit.Bytes(inv.BinWriter, sig) // message signature bytes.
	}
	require.NoError(t, inv.Err)

	return inv.Bytes()
	// Here's an example of the resulting single witness invocation script (66 bytes length, always constant length):
	// NEO-GO-VM > loadbase64 DEBMGKU/MdSizlzaVNDUUbd1zMZQJ43eTaZ4vBCpmkJ/wVh1TYrAWEbFyHhkqq+aYxPCUS43NKJdJTXavcjB8sTP
	// READY: loaded 66 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHDATA1    4c18a53f31d4a2ce5cda54d0d451b775ccc650278dde4da678bc10a99a427fc158754d8ac05846c5c87864aaaf9a6313c2512e3734a25d2535dabdc8c1f2c4cf    <<
	//
	// Here's an example of the 3 out of 4 multisignature invocation script (66 * m bytes length, always constant length):
	// NEO-GO-VM > loadbase64 DEBsPMY3+7sWyZf0gCVcqPzwZ79p+KpeylgtbYIrXp4Tdi6E/8q3DIrEgK7DdVe3YdbfE+VPrpwym/ufBb8MRTB6DED5B9OZDGWdJApRfuy9LeUTa2mLsXP7mBRa181g0Jo7beylWzVgDqHHF2PilECMcLmRbFRknmQm4KgiGkDE+O6ZDEAYt61O2dMfasJHiQD95M5b4mR6NBnDsMTo2e59H3y4YguroVLiUxnQSc4qu9LWvEIKr4/ytjCCuANXOkJmSw8C
	// READY: loaded 198 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHDATA1    6c3cc637fbbb16c997f480255ca8fcf067bf69f8aa5eca582d6d822b5e9e13762e84ffcab70c8ac480aec37557b761d6df13e54fae9c329bfb9f05bf0c45307a    <<
	// 66       PUSHDATA1    f907d3990c659d240a517eecbd2de5136b698bb173fb98145ad7cd60d09a3b6deca55b35600ea1c71763e294408c70b9916c54649e6426e0a8221a40c4f8ee99
	// 132      PUSHDATA1    18b7ad4ed9d31f6ac2478900fde4ce5be2647a3419c3b0c4e8d9ee7d1f7cb8620baba152e25319d049ce2abbd2d6bc420aaf8ff2b63082b803573a42664b0f02
}

// constructMessage constructs message for signing that consists of the
// unhashed constant 4-bytes length LE magic and transaction hash bytes:
//
//	[4-bytes-network-magic-LE, txHash-bytes-BE]
func constructMessage(t *testing.T, magic uint32, tx hash.Hashable) []byte {
	return hash.GetSignedData(magic, tx)
}

// constructMessageNoHash constructs message for signing that consists of the
// unhashed magic and transaction hash bytes:
//
//	[var-bytes-network-magic, txHash-bytes-BE]
func constructMessageNoHash(t *testing.T, magic uint32, tx hash.Hashable) []byte {
	m := big.NewInt(int64(magic))
	return append(m.Bytes(), tx.Hash().BytesBE()...)
}

// constructMessageCompat constructs message for signing that does not follow N3 rules,
// but entails smaller verification script size and smaller verification price:
//
//	sha256([var-bytes-network-magic, txHash-bytes-BE])
func constructMessageSimple(t *testing.T, magic uint32, tx hash.Hashable) []byte {
	m := big.NewInt(int64(magic))
	return hash.Sha256(append(m.Bytes(), tx.Hash().BytesBE()...)).BytesBE()
}

// constructMessageCompat constructs message for signing following the N3 rules:
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE])
func constructMessageCompat(t *testing.T, magic uint32, tx hash.Hashable) []byte {
	return hash.NetSha256(magic, tx).BytesBE()
}

// TestCryptoLib_KoblitzMultisigVerificationScript builds transaction with custom witness that contains
// the Koblitz tx multisignature bytes and Koblitz multisignature verification script.
// This test ensures that transaction signed by m out of n Koblitz keys passes verification and can
// be successfully accepted to the chain.
func TestCryptoLib_KoblitzMultisigVerificationScript(t *testing.T) {
	check := func(
		t *testing.T,
		buildVerificationScript func(t *testing.T, m int, pub keys.PublicKeys) []byte,
		constructMsg func(t *testing.T, magic uint32, tx hash.Hashable) []byte,
	) {
		c := newGasClient(t)
		gasInvoker := c.WithSigners(c.Committee)
		e := c.Executor

		// Consider 4 users willing to sign 3/4 multisignature transaction Secp256k1 private keys.
		const (
			n = 4
			m = 3
		)
		pks := make([]*keys.PrivateKey, n)
		for i := range pks {
			var err error
			pks[i], err = keys.NewSecp256k1PrivateKey()
			require.NoError(t, err)
		}
		// Sort private keys by their public keys.
		sort.Slice(pks, func(i, j int) bool {
			return pks[i].PublicKey().Cmp(pks[j].PublicKey()) < 0
		})

		// Firstly, we need to build the N3 multisig account address based on the users' public keys.
		// Pubs must be sorted, exactly like for the standard CheckMultisig.
		pubs := make(keys.PublicKeys, n)
		for i := range pks {
			pubs[i] = pks[i].PublicKey()
		}
		vrfBytes := buildVerificationScript(t, m, pubs)

		// Construct the user's account script hash. It's effectively a verification script hash.
		from := hash.Hash160(vrfBytes)

		// Supply this account with some initial balance so that the user is able to pay for his transactions.
		gasInvoker.Invoke(t, true, "transfer", c.Committee.ScriptHash(), from, 10000_0000_0000, nil)

		// Construct transaction that transfers 5 GAS from the user's account to some other account.
		to := util.Uint160{1, 2, 3}
		amount := 5
		tx := gasInvoker.PrepareInvokeNoSign(t, "transfer", from, to, amount, nil)
		tx.Signers = []transaction.Signer{
			{
				Account: from,
				Scopes:  transaction.CalledByEntry,
			},
		}
		neotest.AddNetworkFee(t, e.Chain, tx)
		neotest.AddSystemFee(e.Chain, tx, -1)

		// Add some more network fee to pay for the witness verification. This value may be calculated precisely,
		// but let's keep some inaccurate value for the test.
		tx.NetworkFee = 8995470

		// This transaction (along with the network magic) should be signed by the user's Koblitz private key.
		msg := constructMsg(t, uint32(e.Chain.GetConfig().Magic), tx)

		// The users have to sign the hash of the message by their Koblitz key. Collect m signatures from first m keys.
		// Signatures must be sorted by public key.
		sigs := make([][]byte, m)
		for i := range sigs {
			j := i
			if i > 0 {
				j++ // Add some shift to ensure that verification script works correctly.
			}
			if i > 3 {
				j++ // Add more shift for large number of public keys for the same purpose.
			}
			sigs[i] = pks[j].SignHash(native.Keccak256(msg))
		}

		// Build invocation witness script for the signatures.
		invBytes := buildKoblitzInvocationScript(t, sigs)

		// Construct witness for signer #0 (the multisig account itself).
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript:   invBytes,
				VerificationScript: vrfBytes,
			},
		}

		// Add transaction to the chain. No error is expected on new block addition. Note, that this line performs
		// all those checks that are executed during transaction acceptance in the real network.
		e.AddNewBlock(t, tx)

		// Double-check: ensure funds have been transferred.
		e.CheckGASBalance(t, to, big.NewInt(int64(amount)))
	}

	// The proposed multisig verification script.
	// (261 bytes, 8389470 GAS including Invocation script execution for 3/4 multisig).
	// The user has to sign the keccak256([4-bytes-network-magic-LE, txHash-bytes-BE]).
	check(t, buildKoblitzMultisigVerificationScript, constructMessage)
}

// buildKoblitzMultisigVerificationScript builds witness verification script for m signatures out of n Koblitz public keys.
// Public keys must be sorted. Signatures (pushed by witness Invocation script) must be sorted by public keys.
// It checks m out of n multisignature of the following message:
//
//	keccak256([4-bytes-network-magic-LE, txHash-bytes-BE])
func buildKoblitzMultisigVerificationScript(t *testing.T, m int, pubs keys.PublicKeys) []byte {
	if len(pubs) == 0 {
		t.Fatalf("empty pubs list")
	}
	if m > len(pubs) {
		t.Fatalf("m must be not greater than the number of public keys")
	}

	n := len(pubs) // public keys must be sorted.
	cryptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// In fact, the following algorithm is implemented via NeoVM instructions:
	//
	// func Check(sigs []interop.Signature) bool {
	// 	if m != len(sigs) {
	// 		return false
	// 	}
	// 	var pubs []interop.PublicKey = []interop.PublicKey{...}
	// 	msg := append(convert.ToBytes(runtime.GetNetwork()), runtime.GetScriptContainer().Hash...)
	// 	var sigCnt = 0
	// 	var pubCnt = 0
	// 	for ; sigCnt < m && pubCnt < n; { // sigs must be sorted by pub
	// 		sigCnt += crypto.VerifyWithECDsa(msg, pubs[pubCnt], sigs[sigCnt], crypto.Secp256k1Keccak256)
	// 		pubCnt++
	// 	}
	// 	return sigCnt == m
	// }
	vrf := io.NewBufBinWriter()

	// Initialize slots for local variables. Locals slot scheme:
	// LOC0 -> sigs
	// LOC1 -> pubs
	// LOC2 -> msg (ByteString)
	// LOC3 -> sigCnt (Integer)
	// LOC4 -> pubCnt (Integer)
	emit.InitSlot(vrf.BinWriter, 5, 0)

	// Check the number of signatures is m. Return false if not.
	emit.Opcodes(vrf.BinWriter, opcode.DEPTH) // Push the number of signatures onto stack.
	emit.Int(vrf.BinWriter, int64(m))
	emit.Instruction(vrf.BinWriter, opcode.JMPEQ, []byte{0})            // here and below short jumps are sufficient.
	sigsLenCheckEndOffset := vrf.Len()                                  // offset of the signatures count check.
	emit.Opcodes(vrf.BinWriter, opcode.CLEAR, opcode.PUSHF, opcode.RET) // return if length of the signatures not equal to m.

	// Start the check.
	checkStartOffset := vrf.Len()

	// Pack signatures and store at LOC0.
	emit.Int(vrf.BinWriter, int64(m))
	emit.Opcodes(vrf.BinWriter, opcode.PACK, opcode.STLOC0)

	// Pack public keys and store at LOC1.
	for _, pub := range pubs {
		emit.Bytes(vrf.BinWriter, pub.Bytes())
	}
	emit.Int(vrf.BinWriter, int64(n))
	emit.Opcodes(vrf.BinWriter, opcode.PACK, opcode.STLOC1)

	// Get message and store it at LOC2.
	// msg = [4-network-magic-bytes-LE, tx-hash-BE]
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetNetwork) // push network magic (Integer stackitem), can have 0-5 bytes length serialized.
	// Convert network magic to 4-bytes-length LE byte array representation.
	emit.Int(vrf.BinWriter, 0x100000000)
	emit.Opcodes(vrf.BinWriter, opcode.ADD, // some new number that is 5 bytes at least when serialized, but first 4 bytes are intact network value (LE).
		opcode.PUSH4, opcode.LEFT) // cut the first 4 bytes out of a number that is at least 5 bytes long, the result is 4-bytes-length LE network representation.
	// Retrieve executing transaction hash.
	emit.Syscall(vrf.BinWriter, interopnames.SystemRuntimeGetScriptContainer) // push the script container (executing transaction, actually).
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.PICKITEM)                // pick 0-th transaction item (the transaction hash).
	// Concatenate network magic and transaction hash.
	emit.Opcodes(vrf.BinWriter, opcode.CAT)    // this instruction will convert network magic to bytes using BigInteger rules of conversion.
	emit.Opcodes(vrf.BinWriter, opcode.STLOC2) // store msg as a local variable #2.

	// Initialize local variables: sigCnt, pubCnt.
	emit.Opcodes(vrf.BinWriter, opcode.PUSH0, opcode.STLOC3, // initialize sigCnt.
		opcode.PUSH0, opcode.STLOC4) // initialize pubCnt.

	// Loop condition check.
	loopStartOffset := vrf.Len()
	emit.Opcodes(vrf.BinWriter, opcode.LDLOC3) // load sigCnt.
	emit.Int(vrf.BinWriter, int64(m))          // push m.
	emit.Opcodes(vrf.BinWriter, opcode.GE,     // sigCnt >= m
		opcode.LDLOC4) // load pubCnt
	emit.Int(vrf.BinWriter, int64(n))      // push n.
	emit.Opcodes(vrf.BinWriter, opcode.GE, // pubCnt >= n
		opcode.OR) // sigCnt >= m || pubCnt >= n
	emit.Instruction(vrf.BinWriter, opcode.JMPIF, []byte{0}) // jump to the end of the script if (sigCnt >= m || pubCnt >= n).
	loopConditionOffset := vrf.Len()

	// Loop start. Prepare arguments and call CryptoLib's verifyWithECDsa.
	emit.Int(vrf.BinWriter, int64(native.Secp256k1Keccak256)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.LDLOC0,                // load signatures.
		opcode.LDLOC3,             // load sigCnt.
		opcode.PICKITEM,           // pick signature at index sigCnt.
		opcode.LDLOC1,             // load pubs.
		opcode.LDLOC4,             // load pubCnt.
		opcode.PICKITEM,           // pick pub at index pubCnt.
		opcode.LDLOC2,             // load msg.
		opcode.PUSH4, opcode.PACK) // pack 4 arguments for 'verifyWithECDsa' call.
	emit.AppCallNoArgs(vrf.BinWriter, cryptoLibH, "verifyWithECDsa", callflag.All) // emit the call to 'verifyWithECDsa' itself.

	// Update loop variables.
	emit.Opcodes(vrf.BinWriter, opcode.LDLOC3, opcode.ADD, opcode.STLOC3, // increment sigCnt if signature is valid.
		opcode.LDLOC4, opcode.INC, opcode.STLOC4) // increment pubCnt.

	// End of the loop.
	emit.Instruction(vrf.BinWriter, opcode.JMP, []byte{0}) // jump to the start of cycle.
	loopEndOffset := vrf.Len()

	// Return condition: the number of valid signatures should be equal to m.
	progRetOffset := vrf.Len()
	emit.Opcodes(vrf.BinWriter, opcode.LDLOC3)   // load sigCnt.
	emit.Int(vrf.BinWriter, int64(m))            // push m.
	emit.Opcodes(vrf.BinWriter, opcode.NUMEQUAL) // push m == sigCnt.

	require.NoError(t, vrf.Err)
	script := vrf.Bytes()

	// Set JMP* instructions offsets. "-1" is for short JMP parameter offset. JMP parameters
	// are relative offsets.
	script[sigsLenCheckEndOffset-1] = byte(checkStartOffset - sigsLenCheckEndOffset + 2)
	script[loopEndOffset-1] = byte(loopStartOffset - loopEndOffset + 2)
	script[loopConditionOffset-1] = byte(progRetOffset - loopConditionOffset + 2)

	return script
	// Here's an example of the resulting single witness invocation script (261 bytes length, the length may vary depending on m/n):
	// NEO-GO-VM > loadbase64 VwUAQxMoBUkJQBPAcAwhAnDdr99Ja4K3I81KURO2xs8b+dYYVIaMhbDFTYO4FCnKDCECuBwcms5bdqbWeBZ1cnMAJ8z/uUMcxnIK0CxTyxNdYqAMIQLQHl4aPx8PZOgu4EQUh0qCPaCfaZZPLNNS9ZVPcmuXpwwhA+YKTuJo6wB/u/CQdzJczfQQaMk6LHfMlSZMdBD2qCV1FMBxQcX7oOADAAAAAAEAAACeFI1BLVEIMBDOi3IQcxB0axO4bBS4kiRCABhoa85pbM5qFMAfDA92ZXJpZnlXaXRoRUNEc2EMFBv1dasRiWiEE2EKNaEohs3gtmxyQWJ9W1JrnnNsnHQiuWsTsw==
	// READY: loaded 262 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        INITSLOT     5 local, 0 arg    <<
	// 3        DEPTH
	// 4        PUSH3
	// 5        JMPEQ        10 (5/05)
	// 7        CLEAR
	// 8        PUSHF
	// 9        RET
	// 10       PUSH3
	// 11       PACK
	// 12       STLOC0
	// 13       PUSHDATA1    0270ddafdf496b82b723cd4a5113b6c6cf1bf9d61854868c85b0c54d83b81429ca
	// 48       PUSHDATA1    02b81c1c9ace5b76a6d678167572730027ccffb9431cc6720ad02c53cb135d62a0
	// 83       PUSHDATA1    02d01e5e1a3f1f0f64e82ee04414874a823da09f69964f2cd352f5954f726b97a7
	// 118      PUSHDATA1    03e60a4ee268eb007fbbf09077325ccdf41068c93a2c77cc95264c7410f6a82575
	// 153      PUSH4
	// 154      PACK
	// 155      STLOC1
	// 156      SYSCALL      System.Runtime.GetNetwork (c5fba0e0)
	// 161      PUSHINT64    4294967296 (0000000001000000)
	// 170      ADD
	// 171      PUSH4
	// 172      LEFT
	// 173      SYSCALL      System.Runtime.GetScriptContainer (2d510830)
	// 178      PUSH0
	// 179      PICKITEM
	// 180      CAT
	// 181      STLOC2
	// 182      PUSH0
	// 183      STLOC3
	// 184      PUSH0
	// 185      STLOC4
	// 186      LDLOC3
	// 187      PUSH3
	// 188      GE
	// 189      LDLOC4
	// 190      PUSH4
	// 191      GE
	// 192      OR
	// 193      JMPIF        259 (66/42)
	// 195      PUSHINT8     24 (18)
	// 197      LDLOC0
	// 198      LDLOC3
	// 199      PICKITEM
	// 200      LDLOC1
	// 201      LDLOC4
	// 202      PICKITEM
	// 203      LDLOC2
	// 204      PUSH4
	// 205      PACK
	// 206      PUSH15
	// 207      PUSHDATA1    766572696679576974684543447361 ("verifyWithECDsa")
	// 224      PUSHDATA1    1bf575ab1189688413610a35a12886cde0b66c72 ("NNToUmdQBe5n8o53BTzjTFAnSEcpouyy3B", "0x726cb6e0cd8628a1350a611384688911ab75f51b")
	// 246      SYSCALL      System.Contract.Call (627d5b52)
	// 251      LDLOC3
	// 252      ADD
	// 253      STLOC3
	// 254      LDLOC4
	// 255      INC
	// 256      STLOC4
	// 257      JMP          186 (-71/b9)
	// 259      LDLOC3
	// 260      PUSH3
	// 261      NUMEQUAL
}
