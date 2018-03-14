package transaction

// AttrUsage represents the purpose of the attribute.
type AttrUsage uint8

// List of valid attribute usages.
const (
	ContractHash   AttrUsage = 0x00
	ECDH02         AttrUsage = 0x02
	ECDH03         AttrUsage = 0x03
	Script         AttrUsage = 0x20
	Vote           AttrUsage = 0x30
	CertURL        AttrUsage = 0x80
	DescriptionURL AttrUsage = 0x81
	Description    AttrUsage = 0x90

	Hash1  AttrUsage = 0xa1
	Hash2  AttrUsage = 0xa2
	Hash3  AttrUsage = 0xa3
	Hash4  AttrUsage = 0xa4
	Hash5  AttrUsage = 0xa5
	Hash6  AttrUsage = 0xa6
	Hash7  AttrUsage = 0xa7
	Hash8  AttrUsage = 0xa8
	Hash9  AttrUsage = 0xa9
	Hash10 AttrUsage = 0xaa
	Hash11 AttrUsage = 0xab
	Hash12 AttrUsage = 0xac
	Hash13 AttrUsage = 0xad
	Hash14 AttrUsage = 0xae
	Hash15 AttrUsage = 0xaf

	Remark   AttrUsage = 0xf0
	Remark1  AttrUsage = 0xf1
	Remark2  AttrUsage = 0xf2
	Remark3  AttrUsage = 0xf3
	Remark4  AttrUsage = 0xf4
	Remark5  AttrUsage = 0xf5
	Remark6  AttrUsage = 0xf6
	Remark7  AttrUsage = 0xf7
	Remark8  AttrUsage = 0xf8
	Remark9  AttrUsage = 0xf9
	Remark10 AttrUsage = 0xfa
	Remark11 AttrUsage = 0xfb
	Remark12 AttrUsage = 0xfc
	Remark13 AttrUsage = 0xfd
	Remark14 AttrUsage = 0xfe
	Remark15 AttrUsage = 0xff
)
