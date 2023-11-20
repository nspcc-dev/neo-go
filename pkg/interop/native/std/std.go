/*
Package std provides an interface to StdLib native contract.
It implements various useful conversion functions.
*/
package std

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents StdLib contract hash.
const Hash = "\xc0\xef\x39\xce\xe0\xe4\xe9\x25\xc6\xc2\xa0\x6a\x79\xe1\x44\x0d\xd8\x6f\xce\xac"

// Serialize calls `serialize` method of StdLib native contract and serializes
// any given item into a byte slice. It works for all regular VM types (not ones
// from interop package) and allows to save them in the storage or pass them into Notify
// and then Deserialize them on the next run or in the external event receiver.
func Serialize(item any) []byte {
	return neogointernal.CallWithToken(Hash, "serialize", int(contract.NoneFlag),
		item).([]byte)
}

// Deserialize calls `deserialize` method of StdLib native contract and unpacks
// a previously serialized value from a byte slice, it's the opposite of Serialize.
func Deserialize(b []byte) any {
	return neogointernal.CallWithToken(Hash, "deserialize", int(contract.NoneFlag),
		b)
}

// JSONSerialize serializes a value to json. It uses `jsonSerialize` method of StdLib native
// contract.
// Serialization format is the following:
// []byte -> base64 string
// bool -> json boolean
// nil -> Null
// string -> base64 encoded sequence of underlying bytes
// (u)int* -> integer, only value in -2^53..2^53 are allowed
// []interface{} -> json array
// []any -> json array
// map[type1]type2 -> json object with string keys marshaled as strings (not base64).
func JSONSerialize(item any) []byte {
	return neogointernal.CallWithToken(Hash, "jsonSerialize", int(contract.NoneFlag),
		item).([]byte)
}

// JSONDeserialize deserializes a value from json. It uses `jsonDeserialize` method of StdLib
// native contract.
// It performs deserialization as follows:
//
//	strings -> []byte (string) from base64
//	integers -> (u)int* types
//	null -> interface{}(nil)
//	arrays -> []interface{}
//	maps -> map[string]interface{}
func JSONDeserialize(data []byte) any {
	return neogointernal.CallWithToken(Hash, "jsonDeserialize", int(contract.NoneFlag),
		data)
}

// Base64Encode calls `base64Encode` method of StdLib native contract and encodes
// the given byte slice into a base64 string and returns byte representation of this
// string.
func Base64Encode(b []byte) string {
	return neogointernal.CallWithToken(Hash, "base64Encode", int(contract.NoneFlag),
		b).(string)
}

// Base64Decode calls `base64Decode` method of StdLib native contract and decodes
// the given base64 string represented as a byte slice into byte slice.
func Base64Decode(b []byte) []byte {
	return neogointernal.CallWithToken(Hash, "base64Decode", int(contract.NoneFlag),
		b).([]byte)
}

// Base58Encode calls `base58Encode` method of StdLib native contract and encodes
// the given byte slice into a base58 string and returns byte representation of this
// string.
func Base58Encode(b []byte) string {
	return neogointernal.CallWithToken(Hash, "base58Encode", int(contract.NoneFlag),
		b).(string)
}

// Base58Decode calls `base58Decode` method of StdLib native contract and decodes
// the given base58 string represented as a byte slice into a new byte slice.
func Base58Decode(b []byte) []byte {
	return neogointernal.CallWithToken(Hash, "base58Decode", int(contract.NoneFlag),
		b).([]byte)
}

// Base58CheckEncode calls `base58CheckEncode` method of StdLib native contract and encodes
// the given byte slice into a base58 string with checksum and returns byte representation of this
// string.
func Base58CheckEncode(b []byte) string {
	return neogointernal.CallWithToken(Hash, "base58CheckEncode", int(contract.NoneFlag),
		b).(string)
}

// Base58CheckDecode calls `base58CheckDecode` method of StdLib native contract and decodes
// thr given base58 string with a checksum represented as a byte slice into a new byte slice.
func Base58CheckDecode(b []byte) []byte {
	return neogointernal.CallWithToken(Hash, "base58CheckDecode", int(contract.NoneFlag),
		b).([]byte)
}

// Itoa converts num in the given base to a string. Base should be either 10 or 16.
// It uses `itoa` method of StdLib native contract.
func Itoa(num int, base int) string {
	return neogointernal.CallWithToken(Hash, "itoa", int(contract.NoneFlag),
		num, base).(string)
}

// Itoa10 converts num in base 10 to a string.
// It uses `itoa` method of StdLib native contract.
func Itoa10(num int) string {
	return neogointernal.CallWithToken(Hash, "itoa", int(contract.NoneFlag),
		num).(string)
}

// Atoi converts a string to a number in the given base. Base should be either 10 or 16.
// It uses `atoi` method of StdLib native contract.
func Atoi(s string, base int) int {
	return neogointernal.CallWithToken(Hash, "atoi", int(contract.NoneFlag),
		s, base).(int)
}

// Atoi10 converts a string to a number in base 10.
// It uses `atoi` method of StdLib native contract.
func Atoi10(s string) int {
	return neogointernal.CallWithToken(Hash, "atoi", int(contract.NoneFlag),
		s).(int)
}

// MemoryCompare is similar to bytes.Compare:
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
// It uses `memoryCompare` method of StdLib native contract.
func MemoryCompare(s1, s2 []byte) int {
	return neogointernal.CallWithToken(Hash, "memoryCompare", int(contract.NoneFlag),
		s1, s2).(int)
}

// MemorySearch returns the index of the first occurrence of the val in the mem.
// If not found, -1 is returned. It uses `memorySearch` method of StdLib native contract.
func MemorySearch(mem, pattern []byte) int {
	return neogointernal.CallWithToken(Hash, "memorySearch", int(contract.NoneFlag),
		mem, pattern).(int)
}

// MemorySearchIndex returns the index of the first occurrence of the val in the mem starting from the start.
// If not found, -1 is returned. It uses `memorySearch` method of StdLib native contract.
func MemorySearchIndex(mem, pattern []byte, start int) int {
	return neogointernal.CallWithToken(Hash, "memorySearch", int(contract.NoneFlag),
		mem, pattern, start).(int)
}

// MemorySearchLastIndex returns the index of the last occurrence of the val in the mem ending before start.
// If not found, -1 is returned. It uses `memorySearch` method of StdLib native contract.
func MemorySearchLastIndex(mem, pattern []byte, start int) int {
	return neogointernal.CallWithToken(Hash, "memorySearch", int(contract.NoneFlag),
		mem, pattern, start, true).(int)
}

// StringSplit splits s by occurrences of the sep.
// It uses `stringSplit` method of StdLib native contract.
func StringSplit(s, sep string) []string {
	return neogointernal.CallWithToken(Hash, "stringSplit", int(contract.NoneFlag),
		s, sep).([]string)
}

// StringSplitNonEmpty splits s by occurrences of the sep and returns a list of non-empty items.
// It uses `stringSplit` method of StdLib native contract.
func StringSplitNonEmpty(s, sep string) []string {
	return neogointernal.CallWithToken(Hash, "stringSplit", int(contract.NoneFlag),
		s, sep, true).([]string)
}

// StrLen returns length of the string in Utf- characters.
// It uses `strLen` method of StdLib native contract.
func StrLen(s string) int {
	return neogointernal.CallWithToken(Hash, "strLen", int(contract.NoneFlag),
		s).(int)
}
