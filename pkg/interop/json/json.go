/*
Package json provides various JSON serialization/deserialization routines.
*/
package json

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

// ToJSON serializes value to json. It uses `System.Json.Serialize` syscall.
// Serialization format is the following:
// []byte -> base64 string
// bool -> json boolean
// nil -> Null
// string -> base64 encoded sequence of underlying bytes
// (u)int* -> integer, only value in -2^53..2^53 are allowed
// []interface{} -> json array
// map[type1]type2 -> json object with string keys marshaled as strings (not base64).
func ToJSON(item interface{}) []byte {
	return neogointernal.Syscall1("System.Json.Serialize", item).([]byte)
}

// FromJSON deserializes value from json. It uses `System.Json.Deserialize` syscall.
// It performs deserialization as follows:
// strings -> []byte (string) from base64
// integers -> (u)int* types
// null -> interface{}(nil)
// arrays -> []interface{}
// maps -> map[string]interface{}
func FromJSON(data []byte) interface{} {
	return neogointernal.Syscall1("System.Json.Deserialize", data).(interface{})
}
