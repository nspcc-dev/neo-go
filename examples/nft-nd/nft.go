/*
Package nft contains non-divisible non-fungible NEP-11-compatible token
implementation. This token can be minted with GAS transfer to contract address,
it will hash some data (including data provided in transfer) and produce a
base64-encoded string that is your NFT. Since it's based on hashing and basically
you own a hash it's HASHY.
*/
package nft

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/crypto"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/gas"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Prefixes used for contract data storage.
const (
	totalSupplyPrefix = "s"
	// balancePrefix contains map from addresses to balances.
	balancePrefix = "b"
	// accountPrefix contains map from address + token id to tokens
	accountPrefix = "a"
	// tokenPrefix contains map from token id to it's owner.
	tokenPrefix = "t"
)

var (
	// contractOwner is a special address that can perform some management
	// functions on this contract like updating/destroying it and can also
	// be used for contract address verification.
	contractOwner = util.FromAddress("NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB")
)

// Symbol returns token symbol, it's HASHY.
func Symbol() string {
	return "HASHY"
}

// Decimals returns token decimals, this NFT is non-divisible, so it's 0.
func Decimals() int {
	return 0
}

// TotalSupply is a contract method that returns the number of tokens minted.
func TotalSupply() int {
	return totalSupply(storage.GetReadOnlyContext())
}

// totalSupply is an internal implementation of TotalSupply operating with
// the given context. The number itself is stored raw in the DB with totalSupplyPrefix
// key.
func totalSupply(ctx storage.Context) int {
	var res int

	val := storage.Get(ctx, []byte(totalSupplyPrefix))
	if val != nil {
		res = val.(int)
	}
	return res
}

// mkAccountPrefix creates DB key-prefix for the account tokens specified
// by concatenating accountPrefix and account address.
func mkAccountPrefix(holder interop.Hash160) []byte {
	res := []byte(accountPrefix)
	return append(res, holder...)
}

// mkBalanceKey creates DB key for the account specified by concatenating balancePrefix
// and account address.
func mkBalanceKey(holder interop.Hash160) []byte {
	res := []byte(balancePrefix)
	return append(res, holder...)
}

// mkTokenKey creates DB key for the token specified by concatenating tokenPrefix
// and token ID.
func mkTokenKey(tokenID []byte) []byte {
	res := []byte(tokenPrefix)
	return append(res, tokenID...)
}

// BalanceOf returns the number of tokens owned by the specified address.
func BalanceOf(holder interop.Hash160) int {
	if len(holder) != 20 {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()
	return getBalanceOf(ctx, mkBalanceKey(holder))
}

// getBalanceOf returns the balance of an account using database key.
func getBalanceOf(ctx storage.Context, balanceKey []byte) int {
	val := storage.Get(ctx, balanceKey)
	if val != nil {
		return val.(int)
	}
	return 0
}

// addToBalance adds an amount to the account balance. Amount can be negative.
func addToBalance(ctx storage.Context, holder interop.Hash160, amount int) {
	key := mkBalanceKey(holder)
	old := getBalanceOf(ctx, key)
	old += amount
	if old > 0 {
		storage.Put(ctx, key, old)
	} else {
		storage.Delete(ctx, key)
	}
}

// addToken adds a token to the account.
func addToken(ctx storage.Context, holder interop.Hash160, token []byte) {
	key := mkAccountPrefix(holder)
	storage.Put(ctx, append(key, token...), token)
}

// removeToken removes the token from the account.
func removeToken(ctx storage.Context, holder interop.Hash160, token []byte) {
	key := mkAccountPrefix(holder)
	storage.Delete(ctx, append(key, token...))
}

// Tokens returns an iterator that contains all of the tokens minted by the contract.
func Tokens() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	key := []byte(tokenPrefix)
	iter := storage.Find(ctx, key, storage.RemovePrefix|storage.KeysOnly)
	return iter
}

// TokensOf returns an iterator with all tokens held by the specified address.
func TokensOf(holder interop.Hash160) iterator.Iterator {
	if len(holder) != 20 {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()
	key := mkAccountPrefix(holder)
	iter := storage.Find(ctx, key, storage.ValuesOnly)
	return iter
}

// getOwnerOf returns the current owner of the specified token or panics if token
// ID is invalid. The owner is stored as a value of the token key (prefix + token ID).
func getOwnerOf(ctx storage.Context, token []byte) interop.Hash160 {
	key := mkTokenKey(token)
	val := storage.Get(ctx, key)
	if val == nil {
		panic("no token found")
	}
	return val.(interop.Hash160)
}

// setOwnerOf writes the current owner of the specified token into the DB.
func setOwnerOf(ctx storage.Context, token []byte, holder interop.Hash160) {
	key := mkTokenKey(token)
	storage.Put(ctx, key, holder)
}

// OwnerOf returns the owner of the specified token.
func OwnerOf(token []byte) interop.Hash160 {
	ctx := storage.GetReadOnlyContext()
	return getOwnerOf(ctx, token)
}

// Transfer token from its owner to another user, notice that it only has three
// parameters because token owner can be deduced from token ID itself.
func Transfer(to interop.Hash160, token []byte, data interface{}) bool {
	if len(to) != 20 {
		panic("invalid 'to' address")
	}
	ctx := storage.GetContext()
	owner := getOwnerOf(ctx, token)

	// Note that although calling script hash is not checked explicitly in
	// this contract it is in fact checked for in `CheckWitness` itself.
	if !runtime.CheckWitness(owner) {
		return false
	}

	if !owner.Equals(to) {
		addToBalance(ctx, owner, -1)
		removeToken(ctx, owner, token)

		addToBalance(ctx, to, 1)
		addToken(ctx, to, token)
		setOwnerOf(ctx, token, to)
	}
	postTransfer(owner, to, token, data)
	return true
}

// postTransfer emits Transfer event and calls onNEP11Payment if needed.
func postTransfer(from interop.Hash160, to interop.Hash160, token []byte, data interface{}) {
	runtime.Notify("Transfer", from, to, 1, token)
	if management.GetContract(to) != nil {
		contract.Call(to, "onNEP11Payment", contract.All, from, 1, token, data)
	}
}

// OnNEP17Payment mints tokens if at least 10 GAS is provided. You don't call
// this method directly, instead it's called by GAS contract when you transfer
// GAS from your address to the address of this NFT contract.
func OnNEP17Payment(from interop.Hash160, amount int, data interface{}) {
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
		panic("minting HASHY costs at least 10 GAS")
	}
	var tokIn = []byte{}
	var ctx = storage.GetContext()

	total := totalSupply(ctx)
	tokIn = append(tokIn, []byte(std.Itoa(total, 10))...)
	tokIn = append(tokIn, []byte(std.Itoa(amount, 10))...)
	tokIn = append(tokIn, from...)
	tx := runtime.GetScriptContainer()
	tokIn = append(tokIn, tx.Hash...)
	if data != nil {
		tokIn = append(tokIn, std.Serialize(data)...)
	}

	tokenHash := crypto.Ripemd160(tokIn)
	token := std.Base64Encode(tokenHash)

	addToken(ctx, from, []byte(token))
	setOwnerOf(ctx, []byte(token), from)
	addToBalance(ctx, from, 1)

	total++
	storage.Put(ctx, []byte(totalSupplyPrefix), total)

	postTransfer(nil, from, []byte(token), nil) // no `data` during minting
}

// Verify allows an owner to manage a contract's address, including earned GAS
// transfer from the contract's address to somewhere else. It just checks for the transaction
// to also be signed by the contract owner, so contract's witness should be empty.
func Verify() bool {
	return runtime.CheckWitness(contractOwner)
}

// Destroy destroys the contract, only its owner can do that.
func Destroy() {
	if !Verify() {
		panic("only owner can destroy")
	}
	management.Destroy()
}

// Update updates the contract, only its owner can do that.
func Update(nef, manifest []byte) {
	if !Verify() {
		panic("only owner can update")
	}
	management.Update(nef, manifest)
}

// Properties returns properties of the given NFT.
func Properties(id []byte) map[string]string {
	ctx := storage.GetReadOnlyContext()
	owner := storage.Get(ctx, mkTokenKey(id)).(interop.Hash160)
	if owner == nil {
		panic("unknown token")
	}
	result := map[string]string{
		"name": "HASHY " + std.Base64Encode(id), // Not a hex for contract simplicity.
	}
	return result
}
