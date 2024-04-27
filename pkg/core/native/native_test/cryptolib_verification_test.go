package native_test

import (
	"math/big"
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

		// The user has to sign the Sha256 hash of the message by his Koblitz key.
		// Please, note that this Sha256 hash may easily be replaced by Keccaak hash via minor adjustment of
		// CryptoLib's `verifyWithECDsa` behaviour (if needed).
		signature := pk.SignHash(hash.Sha256(msg))

		// Ensure that signature verification passes. This line here is just for testing purposes,
		// it won't be present in the real code.
		require.True(t, pk.PublicKey().Verify(signature, hash.Sha256(msg).BytesBE()))

		// Build invocation witness script for the user's account.
		invBytes := buildKoblitzInvocationScript(t, signature)

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

	// The simplest witness verification script with low length and low execution cost
	// (98 bytes, 2092530 GAS including Invocation script execution).
	// The user has to sign the sha256([var-bytes-network-magic, txHash-bytes-BE]).
	check(t, buildKoblitzVerificationScriptSimpleSingleHash, constructMessageNoHash)

	// Even more simple witness verification script with low length and low execution cost
	// (95 bytes, 2092320 GAS including Invocation script execution).
	// The user has to sign the sha256([var-bytes-network-magic, txHash-bytes-BE]).
	// The difference is that network magic is a static value, thus, both verification script and
	// user address are network-specific.
	check(t, buildKoblitzVerificationScriptSimpleSingleHashStaticMagic, constructMessageNoHash)

	// More complicated verification script with higher length and higher execution cost
	// (136 bytes, 4120620 GAS including Invocation script execution).
	// The user has to sign the sha256(sha256([var-bytes-network-magic, txHash-bytes-BE])).
	check(t, buildKoblitzVerificationScriptSimple, constructMessageSimple)

	// Witness verification script that follows the existing standard CheckSig account generation rules
	// and has larger length and higher execution cost.
	// (186 bytes, 5116020 GAS including Invocation script execution).
	// The user has to sign the sha256(sha256([4-bytes-network-magic-LE, txHash-bytes-BE]))
	check(t, buildKoblitzVerificationScriptCompat, constructMessageCompat)
}

// buildKoblitzVerificationScriptSimpleSingleHash builds witness verification script for Koblitz public key.
// This method differs from buildKoblitzVerificationScriptCompat in that it checks
//
//	sha256([var-bytes-network-magic, txHash-bytes-BE])
//
// instead of (comparing with N3)
//
//	sha256([4-bytes-network-magic-LE, txHash-bytes-BE]).
func buildKoblitzVerificationScriptSimpleSingleHash(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	// vrf is witness verification script corresponding to the pk.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)         // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())           // emit the caller's public key.
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
	// 0        PUSHINT8     22 (16)    <<
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
//	sha256([var-bytes-network-magic, txHash-bytes-BE])
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
	emit.Int(vrf.BinWriter, int64(native.Secp256k1)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)         // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())           // emit the caller's public key.
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
	// 0        PUSHINT8     22 (16)    <<
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
//	sha256(sha256([var-bytes-network-magic, txHash-bytes-BE]))
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
	emit.Int(vrf.BinWriter, int64(native.Secp256k1)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)         // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())           // emit the caller's public key.
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
	// 0        PUSHINT8     22 (16)    <<
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
//	sha256(sha256([4-bytes-network-magic-LE, txHash-bytes-BE]))
//
// It produces constant-length verification script (186 bytes) independently of the network parameters.
func buildKoblitzVerificationScriptCompat(t *testing.T, pub *keys.PublicKey) []byte {
	criptoLibH := state.CreateNativeContractHash(nativenames.CryptoLib)

	// vrf is witness verification script corresponding to the pub.
	vrf := io.NewBufBinWriter()
	emit.Int(vrf.BinWriter, int64(native.Secp256k1)) // push Koblitz curve identifier.
	emit.Opcodes(vrf.BinWriter, opcode.SWAP)         // swap curve identifier with the signature.
	emit.Bytes(vrf.BinWriter, pub.Bytes())           // emit the caller's public key.
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
	// 0        PUSHINT8     22 (16)    <<
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

// buildKoblitzInvocationScript builds witness invocation script for the transaction signature. The signature
// itself may be produced by public key over any curve (not required Koblitz, the algorithm is the same).
func buildKoblitzInvocationScript(t *testing.T, signature []byte) []byte {
	//Exactly like during standard
	// signature verification, the resulting script pushes Koblitz signature bytes onto stack.
	inv := io.NewBufBinWriter()
	emit.Bytes(inv.BinWriter, signature) // message signatre bytes.
	require.NoError(t, inv.Err)

	return inv.Bytes()
	// Here's an example of the resulting witness invocation script (66 bytes length, always constant length):
	// NEO-GO-VM > loadbase64 DEBMGKU/MdSizlzaVNDUUbd1zMZQJ43eTaZ4vBCpmkJ/wVh1TYrAWEbFyHhkqq+aYxPCUS43NKJdJTXavcjB8sTP
	// READY: loaded 66 instructions
	// NEO-GO-VM 0 > ops
	// INDEX    OPCODE       PARAMETER
	// 0        PUSHDATA1    4c18a53f31d4a2ce5cda54d0d451b775ccc650278dde4da678bc10a99a427fc158754d8ac05846c5c87864aaaf9a6313c2512e3734a25d2535dabdc8c1f2c4cf    <<
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
