// Package convert provides functions for type conversion.
package convert

// ToInteger converts it's argument to an Integer.
func ToInteger(v any) int {
	return v.(int)
}

// ToBytes converts it's argument to a Buffer VM type.
func ToBytes(v any) []byte {
	return v.([]byte)
}

// ToString converts it's argument to a ByteString VM type.
func ToString(v any) string {
	return v.(string)
}

// ToBool converts it's argument to a Boolean.
func ToBool(v any) bool {
	return v.(bool)
}
