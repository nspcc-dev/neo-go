package native

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Notary represents Notary native contract.
type Notary struct {
	interop.ContractMD
	GAS   *GAS
	Desig *Designate
}

const (
	notaryName       = "Notary"
	notaryContractID = reservedContractID - 1

	// prefixDeposit is a prefix for storing Notary deposits.
	prefixDeposit           = 1
	defaultDepositDeltaTill = 5760
)

// newNotary returns Notary native contract.
func newNotary() *Notary {
	n := &Notary{ContractMD: *interop.NewContractMD(notaryName)}
	n.ContractID = notaryContractID

	desc := newDescriptor("onPayment", smartcontract.VoidType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md := newMethodAndPrice(n.onPayment, 100_0000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("lockDepositUntil", smartcontract.BoolType,
		manifest.NewParameter("address", smartcontract.Hash160Type),
		manifest.NewParameter("till", smartcontract.IntegerType))
	md = newMethodAndPrice(n.lockDepositUntil, 100_0000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("withdraw", smartcontract.BoolType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.withdraw, 100_0000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("addr", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.balanceOf, 100_0000, smartcontract.AllowStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("expirationOf", smartcontract.IntegerType,
		manifest.NewParameter("addr", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.expirationOf, 100_0000, smartcontract.AllowStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("verify", smartcontract.BoolType,
		manifest.NewParameter("signature", smartcontract.SignatureType))
	md = newMethodAndPrice(n.verify, 100_0000, smartcontract.AllowStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("onPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(n.OnPersist), 0, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("postPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(postPersistBase), 0, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc)

	return n
}

// Metadata implements Contract interface.
func (n *Notary) Metadata() *interop.ContractMD {
	return &n.ContractMD
}

// Initialize initializes Notary native contract and implements Contract interface.
func (n *Notary) Initialize(ic *interop.Context) error {
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
				balance := n.getDepositFor(ic.DAO, payer.Account)
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
	deposit := n.getDepositFor(ic.DAO, to)
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
	deposit := n.getDepositFor(ic.DAO, addr)
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
	deposit := n.getDepositFor(ic.DAO, from)
	if deposit == nil {
		return stackitem.NewBool(false)
	}
	if ic.Chain.BlockHeight() < deposit.Till {
		return stackitem.NewBool(false)
	}
	cs, err := ic.DAO.GetContractState(n.GAS.Hash)
	if err != nil {
		panic(fmt.Errorf("failed to get GAS contract state: %w", err))
	}
	transferArgs := []stackitem.Item{stackitem.NewByteArray(n.Hash.BytesBE()), stackitem.NewByteArray(to.BytesBE()), stackitem.NewBigInteger(deposit.Amount), stackitem.Null{}}
	err = contract.CallExInternal(ic, cs, "transfer", transferArgs, smartcontract.All, vm.EnsureIsEmpty, func(ctx *vm.Context) { // we need EnsureIsEmpty because there's a callback popping result from the stack
		isTransferOk := ic.VM.Estack().Pop().Bool()
		if !isTransferOk {
			panic("failed to transfer GAS from Notary account")
		}
	})
	if err != nil {
		panic(fmt.Errorf("failed to transfer GAS from Notary account: %w", err))
	}
	if err := n.removeDepositFor(ic.DAO, from); err != nil {
		panic(fmt.Errorf("failed to remove withdrawn deposit for %s from the storage: %w", from.StringBE(), err))
	}
	return stackitem.NewBool(true)
}

// balanceOf returns deposited GAS amount for specified address.
func (n *Notary) balanceOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	deposit := n.getDepositFor(ic.DAO, acc)
	if deposit == nil {
		return stackitem.NewBigInteger(big.NewInt(0))
	}
	return stackitem.NewBigInteger(deposit.Amount)
}

// expirationOf Returns deposit lock height for specified address.
func (n *Notary) expirationOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	deposit := n.getDepositFor(ic.DAO, acc)
	if deposit == nil {
		return stackitem.Make(0)
	}
	return stackitem.Make(deposit.Till)
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
		}
	}
	if tx.Sender() == n.Hash {
		if len(tx.Signers) != 2 {
			return stackitem.NewBool(false)
		}
		payer := tx.Signers[1].Account
		balance := n.getDepositFor(ic.DAO, payer)
		if balance == nil || balance.Amount.Cmp(big.NewInt(tx.NetworkFee+tx.SystemFee)) < 0 {
			return stackitem.NewBool(false)
		}
	}
	notaries, err := n.GetNotaryNodes(ic.DAO)
	if err != nil {
		panic(fmt.Errorf("failed to get notary nodes: %w", err))
	}
	hash := tx.GetSignedHash().BytesBE()
	var verified bool
	for _, n := range notaries {
		if n.Verify(sig, hash) {
			verified = true
			break
		}
	}
	return stackitem.NewBool(verified)
}

// GetNotaryNodes returns public keys of notary nodes.
func (n *Notary) GetNotaryNodes(d dao.DAO) (keys.PublicKeys, error) {
	nodes, _, err := n.Desig.GetDesignatedByRole(d, RoleP2PNotary, math.MaxUint32)
	return nodes, err
}

// getDepositFor returns state.Deposit for the account specified. It returns nil in case if
// deposit is not found in storage and panics in case of any other error.
func (n *Notary) getDepositFor(dao dao.DAO, acc util.Uint160) *state.Deposit {
	key := append([]byte{prefixDeposit}, acc.BytesBE()...)
	deposit := new(state.Deposit)
	err := getSerializableFromDAO(n.ContractID, dao, key, deposit)
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
	return putSerializableToDAO(n.ContractID, dao, key, deposit)
}

// removeDepositFor removes deposit from the storage.
func (n *Notary) removeDepositFor(dao dao.DAO, acc util.Uint160) error {
	key := append([]byte{prefixDeposit}, acc.BytesBE()...)
	return dao.DeleteStorageItem(n.ContractID, key)
}

// calculateNotaryReward calculates the reward for a single notary node based on FEE's count and Notary nodes count.
func calculateNotaryReward(nFees int64, notariesCount int) *big.Int {
	return big.NewInt(nFees * transaction.NotaryServiceFeePerKey / int64(notariesCount))
}
