package native

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeprices"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Notary represents Notary native contract.
type Notary struct {
	interop.ContractMD
	GAS   *GAS
	NEO   *NEO
	Desig *Designate

	lock sync.RWMutex
	// isValid defies whether cached values were changed during the current
	// consensus iteration. If false, these values will be updated after
	// blockchain DAO persisting. If true, we can safely use cached values.
	isValid                bool
	maxNotValidBeforeDelta uint32
}

const (
	notaryContractID = reservedContractID - 1
	// prefixDeposit is a prefix for storing Notary deposits.
	prefixDeposit                 = 1
	defaultDepositDeltaTill       = 5760
	defaultMaxNotValidBeforeDelta = 140 // 20 rounds for 7 validators, a little more than half an hour
)

var maxNotValidBeforeDeltaKey = []byte{10}

// newNotary returns Notary native contract.
func newNotary() *Notary {
	n := &Notary{ContractMD: *interop.NewContractMD(nativenames.Notary, notaryContractID)}
	defer n.UpdateHash()

	desc := newDescriptor("onNEP17Payment", smartcontract.VoidType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md := newMethodAndPrice(n.onPayment, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("lockDepositUntil", smartcontract.BoolType,
		manifest.NewParameter("address", smartcontract.Hash160Type),
		manifest.NewParameter("till", smartcontract.IntegerType))
	md = newMethodAndPrice(n.lockDepositUntil, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("withdraw", smartcontract.BoolType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.withdraw, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("addr", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.balanceOf, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("expirationOf", smartcontract.IntegerType,
		manifest.NewParameter("addr", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.expirationOf, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("verify", smartcontract.BoolType,
		manifest.NewParameter("signature", smartcontract.SignatureType))
	md = newMethodAndPrice(n.verify, nativeprices.NotaryVerificationPrice, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getMaxNotValidBeforeDelta", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getMaxNotValidBeforeDelta, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("setMaxNotValidBeforeDelta", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setMaxNotValidBeforeDelta, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	return n
}

// Metadata implements Contract interface.
func (n *Notary) Metadata() *interop.ContractMD {
	return &n.ContractMD
}

// Initialize initializes Notary native contract and implements Contract interface.
func (n *Notary) Initialize(ic *interop.Context) error {
	err := setIntWithKey(n.ID, ic.DAO, maxNotValidBeforeDeltaKey, defaultMaxNotValidBeforeDelta)
	if err != nil {
		return err
	}

	n.isValid = true
	n.maxNotValidBeforeDelta = defaultMaxNotValidBeforeDelta
	return nil
}

// OnPersist implements Contract interface.
func (n *Notary) OnPersist(ic *interop.Context) error {
	var (
		nFees    int64
		notaries keys.PublicKeys
		err      error
	)
	for _, tx := range ic.Block.Transactions {
		if tx.HasAttribute(transaction.NotaryAssistedT) {
			if notaries == nil {
				notaries, err = n.GetNotaryNodes(ic.DAO)
				if err != nil {
					return fmt.Errorf("failed to get notary nodes: %w", err)
				}
			}
			nKeys := tx.GetAttributes(transaction.NotaryAssistedT)[0].Value.(*transaction.NotaryAssisted).NKeys
			nFees += int64(nKeys) + 1
			if tx.Sender() == n.Hash {
				payer := tx.Signers[1]
				balance := n.GetDepositFor(ic.DAO, payer.Account)
				balance.Amount.Sub(balance.Amount, big.NewInt(tx.SystemFee+tx.NetworkFee))
				if balance.Amount.Sign() == 0 {
					err := n.removeDepositFor(ic.DAO, payer.Account)
					if err != nil {
						return fmt.Errorf("failed to remove an empty deposit for %s from storage: %w", payer.Account.StringBE(), err)
					}
				} else {
					err := n.putDepositFor(ic.DAO, balance, payer.Account)
					if err != nil {
						return fmt.Errorf("failed to update deposit for %s: %w", payer.Account.StringBE(), err)
					}
				}
			}
		}
	}
	if nFees == 0 {
		return nil
	}
	singleReward := calculateNotaryReward(nFees, len(notaries))
	for _, notary := range notaries {
		n.GAS.mint(ic, notary.GetScriptHash(), singleReward, false)
	}
	return nil
}

// PostPersist implements Contract interface.
func (n *Notary) PostPersist(ic *interop.Context) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	if n.isValid {
		return nil
	}

	n.maxNotValidBeforeDelta = uint32(getIntWithKey(n.ID, ic.DAO, maxNotValidBeforeDeltaKey))
	n.isValid = true
	return nil
}

// onPayment records deposited amount as belonging to "from" address with a lock
// till the specified chain's height.
func (n *Notary) onPayment(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	if h := ic.VM.GetCallingScriptHash(); h != n.GAS.Hash {
		panic(fmt.Errorf("only GAS can be accepted for deposit, got %s", h.StringBE()))
	}
	from := toUint160(args[0])
	to := from
	amount := toBigInt(args[1])
	data, ok := args[2].(*stackitem.Array)
	if !ok || len(data.Value().([]stackitem.Item)) != 2 {
		panic(errors.New("`data` parameter should be an array of 2 elements"))
	}
	additionalParams := data.Value().([]stackitem.Item)
	if !additionalParams[0].Equals(stackitem.Null{}) {
		to = toUint160(additionalParams[0])
	}

	allowedChangeTill := ic.Tx.Sender() == to
	currentHeight := ic.Chain.BlockHeight()
	deposit := n.GetDepositFor(ic.DAO, to)
	till := toUint32(additionalParams[1])
	if till < currentHeight {
		panic(fmt.Errorf("`till` shouldn't be less then the chain's height %d", currentHeight))
	}
	if deposit != nil && till < deposit.Till {
		panic(fmt.Errorf("`till` shouldn't be less then the previous value %d", deposit.Till))
	}
	if deposit == nil {
		if amount.Cmp(big.NewInt(2*transaction.NotaryServiceFeePerKey)) < 0 {
			panic(fmt.Errorf("first deposit can not be less then %d, got %d", 2*transaction.NotaryServiceFeePerKey, amount.Int64()))
		}
		deposit = &state.Deposit{
			Amount: new(big.Int),
		}
		if !allowedChangeTill {
			till = currentHeight + defaultDepositDeltaTill
		}
	} else if !allowedChangeTill { // only deposit's owner is allowed to set or update `till`
		till = deposit.Till
	}
	deposit.Amount.Add(deposit.Amount, amount)
	deposit.Till = till

	if err := n.putDepositFor(ic.DAO, deposit, to); err != nil {
		panic(fmt.Errorf("failed to put deposit for %s into the storage: %w", from.StringBE(), err))
	}
	return stackitem.Null{}
}

// lockDepositUntil updates the chain's height until which deposit is locked.
func (n *Notary) lockDepositUntil(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	addr := toUint160(args[0])
	ok, err := runtime.CheckHashedWitness(ic, addr)
	if err != nil {
		panic(fmt.Errorf("failed to check witness for %s: %w", addr.StringBE(), err))
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	till := toUint32(args[1])
	if till < ic.Chain.BlockHeight() {
		return stackitem.NewBool(false)
	}
	deposit := n.GetDepositFor(ic.DAO, addr)
	if deposit == nil {
		return stackitem.NewBool(false)
	}
	if till < deposit.Till {
		return stackitem.NewBool(false)
	}
	deposit.Till = till
	err = n.putDepositFor(ic.DAO, deposit, addr)
	if err != nil {
		panic(fmt.Errorf("failed to put deposit for %s into the storage: %w", addr.StringBE(), err))
	}
	return stackitem.NewBool(true)
}

// withdraw sends all deposited GAS for "from" address to "to" address.
func (n *Notary) withdraw(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	from := toUint160(args[0])
	ok, err := runtime.CheckHashedWitness(ic, from)
	if err != nil {
		panic(fmt.Errorf("failed to check witness for %s: %w", from.StringBE(), err))
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	to := from
	if !args[1].Equals(stackitem.Null{}) {
		to = toUint160(args[1])
	}
	deposit := n.GetDepositFor(ic.DAO, from)
	if deposit == nil {
		return stackitem.NewBool(false)
	}
	if ic.Chain.BlockHeight() < deposit.Till {
		return stackitem.NewBool(false)
	}
	cs, err := ic.GetContract(n.GAS.Hash)
	if err != nil {
		panic(fmt.Errorf("failed to get GAS contract state: %w", err))
	}
	transferArgs := []stackitem.Item{stackitem.NewByteArray(n.Hash.BytesBE()), stackitem.NewByteArray(to.BytesBE()), stackitem.NewBigInteger(deposit.Amount), stackitem.Null{}}
	err = contract.CallFromNative(ic, n.Hash, cs, "transfer", transferArgs, true)
	if err != nil {
		panic(fmt.Errorf("failed to transfer GAS from Notary account: %w", err))
	}
	if !ic.VM.Estack().Pop().Bool() {
		panic("failed to transfer GAS from Notary account: `transfer` returned false")
	}
	if err := n.removeDepositFor(ic.DAO, from); err != nil {
		panic(fmt.Errorf("failed to remove withdrawn deposit for %s from the storage: %w", from.StringBE(), err))
	}
	return stackitem.NewBool(true)
}

// balanceOf returns deposited GAS amount for specified address.
func (n *Notary) balanceOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	return stackitem.NewBigInteger(n.BalanceOf(ic.DAO, acc))
}

// BalanceOf is an internal representation of `balanceOf` Notary method.
func (n *Notary) BalanceOf(dao dao.DAO, acc util.Uint160) *big.Int {
	deposit := n.GetDepositFor(dao, acc)
	if deposit == nil {
		return big.NewInt(0)
	}
	return deposit.Amount
}

// expirationOf Returns deposit lock height for specified address.
func (n *Notary) expirationOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	return stackitem.Make(n.ExpirationOf(ic.DAO, acc))
}

// ExpirationOf is an internal representation of `expirationOf` Notary method.
func (n *Notary) ExpirationOf(dao dao.DAO, acc util.Uint160) uint32 {
	deposit := n.GetDepositFor(dao, acc)
	if deposit == nil {
		return 0
	}
	return deposit.Till
}

// verify checks whether the transaction was signed by one of the notaries.
func (n *Notary) verify(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	sig, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("failed to get signature bytes: %w", err))
	}
	tx := ic.Tx
	if len(tx.GetAttributes(transaction.NotaryAssistedT)) == 0 {
		return stackitem.NewBool(false)
	}
	for _, signer := range tx.Signers {
		if signer.Account == n.Hash {
			if signer.Scopes != transaction.None {
				return stackitem.NewBool(false)
			}
			break
		}
	}
	if tx.Sender() == n.Hash {
		if len(tx.Signers) != 2 {
			return stackitem.NewBool(false)
		}
		payer := tx.Signers[1].Account
		balance := n.GetDepositFor(ic.DAO, payer)
		if balance == nil || balance.Amount.Cmp(big.NewInt(tx.NetworkFee+tx.SystemFee)) < 0 {
			return stackitem.NewBool(false)
		}
	}
	notaries, err := n.GetNotaryNodes(ic.DAO)
	if err != nil {
		panic(fmt.Errorf("failed to get notary nodes: %w", err))
	}
	shash := hash.NetSha256(uint32(ic.Network), tx)
	var verified bool
	for _, n := range notaries {
		if n.Verify(sig, shash[:]) {
			verified = true
			break
		}
	}
	return stackitem.NewBool(verified)
}

// GetNotaryNodes returns public keys of notary nodes.
func (n *Notary) GetNotaryNodes(d dao.DAO) (keys.PublicKeys, error) {
	nodes, _, err := n.Desig.GetDesignatedByRole(d, noderoles.P2PNotary, math.MaxUint32)
	return nodes, err
}

// getMaxNotValidBeforeDelta is Notary contract method and returns the maximum NotValidBefore delta.
func (n *Notary) getMaxNotValidBeforeDelta(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(n.GetMaxNotValidBeforeDelta(ic.DAO))))
}

// GetMaxNotValidBeforeDelta is an internal representation of Notary getMaxNotValidBeforeDelta method.
func (n *Notary) GetMaxNotValidBeforeDelta(dao dao.DAO) uint32 {
	n.lock.RLock()
	defer n.lock.RUnlock()
	if n.isValid {
		return n.maxNotValidBeforeDelta
	}
	return uint32(getIntWithKey(n.ID, dao, maxNotValidBeforeDeltaKey))
}

// setMaxNotValidBeforeDelta is Notary contract method and sets the maximum NotValidBefore delta.
func (n *Notary) setMaxNotValidBeforeDelta(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	maxInc := ic.Chain.GetConfig().MaxValidUntilBlockIncrement
	if value > maxInc/2 || value < uint32(ic.Chain.GetConfig().ValidatorsCount) {
		panic(fmt.Errorf("MaxNotValidBeforeDelta cannot be more than %d or less than %d", maxInc/2, ic.Chain.GetConfig().ValidatorsCount))
	}
	if !n.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	n.lock.Lock()
	defer n.lock.Unlock()
	err := setIntWithKey(n.ID, ic.DAO, maxNotValidBeforeDeltaKey, int64(value))
	if err != nil {
		panic(fmt.Errorf("failed to put value into the storage: %w", err))
	}
	n.isValid = false
	return stackitem.Null{}
}

// GetDepositFor returns state.Deposit for the account specified. It returns nil in case if
// deposit is not found in storage and panics in case of any other error.
func (n *Notary) GetDepositFor(dao dao.DAO, acc util.Uint160) *state.Deposit {
	key := append([]byte{prefixDeposit}, acc.BytesBE()...)
	deposit := new(state.Deposit)
	err := getConvertibleFromDAO(n.ID, dao, key, deposit)
	if err == nil {
		return deposit
	}
	if err == storage.ErrKeyNotFound {
		return nil
	}
	panic(fmt.Errorf("failed to get deposit for %s from storage: %w", acc.StringBE(), err))
}

// putDepositFor puts deposit on the balance of the specified account in the storage.
func (n *Notary) putDepositFor(dao dao.DAO, deposit *state.Deposit, acc util.Uint160) error {
	key := append([]byte{prefixDeposit}, acc.BytesBE()...)
	return putConvertibleToDAO(n.ID, dao, key, deposit)
}

// removeDepositFor removes deposit from the storage.
func (n *Notary) removeDepositFor(dao dao.DAO, acc util.Uint160) error {
	key := append([]byte{prefixDeposit}, acc.BytesBE()...)
	return dao.DeleteStorageItem(n.ID, key)
}

// calculateNotaryReward calculates the reward for a single notary node based on FEE's count and Notary nodes count.
func calculateNotaryReward(nFees int64, notariesCount int) *big.Int {
	return big.NewInt(nFees * transaction.NotaryServiceFeePerKey / int64(notariesCount))
}
