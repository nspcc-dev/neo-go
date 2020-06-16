/*
Package binary provides binary serialization routines.
*/
package binary

// Serialize serializes any given item into a byte slice. It works for all
// regular VM types (not ones from interop package) and allows to save them in
// storage or pass into Notify and then Deserialize them on the next run or in
// the external event receiver. It uses `System.Binary.Serialize` syscall.
func Serialize(item interface{}) []byte {
	return nil
}

// Deserialize unpacks previously serialized value from a byte slice, it's the
// opposite of Serialize. It uses `System.Binary.Deserialize` syscall.
func Deserialize(b []byte) interface{} {
	return nil
}
