package transaction

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage  uint8
	Length uint8
	Data   []byte
}
