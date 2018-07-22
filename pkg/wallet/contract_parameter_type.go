package wallet

const (
	Signature        = byte(0x00)
	Boolean          = byte(0x01)
	Int              = byte(0x02)
	Hash160          = byte(0x03)
	Hash256          = byte(0x04)
	ByteArray        = byte(0x05)
	PublicKey        = byte(0x06)
	String           = byte(0x07)
	Array            = byte(0x10)
	InteropInterface = byte(0xf0)
	Void             = byte(0xff)
)
