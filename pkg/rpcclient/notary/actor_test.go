package notary

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

type RPCClient struct {
	err     error
	invRes  *result.Invoke
	netFee  int64
	bCount  uint32
	version *result.Version
	hash    util.Uint256
	nhash   util.Uint256
	mirror  bool
}

func (r *RPCClient) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	return r.netFee, r.err
}
func (r *RPCClient) GetBlockCount() (uint32, error) {
	return r.bCount, r.err
}
func (r *RPCClient) GetVersion() (*result.Version, error) {
	verCopy := *r.version
	return &verCopy, r.err
}
func (r *RPCClient) SendRawTransaction(tx *transaction.Transaction) (util.Uint256, error) {
	return r.hash, r.err
}
func (r *RPCClient) SubmitP2PNotaryRequest(req *payload.P2PNotaryRequest) (util.Uint256, error) {
	if r.mirror {
		return req.FallbackTransaction.Hash(), nil
	}
	return r.nhash, r.err
}
func (r *RPCClient) TerminateSession(sessionID uuid.UUID) (bool, error) {
	return false, nil // Just a stub, unused by actor.
}
func (r *RPCClient) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	return nil, nil // Just a stub, unused by actor.
}

func TestNewActor(t *testing.T) {
	rc := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
	}

	_, err := NewActor(rc, nil, nil)
	require.Error(t, err)

	var (
		keyz  [4]*keys.PrivateKey
		accs  [4]*wallet.Account
		faccs [4]*wallet.Account
		pkeys [4]*keys.PublicKey
	)
	for i := range accs {
		keyz[i], err = keys.NewPrivateKey()
		require.NoError(t, err)
		accs[i] = wallet.NewAccountFromPrivateKey(keyz[i])
		pkeys[i] = keyz[i].PublicKey()
		faccs[i] = FakeSimpleAccount(pkeys[i])
	}
	var multiAccs [4]*wallet.Account
	for i := range accs {
		multiAccs[i] = &wallet.Account{}
		*multiAccs[i] = *accs[i]
		require.NoError(t, multiAccs[i].ConvertMultisig(smartcontract.GetDefaultHonestNodeCount(len(pkeys)), pkeys[:]))
	}

	// nil Contract
	badMultiAcc0 := &wallet.Account{}
	*badMultiAcc0 = *multiAccs[0]
	badMultiAcc0.Contract = nil
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: badMultiAcc0,
	}}, accs[0])
	require.Error(t, err)

	// Non-standard script.
	badMultiAcc0.Contract = &wallet.Contract{}
	*badMultiAcc0.Contract = *multiAccs[0].Contract
	badMultiAcc0.Contract.Script = append(badMultiAcc0.Contract.Script, byte(opcode.NOP))
	badMultiAcc0.Address = address.Uint160ToString(badMultiAcc0.Contract.ScriptHash())
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: badMultiAcc0.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: badMultiAcc0,
	}}, accs[0])
	require.Error(t, err)

	// Too many keys
	var (
		manyKeys  [256]*keys.PrivateKey
		manyPkeys [256]*keys.PublicKey
	)
	for i := range manyKeys {
		manyKeys[i], err = keys.NewPrivateKey()
		require.NoError(t, err)
		manyPkeys[i] = manyKeys[i].PublicKey()
	}
	bigMultiAcc := &wallet.Account{}
	*bigMultiAcc = *wallet.NewAccountFromPrivateKey(manyKeys[0])
	require.NoError(t, bigMultiAcc.ConvertMultisig(129, manyPkeys[:]))

	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: bigMultiAcc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: bigMultiAcc,
	}}, wallet.NewAccountFromPrivateKey(manyKeys[0]))
	require.Error(t, err)

	// No contract in the simple account.
	badSimple0 := &wallet.Account{}
	*badSimple0 = *accs[0]
	badSimple0.Contract = nil
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, badSimple0)
	require.Error(t, err)

	// Simple account that can't sign.
	badSimple0 = FakeSimpleAccount(pkeys[0])
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, badSimple0)
	require.Error(t, err)

	// Multisig account instead of simple one.
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, multiAccs[0])
	require.Error(t, err)

	// Main actor freaking out on hash mismatch.
	_, err = NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: accs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, accs[0])
	require.Error(t, err)

	// FB actor freaking out on hash mismatch.
	opts := NewDefaultActorOptions(NewReader(invoker.New(rc, nil)), accs[0])
	opts.FbSigner.Signer.Account = multiAccs[0].Contract.ScriptHash()
	_, err = NewTunedActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, opts)
	require.Error(t, err)

	// Good, one multisig.
	multi0, err := NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAccs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAccs[0],
	}}, accs[0])
	require.NoError(t, err)

	script := []byte{byte(opcode.RET)}
	rc.invRes = &result.Invoke{
		State:       "HALT",
		GasConsumed: 3,
		Script:      script,
		Stack:       []stackitem.Item{stackitem.Make(42)},
	}
	tx, err := multi0.MakeRun(script)
	require.NoError(t, err)
	require.Equal(t, 1, len(tx.Attributes))
	require.Equal(t, transaction.NotaryAssistedT, tx.Attributes[0].Type)
	require.Equal(t, &transaction.NotaryAssisted{NKeys: 4}, tx.Attributes[0].Value)

	// Good, 4 single sigs with one that can sign and one contract.
	single4, err := NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: accs[0].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: accs[0],
	}, {
		Signer: transaction.Signer{
			Account: faccs[1].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: faccs[1],
	}, {
		Signer: transaction.Signer{
			Account: faccs[2].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: faccs[2],
	}, {
		Signer: transaction.Signer{
			Account: accs[3].Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: faccs[3],
	}, {
		Signer: transaction.Signer{
			Account: util.Uint160{1, 2, 3},
			Scopes:  transaction.CalledByEntry,
		},
		Account: FakeContractAccount(util.Uint160{1, 2, 3}),
	}}, accs[0])
	require.NoError(t, err)

	tx, err = single4.MakeRun(script)
	require.NoError(t, err)
	require.Equal(t, 1, len(tx.Attributes))
	require.Equal(t, transaction.NotaryAssistedT, tx.Attributes[0].Type)
	require.Equal(t, &transaction.NotaryAssisted{NKeys: 4}, tx.Attributes[0].Value) // One account can sign, three need to collect additional sigs.
}

func TestSendRequestExactly(t *testing.T) {
	rc := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
	}

	key0, err := keys.NewPrivateKey()
	require.NoError(t, err)
	key1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	acc0 := wallet.NewAccountFromPrivateKey(key0)
	facc1 := FakeSimpleAccount(key1.PublicKey())

	act, err := NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: acc0.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc0,
	}, {
		Signer: transaction.Signer{
			Account: facc1.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: facc1,
	}}, acc0)
	require.NoError(t, err)

	script := []byte{byte(opcode.RET)}
	mainTx := transaction.New(script, 1)
	fbTx := transaction.New(script, 1)

	// Hashes mismatch
	_, _, _, err = act.SendRequestExactly(mainTx, fbTx)
	require.Error(t, err)

	// Error returned
	rc.err = errors.New("")
	_, _, _, err = act.SendRequestExactly(mainTx, fbTx)
	require.Error(t, err)

	// OK returned
	rc.err = nil
	rc.nhash = fbTx.Hash()
	mHash, fbHash, vub, err := act.SendRequestExactly(mainTx, fbTx)
	require.NoError(t, err)
	require.Equal(t, mainTx.Hash(), mHash)
	require.Equal(t, fbTx.Hash(), fbHash)
	require.Equal(t, mainTx.ValidUntilBlock, vub)
}

func TestSendRequest(t *testing.T) {
	rc := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
		bCount: 42,
	}

	key0, err := keys.NewPrivateKey()
	require.NoError(t, err)
	key1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	acc0 := wallet.NewAccountFromPrivateKey(key0)
	facc0 := FakeSimpleAccount(key0.PublicKey())
	facc1 := FakeSimpleAccount(key1.PublicKey())

	act, err := NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: acc0.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc0,
	}, {
		Signer: transaction.Signer{
			Account: facc1.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: facc1,
	}}, acc0)
	require.NoError(t, err)

	script := []byte{byte(opcode.RET)}
	rc.invRes = &result.Invoke{
		State:       "HALT",
		GasConsumed: 3,
		Script:      script,
		Stack:       []stackitem.Item{stackitem.Make(42)},
	}

	mainTx, err := act.MakeRun(script)
	require.NoError(t, err)

	// No attributes.
	fbTx, err := act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	fbTx.Attributes = nil
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)

	// Bad NVB.
	fbTx, err = act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	fbTx.Attributes[1].Type = transaction.HighPriority
	fbTx.Attributes[1].Value = nil
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)

	// Bad Conflicts.
	fbTx, err = act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	fbTx.Attributes[2].Type = transaction.HighPriority
	fbTx.Attributes[2].Value = nil
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)

	// GetBlockCount error.
	fbTx, err = act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	rc.err = errors.New("")
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)

	// Can't sign suddenly.
	rc.err = nil
	acc0Backup := &wallet.Account{}
	*acc0Backup = *acc0
	*acc0 = *facc0
	fbTx, err = act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)

	// Good.
	*acc0 = *acc0Backup
	fbTx, err = act.FbActor.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	_, _, _, err = act.SendRequest(mainTx, fbTx)
	require.Error(t, err)
}

func TestNotarize(t *testing.T) {
	rc := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
		bCount: 42,
	}

	key0, err := keys.NewPrivateKey()
	require.NoError(t, err)
	key1, err := keys.NewPrivateKey()
	require.NoError(t, err)

	acc0 := wallet.NewAccountFromPrivateKey(key0)
	facc1 := FakeSimpleAccount(key1.PublicKey())

	act, err := NewActor(rc, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: acc0.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc0,
	}, {
		Signer: transaction.Signer{
			Account: facc1.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: facc1,
	}}, acc0)
	require.NoError(t, err)

	script := []byte{byte(opcode.RET)}

	// Immediate error from MakeRun.
	rc.invRes = &result.Invoke{
		State:       "FAULT",
		GasConsumed: 3,
		Script:      script,
		Stack:       []stackitem.Item{stackitem.Make(42)},
	}
	_, _, _, err = act.Notarize(act.MakeRun(script))
	require.Error(t, err)

	// Explicitly good transaction. but failure to create a fallback.
	rc.invRes.State = "HALT"
	tx, err := act.MakeRun(script)
	require.NoError(t, err)

	rc.invRes.State = "FAULT"
	_, _, _, err = act.Notarize(tx, nil)
	require.Error(t, err)

	// FB hash mismatch from SendRequestExactly.
	rc.invRes.State = "HALT"
	_, _, _, err = act.Notarize(act.MakeRun(script))
	require.Error(t, err)

	// Good.
	rc.mirror = true
	mHash, fbHash, vub, err := act.Notarize(act.MakeRun(script))
	require.NoError(t, err)
	require.NotEqual(t, util.Uint256{}, mHash)
	require.NotEqual(t, util.Uint256{}, fbHash)
	require.Equal(t, uint32(92), vub)
}

func TestDefaultActorOptions(t *testing.T) {
	rc := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
	}
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	opts := NewDefaultActorOptions(NewReader(invoker.New(rc, nil)), acc)
	rc.invRes = &result.Invoke{
		State:       "HALT",
		GasConsumed: 3,
		Script:      opts.FbScript,
		Stack:       []stackitem.Item{stackitem.Make(42)},
	}
	tx := transaction.New(opts.FbScript, 1)
	require.Error(t, opts.MainCheckerModifier(&result.Invoke{State: "FAULT"}, tx))
	rc.invRes.State = "FAULT"
	require.Error(t, opts.MainCheckerModifier(&result.Invoke{State: "HALT"}, tx))
	rc.invRes.State = "HALT"
	require.NoError(t, opts.MainCheckerModifier(&result.Invoke{State: "HALT"}, tx))
	require.Equal(t, uint32(42), tx.ValidUntilBlock)
}
