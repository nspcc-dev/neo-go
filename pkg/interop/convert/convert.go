// Package convert provides functions for type conversion.
package convert

// ToInteger converts it's argument to an Integer.
func ToInteger(v interface{}) int64 {
	return v.(int64)
}

// ToByteArray converts it's argument to a ByteArray.
func ToByteArray(v interface{}) []byte {
	return v.([]byte)
}

// ToBool converts it's argument to a Boolean.
func ToBool(v interface{}) bool {
	return v.(bool)
}
