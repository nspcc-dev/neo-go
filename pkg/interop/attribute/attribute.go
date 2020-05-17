/*
Package attribute provides getters for transaction attributes.
*/
package attribute

// Attribute represents transaction attribute in Neo, it's an opaque data
// structure that you can get data from only using functions from this package.
// It's similar in function to the TransactionAttribute class in the Neo .net
// framework. To use it you need to get is first using transaction.GetAttributes.
type Attribute struct{}

// GetUsage returns the Usage field of the given attribute. It is an enumeration
// with the following possible values:
//      ContractHash   = 0x00
//      ECDH02         = 0x02
//      ECDH03         = 0x03
//      Script         = 0x20
//      Vote           = 0x30
//      CertURL        = 0x80
//      DescriptionURL = 0x81
//      Description    = 0x90
//
//      Hash1  = 0xa1
//      Hash2  = 0xa2
//      Hash3  = 0xa3
//      Hash4  = 0xa4
//      Hash5  = 0xa5
//      Hash6  = 0xa6
//      Hash7  = 0xa7
//      Hash8  = 0xa8
//      Hash9  = 0xa9
//      Hash10 = 0xaa
//      Hash11 = 0xab
//      Hash12 = 0xac
//      Hash13 = 0xad
//      Hash14 = 0xae
//      Hash15 = 0xaf
//
//      Remark   = 0xf0
//      Remark1  = 0xf1
//      Remark2  = 0xf2
//      Remark3  = 0xf3
//      Remark4  = 0xf4
//      Remark5  = 0xf5
//      Remark6  = 0xf6
//      Remark7  = 0xf7
//      Remark8  = 0xf8
//      Remark9  = 0xf9
//      Remark10 = 0xfa
//      Remark11 = 0xfb
//      Remark12 = 0xfc
//      Remark13 = 0xfd
//      Remark14 = 0xfe
//      Remark15 = 0xff
// This function uses `Neo.Attribute.GetUsage` syscall internally.
func GetUsage(attr Attribute) byte {
	return 0x00
}

// GetData returns the data of the given attribute, exact interpretation of this
// data depends on attribute's Usage type. It uses `Neo.Attribute.GetData`
// syscall internally.
func GetData(attr Attribute) []byte {
	return nil
}
