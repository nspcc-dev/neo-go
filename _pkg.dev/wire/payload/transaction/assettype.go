package transaction

// AssetType represent a NEO asset type
type AssetType uint8

// Valid asset types.
const (
	CreditFlag     AssetType = 0x40
	DutyFlag       AssetType = 0x80
	GoverningToken AssetType = 0x00
	UtilityToken   AssetType = 0x01
	Currency       AssetType = 0x08
	Share          AssetType = DutyFlag | 0x10
	Invoice        AssetType = DutyFlag | 0x18
	Token          AssetType = CreditFlag | 0x20
)
