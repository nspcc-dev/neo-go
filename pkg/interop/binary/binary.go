/*
Package binary provides binary serialization routines.
*/
package binary

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Serialize serializes any given item into a byte slice. It works for all
// regular VM types (not ones from interop package) and allows to save them in
// storage or pass into Notify and then Deserialize them on the next run or in
// the external event receiver. It uses `System.Binary.Serialize` syscall.
func Serialize(item interface{}) []byte {
	return neogointernal.Syscall1("System.Binary.Serialize", item).([]byte)
}

// Deserialize unpacks previously serialized value from a byte slice, it's the
// opposite of Serialize. It uses `System.Binary.Deserialize` syscall.
func Deserialize(b []byte) interface{} {
	return neogointernal.Syscall1("System.Binary.Deserialize", b)
}

// Base64Encode encodes given byte slice into a base64 string and returns byte
// representation of this string. It uses `System.Binary.Base64Encode` interop.
func Base64Encode(b []byte) string {
	return neogointernal.Syscall1("System.Binary.Base64Encode", b).(string)
}

// Base64Decode decodes given base64 string represented as a byte slice into
// byte slice. It uses `System.Binary.Base64Decode` interop.
func Base64Decode(b []byte) []byte {
	return neogointernal.Syscall1("System.Binary.Base64Decode", b).([]byte)
}

// Base58Encode encodes given byte slice into a base58 string and returns byte
// representation of this string. It uses `System.Binary.Base58Encode` syscall.
func Base58Encode(b []byte) string {
	return neogointernal.Syscall1("System.Binary.Base58Encode", b).(string)
}

// Base58Decode decodes given base58 string represented as a byte slice into
// a new byte slice. It uses `System.Binary.Base58Decode` syscall.
func Base58Decode(b []byte) []byte {
	return neogointernal.Syscall1("System.Binary.Base64Decode", b).([]byte)
}

// Itoa converts num in a given base to string. Base should be either 10 or 16.
// It uses `System.Binary.Itoa` syscall.
func Itoa(num int, base int) string {
	return neogointernal.Syscall2("System.Binary.Itoa", num, base).(string)
}

// Atoi converts string to a number in a given base. Base should be either 10 or 16.
// It uses `System.Binary.Atoi` syscall.
func Atoi(s string, base int) int {
	return neogointernal.Syscall2("System.Binary.Atoi", s, base).(int)
}
