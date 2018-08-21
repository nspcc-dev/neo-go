package attribute

// Package attribute provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Attribute stubs a NEO transaction attribute type.
type Attribute struct{}

// GetUsage returns the usage of the given attribute.
func GetUsage(attr Attribute) byte {
	return 0x00
}

// GetData returns the data of the given attribute.
func GetData(attr Attribute) []byte {
	return nil
}
