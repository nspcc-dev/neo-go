package nameservice

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
)

// RecordType represents NameService record type.
type RecordType byte

// Various record type.
const (
	TypeA     RecordType = 1
	TypeCNAME RecordType = 5
	TypeTXT   RecordType = 16
	TypeAAAA  RecordType = 28
)

// Hash represents NameService contract hash.
const Hash = "\x8c\x02\xb8\x43\x98\x6b\x3c\x44\x4f\xf8\x6a\xd5\xa9\x43\xfe\x8d\xb6\x24\xb5\xa2"

// Symbol represents `symbol` method of NameService native contract.
func Symbol() string {
	return contract.Call(interop.Hash160(Hash), "symbol", contract.NoneFlag).(string)
}

// Decimals represents `decimals` method of NameService native contract.
func Decimals() int {
	return contract.Call(interop.Hash160(Hash), "decimals", contract.NoneFlag).(int)
}

// TotalSupply represents `totalSupply` method of NameService native contract.
func TotalSupply() int {
	return contract.Call(interop.Hash160(Hash), "totalSupply", contract.ReadStates).(int)
}

// OwnerOf represents `ownerOf` method of NameService native contract.
func OwnerOf(tokenID string) interop.Hash160 {
	return contract.Call(interop.Hash160(Hash), "ownerOf", contract.ReadStates, tokenID).(interop.Hash160)
}

// BalanceOf represents `balanceOf` method of NameService native contract.
func BalanceOf(owner interop.Hash160) int {
	return contract.Call(interop.Hash160(Hash), "balanceOf", contract.ReadStates, owner).(int)
}

// Properties represents `properties` method of NameService native contract.
func Properties(tokenID string) map[string]interface{} {
	return contract.Call(interop.Hash160(Hash), "properties", contract.ReadStates, tokenID).(map[string]interface{})
}

// Tokens represents `tokens` method of NameService native contract.
func Tokens() iterator.Iterator {
	return contract.Call(interop.Hash160(Hash), "tokens",
		contract.ReadStates).(iterator.Iterator)
}

// TokensOf represents `tokensOf` method of NameService native contract.
func TokensOf(addr interop.Hash160) iterator.Iterator {
	return contract.Call(interop.Hash160(Hash), "tokensOf",
		contract.ReadStates, addr).(iterator.Iterator)
}

// Transfer represents `transfer` method of NameService native contract.
func Transfer(to interop.Hash160, tokenID string) bool {
	return contract.Call(interop.Hash160(Hash), "transfer",
		contract.WriteStates|contract.AllowNotify, to, tokenID).(bool)
}

// AddRoot represents `addRoot` method of NameService native contract.
func AddRoot(root string) {
	contract.Call(interop.Hash160(Hash), "addRoot", contract.WriteStates, root)
}

// SetPrice represents `setPrice` method of NameService native contract.
func SetPrice(price int) {
	contract.Call(interop.Hash160(Hash), "setPrice", contract.WriteStates, price)
}

// GetPrice represents `getPrice` method of NameService native contract.
func GetPrice() int {
	return contract.Call(interop.Hash160(Hash), "getPrice", contract.ReadStates).(int)
}

// IsAvailable represents `isAvailable` method of NameService native contract.
func IsAvailable(name string) bool {
	return contract.Call(interop.Hash160(Hash), "isAvailable", contract.ReadStates, name).(bool)
}

// Register represents `register` method of NameService native contract.
func Register(name string, owner interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "register", contract.WriteStates, name, owner).(bool)
}

// Renew represents `renew` method of NameService native contract.
func Renew(name string) int {
	return contract.Call(interop.Hash160(Hash), "renew", contract.WriteStates, name).(int)
}

// SetAdmin represents `setAdmin` method of NameService native contract.
func SetAdmin(name string, admin interop.Hash160) {
	contract.Call(interop.Hash160(Hash), "setAdmin", contract.WriteStates, name, admin)
}

// SetRecord represents `setRecord` method of NameService native contract.
func SetRecord(name string, recType RecordType, data string) {
	contract.Call(interop.Hash160(Hash), "setRecord", contract.WriteStates, name, recType, data)
}

// GetRecord represents `getRecord` method of NameService native contract.
// It returns `nil` if record is missing.
func GetRecord(name string, recType RecordType) []byte {
	return contract.Call(interop.Hash160(Hash), "getRecord", contract.ReadStates, name, recType).([]byte)
}

// DeleteRecord represents `deleteRecord` method of NameService native contract.
func DeleteRecord(name string, recType RecordType) {
	contract.Call(interop.Hash160(Hash), "deleteRecord", contract.WriteStates, name, recType)
}

// Resolve represents `resolve` method of NameService native contract.
func Resolve(name string, recType RecordType) []byte {
	return contract.Call(interop.Hash160(Hash), "resolve", contract.ReadStates, name, recType).([]byte)
}
