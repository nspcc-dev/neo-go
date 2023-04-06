/*
Package nft contains divisible non-fungible NEP-11-compatible token
implementation. This token can be minted with GAS transfer to contract address,
it will retrieve NeoFS container ID and object ID from the transfer data and
produce NFT which represents NeoFS object.
*/
package nft

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/gas"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

const (
	decimals   = 2
	multiplier = 100
)

// Prefixes used for contract data storage.
const (
	totalSupplyPrefix = "s"
	// balancePrefix contains map from [address + token id] to address's balance of the specified token.
	balancePrefix = "b"
	// tokenOwnerPrefix contains map from [token id + owner] to token's owner.
	tokenOwnerPrefix = "t"
	// tokenPrefix contains map from token id to its properties (serialised containerID + objectID).
	tokenPrefix = "i"
)

var (
	// contractOwner is a special address that can perform some management
	// functions on this contract like updating/destroying it and can also
	// be used for contract address verification.
	contractOwner = address.ToHash160("NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB")
)

// ObjectIdentifier represents NFT structure and contains the container ID and
// object ID of the NeoFS object.
type ObjectIdentifier struct {
	ContainerID []byte
	ObjectID    []byte
}

// Common methods

// Symbol returns token symbol, it's NFSO.
func Symbol() string {
	return "NFSO"
}

// Decimals returns token decimals, this NFT is divisible.
func Decimals() int {
	return decimals
}

// TotalSupply is a contract method that returns the number of tokens minted.
func TotalSupply() int {
	return totalSupply(storage.GetReadOnlyContext())
}

// totalSupply is an internal implementation of TotalSupply operating with
// given context. The number itself is stored raw in the DB with totalSupplyPrefix
// key.
func totalSupply(ctx storage.Context) int {
	var res int

	val := storage.Get(ctx, []byte(totalSupplyPrefix))
	if val != nil {
		res = val.(int)
	}
	return res
}

// mkBalancePrefix creates DB key-prefix for account balances specified
// by concatenating balancePrefix and account address.
func mkBalancePrefix(holder interop.Hash160) []byte {
	res := []byte(balancePrefix)
	return append(res, holder...)
}

// mkBalanceKey creates DB key for account specified by concatenating balancePrefix,
// account address and token ID.
func mkBalanceKey(holder interop.Hash160, tokenID []byte) []byte {
	res := mkBalancePrefix(holder)
	return append(res, tokenID...)
}

// mkTokenOwnerPrefix creates DB key prefix for token specified by its ID.
func mkTokenOwnerPrefix(tokenID []byte) []byte {
	res := []byte(tokenOwnerPrefix)
	return append(res, tokenID...)
}

// mkTokenOwnerKey creates DB key for token specified by concatenating tokenOwnerPrefix,
// token ID and holder.
func mkTokenOwnerKey(tokenID []byte, holder interop.Hash160) []byte {
	res := mkTokenOwnerPrefix(tokenID)
	return append(res, holder...)
}

// mkTokenKey creates DB key for token specified by its ID.
func mkTokenKey(tokenID []byte) []byte {
	res := []byte(tokenPrefix)
	return append(res, tokenID...)
}

// BalanceOf returns the overall number of tokens owned by specified address.
func BalanceOf(holder interop.Hash160) int {
	if len(holder) != interop.Hash160Len {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()
	balance := 0
	iter := tokensOf(ctx, holder)
	for iterator.Next(iter) {
		tokenID := iterator.Value(iter).([]byte)
		key := mkBalanceKey(holder, tokenID)
		balance += getBalanceOf(ctx, key)
	}
	return balance
}

// getBalanceOf returns balance of the account of the specified tokenID using
// database key.
func getBalanceOf(ctx storage.Context, balanceKey []byte) int {
	val := storage.Get(ctx, balanceKey)
	if val != nil {
		return val.(int)
	}
	return 0
}

// addToBalance adds amount to the account balance. Amount can be negative. It returns
// updated balance value.
func addToBalance(ctx storage.Context, holder interop.Hash160, tokenID []byte, amount int) int {
	key := mkBalanceKey(holder, tokenID)
	old := getBalanceOf(ctx, key)
	old += amount
	if old > 0 {
		storage.Put(ctx, key, old)
	} else {
		storage.Delete(ctx, key)
	}
	return old
}

// TokensOf returns an iterator with all tokens held by specified address.
func TokensOf(holder interop.Hash160) iterator.Iterator {
	if len(holder) != interop.Hash160Len {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()

	return tokensOf(ctx, holder)
}

func tokensOf(ctx storage.Context, holder interop.Hash160) iterator.Iterator {
	key := mkBalancePrefix(holder)
	// We don't store zero balances, thus only relevant token IDs of the holder will
	// be returned.
	iter := storage.Find(ctx, key, storage.KeysOnly|storage.RemovePrefix)
	return iter
}

// Transfer token from its owner to another user, if there's one owner of the token.
// It will return false if token is shared between multiple owners.
func Transfer(to interop.Hash160, token []byte, data any) bool {
	if len(to) != interop.Hash160Len {
		panic("invalid 'to' address")
	}
	ctx := storage.GetContext()
	var (
		owner interop.Hash160
		ok    bool
	)
	iter := ownersOf(ctx, token)
	for iterator.Next(iter) {
		if ok {
			// Token is shared between multiple owners.
			return false
		}
		owner = iterator.Value(iter).(interop.Hash160)
		ok = true
	}
	if !ok {
		panic("unknown token")
	}

	// Note that although calling script hash is not checked explicitly in
	// this contract it is in fact checked for in `CheckWitness` itself.
	if !runtime.CheckWitness(owner) {
		return false
	}

	key := mkBalanceKey(owner, token)
	amount := getBalanceOf(ctx, key)

	if !owner.Equals(to) {
		addToBalance(ctx, owner, token, -amount)
		removeOwner(ctx, token, owner)

		addToBalance(ctx, to, token, amount)
		addOwner(ctx, token, to)
	}
	postTransfer(owner, to, token, amount, data)
	return true
}

// postTransfer emits Transfer event and calls onNEP11Payment if needed.
func postTransfer(from interop.Hash160, to interop.Hash160, token []byte, amount int, data any) {
	runtime.Notify("Transfer", from, to, amount, token)
	if management.GetContract(to) != nil {
		contract.Call(to, "onNEP11Payment", contract.All, from, amount, token, data)
	}
}

// end of common methods.

// Optional methods.

// Properties returns properties of the given NFT.
func Properties(id []byte) map[string]string {
	ctx := storage.GetReadOnlyContext()
	if !isTokenValid(ctx, id) {
		panic("unknown token")
	}
	key := mkTokenKey(id)
	props := storage.Get(ctx, key).([]byte)
	t := std.Deserialize(props).(ObjectIdentifier)
	result := map[string]string{
		"name":        "NeoFS Object " + std.Base64Encode(id), // Not a hex for contract simplicity.
		"containerID": std.Base64Encode(t.ContainerID),
		"objectID":    std.Base64Encode(t.ObjectID),
	}
	return result
}

// Tokens returns all token IDs minted by the contract.
func Tokens() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	prefix := []byte(tokenPrefix)
	iter := storage.Find(ctx, prefix, storage.KeysOnly|storage.RemovePrefix)
	return iter
}

func isTokenValid(ctx storage.Context, tokenID []byte) bool {
	key := mkTokenKey(tokenID)
	result := storage.Get(ctx, key)
	return result != nil
}

// End of optional methods.

// Divisible methods.

// TransferDivisible token from its owner to another user, notice that it only has three
// parameters because token owner can be deduced from token ID itself.
func TransferDivisible(from, to interop.Hash160, amount int, token []byte, data any) bool {
	if len(from) != interop.Hash160Len {
		panic("invalid 'from' address")
	}
	if len(to) != interop.Hash160Len {
		panic("invalid 'to' address")
	}
	if amount < 0 {
		panic("negative 'amount'")
	}
	if amount > multiplier {
		panic("invalid 'amount'")
	}
	ctx := storage.GetContext()
	if !isTokenValid(ctx, token) {
		panic("unknown token")
	}

	// Note that although calling script hash is not checked explicitly in
	// this contract it is in fact checked for in `CheckWitness` itself.
	if !runtime.CheckWitness(from) {
		return false
	}

	key := mkBalanceKey(from, token)
	balance := getBalanceOf(ctx, key)
	if amount > balance {
		return false
	}

	if !from.Equals(to) {
		updBalance := addToBalance(ctx, from, token, -amount)
		if updBalance == 0 {
			removeOwner(ctx, token, from)
		}

		updBalance = addToBalance(ctx, to, token, amount)
		if updBalance != 0 {
			addOwner(ctx, token, to)
		}
	}
	postTransfer(from, to, token, amount, data)
	return true
}

// OwnerOf returns owner of specified token.
func OwnerOf(token []byte) iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	if !isTokenValid(ctx, token) {
		panic("unknown token")
	}
	return ownersOf(ctx, token)
}

// BalanceOfDivisible returns the number of token with the specified tokenID owned by specified address.
func BalanceOfDivisible(holder interop.Hash160, token []byte) int {
	if len(holder) != interop.Hash160Len {
		panic("bad holder address")
	}
	ctx := storage.GetReadOnlyContext()
	key := mkBalanceKey(holder, token)
	return getBalanceOf(ctx, key)
}

// end of divisible methods.

// ownersOf returns iterator over owners of the specified token. Owner is
// stored as value of the token key (prefix + token ID + owner).
func ownersOf(ctx storage.Context, token []byte) iterator.Iterator {
	key := mkTokenOwnerPrefix(token)
	iter := storage.Find(ctx, key, storage.ValuesOnly)
	return iter
}

func addOwner(ctx storage.Context, token []byte, holder interop.Hash160) {
	key := mkTokenOwnerKey(token, holder)
	storage.Put(ctx, key, holder)
}

func removeOwner(ctx storage.Context, token []byte, holder interop.Hash160) {
	key := mkTokenOwnerKey(token, holder)
	storage.Delete(ctx, key)
}

// OnNEP17Payment mints tokens if at least 10 GAS is provided. You don't call
// this method directly, instead it's called by GAS contract when you transfer
// GAS from your address to the address of this NFT contract.
func OnNEP17Payment(from interop.Hash160, amount int, data any) {
	defer func() {
		if r := recover(); r != nil {
			runtime.Log(r.(string))
			util.Abort()
		}
	}()
	callingHash := runtime.GetCallingScriptHash()
	if !callingHash.Equals(gas.Hash) {
		panic("only GAS is accepted")
	}
	if amount < 10_00000000 {
		panic("minting NFSO costs at least 10 GAS")
	}
	tokenInfo := data.([]any)
	if len(tokenInfo) != 2 {
		panic("invalid 'data'")
	}
	containerID := tokenInfo[0].([]byte)
	if len(containerID) != interop.Hash256Len {
		panic("invalid container ID")
	}
	objectID := tokenInfo[1].([]byte)
	if len(objectID) != interop.Hash256Len {
		panic("invalid object ID")
	}

	t := ObjectIdentifier{
		ContainerID: containerID,
		ObjectID:    objectID,
	}
	props := std.Serialize(t)
	id := crypto.Ripemd160(props)

	var ctx = storage.GetContext()
	if isTokenValid(ctx, id) {
		panic("NFSO for the specified object is already minted")
	}
	key := mkTokenKey(id)
	storage.Put(ctx, key, props)

	total := totalSupply(ctx)

	addOwner(ctx, id, from)
	addToBalance(ctx, from, id, multiplier)

	total++
	storage.Put(ctx, []byte(totalSupplyPrefix), total)

	postTransfer(nil, from, id, multiplier, nil) // no `data` during minting
}

// Verify allows owner to manage contract's address, including earned GAS
// transfer from contract's address to somewhere else. It just checks for transaction
// to also be signed by contract owner, so contract's witness should be empty.
func Verify() bool {
	return runtime.CheckWitness(contractOwner)
}

// Destroy destroys the contract, only owner can do that.
func Destroy() {
	if !Verify() {
		panic("only owner can destroy")
	}
	management.Destroy()
}

// Update updates the contract, only owner can do that.
func Update(nef, manifest []byte) {
	if !Verify() {
		panic("only owner can update")
	}
	management.Update(nef, manifest)
}
