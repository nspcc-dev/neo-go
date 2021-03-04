// Package convert provides functions for type conversion.
package convert

// ToInteger converts it's argument to an Integer.
func ToInteger(v interface{}) int {
	return v.(int)
}

// ToByteArray converts it's argument to a ByteArray.
func ToByteArray(v interface{}) []byte {
	return v.([]byte)
}

// ToBool converts it's argument to a Boolean.
func ToBool(v interface{}) bool {
	return v.(bool)
}
