/*
Package nft contains non-divisible non-fungible NEP11-compatible token
implementation. This token can be minted with GAS transfer to contract address,
it will hash some data (including data provided in transfer) and produce
base58-encoded string that is your NFT. Since it's based on hashing and basically
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
	accountPrefix     = "a"
	tokenPrefix       = "t"
	tokensPrefix      = "ts"
)

var (
	// contractOwner is a special address that can perform some management
	// functions on this contract like updating/destroying it and can also
	// be used for contract address verification.
	contractOwner = util.FromAddress("NX1yL5wDx3inK2qUVLRVaqCLUxYnAbv85S")
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

// mkAccountKey creates DB key for account specified by concatenating accountPrefix
// and account address.
func mkAccountKey(holder interop.Hash160) []byte {
	res := []byte(accountPrefix)
	return append(res, holder...)
}

// mkStringKey creates DB key for token specified by concatenating tokenPrefix
// and token ID.
func mkTokenKey(token []byte) []byte {
	res := []byte(tokenPrefix)
	return append(res, token...)
}

func mkTokensKey() []byte {
	return []byte(tokensPrefix)
}

// BalanceOf returns the number of tokens owned by specified address.
func BalanceOf(holder interop.Hash160) int {
	if len(holder) != 20 {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()
	tokens := getTokensOf(ctx, holder)
	return len(tokens)
}

// getTokensOf is an internal implementation of TokensOf, tokens are stored
// as a serialized slice of strings in the DB, so it gets and unwraps them
// (or returns an empty slice).
func getTokensOf(ctx storage.Context, holder interop.Hash160) []string {
	var res = []string{}

	key := mkAccountKey(holder)
	val := storage.Get(ctx, key)
	if val != nil {
		res = std.Deserialize(val.([]byte)).([]string)
	}
	return res
}

// setTokensOf saves current tokens owned by account if there are any,
// otherwise it just drops the appropriate key from the DB.
func setTokensOf(ctx storage.Context, holder interop.Hash160, tokens []string) {
	key := mkAccountKey(holder)
	if len(tokens) != 0 {
		val := std.Serialize(tokens)
		storage.Put(ctx, key, val)
	} else {
		storage.Delete(ctx, key)
	}
}

// setTokens saves minted token if it is not saved yet.
func setTokens(ctx storage.Context, newToken string) {
	key := mkTokensKey()
	var tokens = []string{}
	val := storage.Get(ctx, key)
	if val != nil {
		tokens = std.Deserialize(val.([]byte)).([]string)
	}
	for i := 0; i < len(tokens); i++ {
		if util.Equals(tokens[i], newToken) {
			return
		}
	}
	tokens = append(tokens, newToken)
	val = std.Serialize(tokens)
	storage.Put(ctx, key, val)
}

// Tokens returns an iterator that contains all of the tokens minted by the contract.
func Tokens() iterator.Iterator {
	ctx := storage.GetReadOnlyContext()
	var arr = []string{}
	key := mkTokensKey()
	val := storage.Get(ctx, key)
	if val != nil {
		arr = std.Deserialize(val.([]byte)).([]string)
	}
	return iterator.Create(arr)
}

// TokensOf returns an iterator with all tokens held by specified address.
func TokensOf(holder interop.Hash160) iterator.Iterator {
	if len(holder) != 20 {
		panic("bad owner address")
	}
	ctx := storage.GetReadOnlyContext()
	tokens := getTokensOf(ctx, holder)

	return iterator.Create(tokens)
}

// getOwnerOf returns current owner of the specified token or panics if token
// ID is invalid. Owner is stored as value of the token key (prefix + token ID).
func getOwnerOf(ctx storage.Context, token []byte) interop.Hash160 {
	key := mkTokenKey(token)
	val := storage.Get(ctx, key)
	if val == nil {
		panic("no token found")
	}
	return val.(interop.Hash160)
}

// setOwnerOf writes current owner of the specified token into the DB.
func setOwnerOf(ctx storage.Context, token []byte, holder interop.Hash160) {
	key := mkTokenKey(token)
	storage.Put(ctx, key, holder)
}

// OwnerOf returns owner of specified token.
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

	if string(owner) != string(to) {
		toksOwner := getTokensOf(ctx, owner)
		toksTo := getTokensOf(ctx, to)

		var newToksOwner = []string{}
		for _, tok := range toksOwner {
			if tok != string(token) {
				newToksOwner = append(newToksOwner, tok)
			}
		}
		toksTo = append(toksTo, string(token))
		setTokensOf(ctx, owner, newToksOwner)
		setTokensOf(ctx, to, toksTo)
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
	if string(runtime.GetCallingScriptHash()) != gas.Hash {
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

	tokenHash := crypto.Sha256(tokIn)
	token := std.Base58Encode(tokenHash)

	toksOf := getTokensOf(ctx, from)
	toksOf = append(toksOf, token)
	setTokensOf(ctx, from, toksOf)
	setOwnerOf(ctx, []byte(token), from)
	setTokens(ctx, token)

	total++
	storage.Put(ctx, []byte(totalSupplyPrefix), total)

	postTransfer(nil, from, []byte(token), nil) // no `data` during minting
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

// Properties returns properties of the given NFT.
func Properties(id []byte) map[string]string {
	ctx := storage.GetReadOnlyContext()
	var tokens = []string{}
	key := mkTokensKey()
	val := storage.Get(ctx, key)
	if val != nil {
		tokens = std.Deserialize(val.([]byte)).([]string)
	}
	var exists bool
	for i := 0; i < len(tokens); i++ {
		if util.Equals(tokens[i], id) {
			exists = true
			break
		}
	}
	if !exists {
		panic("unknown token")
	}
	result := map[string]string{
		"name": "HASHY " + string(id),
	}
	return result
}
