package wallet

//// PublicKey represent a NEO public key.
//type PublicKey struct {
//	crypto.EllipticCurvePoint
//}
//
//// Bytes returns a byteslice representation of the PublicKey.
//func (p *PublicKey) Bytes() []byte {
//	b := p.X.Bytes()
//	padding := append(
//		bytes.Repeat(
//			[]byte{0x00},
//			32-len(b),
//		),
//		b...,
//	)
//
//	prefix := []byte{0x03}
//	if p.Y.Bit(0) == 0 {
//		prefix = []byte{0x02}
//	}
//
//	return append(prefix, padding...)
//}
//
//// Signature creates the signature of the PublicKey.
//func (p *PublicKey) Signature() []byte {
//	b := p.Bytes()
//
//	b = append([]byte{0x21}, b...)
//	b = append(b, 0xAC)
//
//	sha256H := sha256.New()
//	sha256H.Write(b)
//	hash := sha256H.Sum(nil)
//
//	ripemd160H := ripemd160.New()
//	ripemd160H.Write(hash)
//	return ripemd160H.Sum(nil)
//}
//
//// PublicAddress derives the public NEO address that is coupled with the private key,
//// and returns it as a string.
//func (p *PublicKey) PublicAddress() string {
//	var (
//		b        = p.Signature()
//		ver byte = 0x17
//	)
//
//	b = append([]byte{ver}, b...)
//
//	sha256H := sha256.New()
//	sha256H.Write(b)
//	hash := sha256H.Sum(nil)
//
//	sha256H.Reset()
//	sha256H.Write(hash)
//	hash = sha256H.Sum(nil)
//
//	b = append(b, hash[0:4]...)
//
//	return crypto.Base58Encode(b)
//}
