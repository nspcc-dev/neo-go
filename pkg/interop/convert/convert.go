// Package convert provides functions for type conversion.
package convert

// ToInteger converts it's argument to an Integer.
func ToInteger(v interface{}) int {
	return v.(int)
}

// ToBytes converts it's argument to a Buffer VM type.
func ToBytes(v interface{}) []byte {
	return v.([]byte)
}

// ToString converts it's argument to a ByteString VM type.
func ToString(v interface{}) string {
	return v.(string)
}

// ToBool converts it's argument to a Boolean.
func ToBool(v interface{}) bool {
	return v.(bool)
}
