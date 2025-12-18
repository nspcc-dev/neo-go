package native

import (
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/twmb/murmur3"
	"golang.org/x/crypto/sha3"
)

// Crypto represents CryptoLib contract.
type Crypto struct {
	interop.ContractMD
}

// HashFunc is a delegate representing a hasher function with 256 bytes output length.
type HashFunc func([]byte) util.Uint256

// NamedCurveHash identifies a pair of named elliptic curve and hash function.
type NamedCurveHash byte

// Various pairs of named elliptic curves and hash functions.
const (
	Secp256k1Sha256    NamedCurveHash = 22
	Secp256r1Sha256    NamedCurveHash = 23
	Secp256k1Keccak256 NamedCurveHash = 122
	Secp256r1Keccak256 NamedCurveHash = 123
)

func newCrypto() *Crypto {
	c := &Crypto{ContractMD: *interop.NewContractMD(nativenames.CryptoLib, nativeids.CryptoLib)}
	defer c.BuildHFSpecificMD(c.ActiveIn())

	desc := NewDescriptor("sha256", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md := NewMethodAndPrice(c.sha256, 1<<15, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("ripemd160", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.ripemd160, 1<<15, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("murmur32", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType),
		manifest.NewParameter("seed", smartcontract.IntegerType))
	md = NewMethodAndPrice(c.murmur32, 1<<13, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("verifyWithECDsa", smartcontract.BoolType,
		manifest.NewParameter("message", smartcontract.ByteArrayType),
		manifest.NewParameter("pubkey", smartcontract.ByteArrayType),
		manifest.NewParameter("signature", smartcontract.ByteArrayType),
		manifest.NewParameter("curve", smartcontract.IntegerType))
	md = NewMethodAndPrice(c.verifyWithECDsaPreCockatrice, 1<<15, callflag.NoneFlag, config.HFDefault, config.HFCockatrice)
	c.AddMethod(md, desc)

	desc = NewDescriptor("verifyWithECDsa", smartcontract.BoolType,
		manifest.NewParameter("message", smartcontract.ByteArrayType),
		manifest.NewParameter("pubkey", smartcontract.ByteArrayType),
		manifest.NewParameter("signature", smartcontract.ByteArrayType),
		manifest.NewParameter("curveHash", smartcontract.IntegerType))
	md = NewMethodAndPrice(c.verifyWithECDsa, 1<<15, callflag.NoneFlag, config.HFCockatrice)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Serialize", smartcontract.ByteArrayType,
		manifest.NewParameter("g", smartcontract.InteropInterfaceType))
	md = NewMethodAndPrice(c.bls12381Serialize, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Deserialize", smartcontract.InteropInterfaceType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381Deserialize, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Equal", smartcontract.BoolType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("y", smartcontract.InteropInterfaceType))
	md = NewMethodAndPrice(c.bls12381Equal, 1<<5, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Add", smartcontract.InteropInterfaceType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("y", smartcontract.InteropInterfaceType))
	md = NewMethodAndPrice(c.bls12381Add, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Mul", smartcontract.InteropInterfaceType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("mul", smartcontract.ByteArrayType),
		manifest.NewParameter("neg", smartcontract.BoolType))
	md = NewMethodAndPrice(c.bls12381Mul, 1<<21, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381Pairing", smartcontract.InteropInterfaceType,
		manifest.NewParameter("g1", smartcontract.InteropInterfaceType),
		manifest.NewParameter("g2", smartcontract.InteropInterfaceType))
	md = NewMethodAndPrice(c.bls12381Pairing, 1<<23, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = NewDescriptor("keccak256", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.keccak256, 1<<15, callflag.NoneFlag, config.HFCockatrice)
	c.AddMethod(md, desc)

	desc = NewDescriptor("verifyWithEd25519", smartcontract.BoolType,
		manifest.NewParameter("message", smartcontract.ByteArrayType),
		manifest.NewParameter("pubkey", smartcontract.ByteArrayType),
		manifest.NewParameter("signature", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.verifyWithEd25519, 1<<15, callflag.NoneFlag, config.HFEchidna)
	c.AddMethod(md, desc)

	desc = NewDescriptor("recoverSecp256K1", smartcontract.ByteArrayType,
		manifest.NewParameter("messageHash", smartcontract.ByteArrayType),
		manifest.NewParameter("signature", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.recoverSecp256K1, 1<<15, callflag.NoneFlag, config.HFEchidna)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381SerializeEthereum", smartcontract.ByteArrayType,
		manifest.NewParameter("g", smartcontract.InteropInterfaceType))
	md = NewMethodAndPrice(c.bls12381SerializeEth, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeEthereum", smartcontract.InteropInterfaceType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeEth, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381SerializeList", smartcontract.ByteArrayType,
		manifest.NewParameter("points", smartcontract.ArrayType))
	md = NewMethodAndPrice(c.bls12381SerializeList, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381SerializeEthereumList", smartcontract.ByteArrayType,
		manifest.NewParameter("points", smartcontract.ArrayType))
	md = NewMethodAndPrice(c.bls12381SerializeEthList, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeList", smartcontract.ArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeList, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeEthereumList", smartcontract.ArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeEthList, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeG1ScalarPairs", smartcontract.ArrayType,
		manifest.NewParameter("pairs", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeG1ScalarPairs, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeG2ScalarPairs", smartcontract.ArrayType,
		manifest.NewParameter("pairs", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeG2ScalarPairs, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeEthereumG1ScalarPairs", smartcontract.ArrayType,
		manifest.NewParameter("pairs", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeEthG1ScalarPairs, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381DeserializeEthereumG2ScalarPairs", smartcontract.ArrayType,
		manifest.NewParameter("pairs", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bls12381DeserializeEthG2ScalarPairs, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381MultiExp", smartcontract.InteropInterfaceType,
		manifest.NewParameter("pairs", smartcontract.ArrayType))
	md = NewMethodAndPrice(c.bls12381MultiExp, 1<<23, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bls12381PairingList", smartcontract.BoolType,
		manifest.NewParameter("points", smartcontract.ArrayType))
	md = NewMethodAndPrice(c.bls12381PairingList, 1<<23, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)
	return c
}

func (c *Crypto) sha256(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	bs, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	return stackitem.NewByteArray(hash.Sha256(bs).BytesBE())
}

func (c *Crypto) ripemd160(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	bs, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	return stackitem.NewByteArray(hash.RipeMD160(bs).BytesBE())
}

func (c *Crypto) murmur32(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	bs, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	seed := toUint32(args[1])
	h := murmur3.SeedSum32(seed, bs)
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, h)
	return stackitem.NewByteArray(result)
}

func (c *Crypto) verifyWithECDsaPreCockatrice(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	return verifyWithECDsaGeneric(args, false)
}

func (c *Crypto) verifyWithECDsa(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	return verifyWithECDsaGeneric(args, true)
}

func verifyWithECDsaGeneric(args []stackitem.Item, allowKeccak bool) stackitem.Item {
	msg, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid message stackitem: %w", err))
	}
	pubkey, err := args[1].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid pubkey stackitem: %w", err))
	}
	signature, err := args[2].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid signature stackitem: %w", err))
	}
	curve, hasher, err := curveHasherFromStackitem(args[3], allowKeccak)
	if err != nil {
		panic(fmt.Errorf("invalid curveHash stackitem: %w", err))
	}
	hashToCheck := hasher(msg)
	pkey, err := keys.NewPublicKeyFromBytes(pubkey, curve)
	if err != nil {
		panic(fmt.Errorf("failed to decode pubkey: %w", err))
	}
	res := pkey.Verify(signature, hashToCheck.BytesBE())
	return stackitem.NewBool(res)
}

func curveHasherFromStackitem(si stackitem.Item, allowKeccak bool) (elliptic.Curve, HashFunc, error) {
	curve, err := si.TryInteger()
	if err != nil {
		return nil, nil, err
	}
	if !curve.IsInt64() {
		return nil, nil, errors.New("not an int64")
	}
	c := curve.Int64()
	switch c {
	case int64(Secp256k1Sha256):
		return secp256k1.S256(), hash.Sha256, nil
	case int64(Secp256r1Sha256):
		return elliptic.P256(), hash.Sha256, nil
	case int64(Secp256k1Keccak256):
		if !allowKeccak {
			return nil, nil, fmt.Errorf("%w: keccak hash", errors.ErrUnsupported)
		}
		return secp256k1.S256(), Keccak256, nil
	case int64(Secp256r1Keccak256):
		if !allowKeccak {
			return nil, nil, fmt.Errorf("%w: keccak hash", errors.ErrUnsupported)
		}
		return elliptic.P256(), Keccak256, nil
	default:
		return nil, nil, fmt.Errorf("%w: unknown curve/hash", errors.ErrUnsupported)
	}
}

func (c *Crypto) verifyWithEd25519(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	msg, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid message stackitem: %w", err))
	}
	pubkey, err := args[1].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid pubkey stackitem: %w", err))
	}
	signature, err := args[2].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid signature stackitem: %w", err))
	}
	if len(signature) != ed25519.SignatureSize {
		return stackitem.NewBool(false)
	}
	if len(pubkey) != ed25519.PublicKeySize {
		return stackitem.NewBool(false)
	}
	return stackitem.NewBool(ed25519.Verify(pubkey, msg, signature))
}

func (c *Crypto) recoverSecp256K1(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	msgH, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid message hash stackitem: %w", err))
	}
	if len(msgH) != 32 {
		return stackitem.Null{}
	}
	signature, err := args[1].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid signature stackitem: %w", err))
	}
	if len(signature) != 64 && len(signature) != 65 {
		return stackitem.Null{}
	}

	// ecdsa library (as most other general purpose crypto libraries) work only with
	// canonical representation, compact representation is a pure Ethereum feature (ref.
	// https://eips.ethereum.org/EIPS/eip-2098#specification). Also, ecdsa expects
	// leading signature byte to be a key recovery ID.
	canonical := koblitzSigToCanonical(signature)
	pub, _, err := ecdsa.RecoverCompact(canonical, msgH)
	if err != nil {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray(pub.SerializeCompressed())
}

// koblitzSigToCanonical converts compact Secp256K1 signature representation
// (https://eips.ethereum.org/EIPS/eip-2098#specification) to canonical
// form (https://www.secg.org/sec1-v2.pdf, section 4.1.6) and moves key recovery
// ID from the last byte to the first byte of the resulting signature (for both
// compact and non-compact forms), as it is required by ecdsa package.
func koblitzSigToCanonical(signature []byte) []byte {
	var res = make([]byte, 65) // don't modify the original slice.

	if len(signature) == 64 {
		// Convert from compact input format `r[32] || yParityAndS[32]` (where yParity
		// is fused into the top bit of s) to a canonical form `v[1] || r[32] || s[32]`.
		copy(res[1:], signature)
		s := res[33:]
		yParity := s[0] >> 7
		s[0] = s[0] & ((1 << 7) - 1)
		if yParity == 0 {
			res[0] = 27 // compact key recovery code for uncompressed public key inherited from Bitcoin.
		} else {
			res[0] = 28 // compact key recovery code for compressed public key inherited from Bitcoin.
		}
	} else {
		// Convert from `r[32] || s[32] || v[1]` form to a canonical `v[1] || r[32] || s[32]` form since
		// dcrd uses format with 'recovery id' v at the beginning.
		res[0] = signature[64]
		copy(res[1:], signature)

		// Denormalize key recovery code (if needed) to match the original Bitcoin form since
		// dcrd requires canonical format.
		if res[0] < 27 {
			res[0] += 27
		}
	}

	return res
}

func (c *Crypto) bls12381Serialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	val, ok := args[0].(*stackitem.Interop).Value().(blsPoint)
	if !ok {
		panic(errors.New("not a bls12381 point"))
	}
	return stackitem.NewByteArray(val.Bytes())
}

func (c *Crypto) bls12381Deserialize(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	buf, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid serialized bls12381 point: %w", err))
	}
	p := new(blsPoint)
	err = p.FromBytes(buf)
	if err != nil {
		panic(err)
	}
	return stackitem.NewInterop(*p)
}

func (c *Crypto) bls12381SerializeEth(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	val, ok := args[0].(*stackitem.Interop).Value().(blsPoint)
	if !ok {
		panic(errors.New("not a bls12381 point"))
	}
	return stackitem.NewByteArray(val.BytesEth())
}

func (c *Crypto) bls12381DeserializeEth(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	buf, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid serialized bls12381 point: %w", err))
	}
	p := new(blsPoint)
	err = p.FromBytesEth(buf)
	if err != nil {
		panic(err)
	}
	return stackitem.NewInterop(*p)
}

// bls12381SerializeList serializes a list of BLS12-381 points or (point, scalar) pairs.
func (c *Crypto) bls12381SerializeList(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	items := args[0].Value().([]stackitem.Item)
	if len(items) == 0 {
		panic("at least one element is required")
	}
	var res []byte
	if items[0].Type() == stackitem.InteropT {
		for _, v := range items {
			p, ok := v.(*stackitem.Interop).Value().(blsPoint)
			if !ok {
				panic("not a bls12381 point")
			}
			res = append(res, p.Bytes()...)
		}
		return stackitem.NewByteArray(res)
	}
	for _, si := range items {
		if si.Type() != stackitem.ArrayT && si.Type() != stackitem.StructT {
			panic("pair must be Array or Struct")
		}
		pair := si.Value().([]stackitem.Item)
		if len(pair) != 2 {
			panic("pair must contain point and scalar")
		}
		scalar, ok := pair[1].Value().(*big.Int)
		if !ok {
			panic("scalar must be bigint")
		}
		p, ok := pair[0].(*stackitem.Interop).Value().(blsPoint)
		if !ok {
			panic("not a bls12381 point")
		}
		res = append(res, p.Bytes()...)
		scalarBytes := make([]byte, fr.Bytes)
		bigint.ToPreallocatedBytes(scalar, scalarBytes)
		res = append(res, scalarBytes...)
	}
	return stackitem.NewByteArray(res)
}

func (c *Crypto) bls12381SerializeEthList(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	items := args[0].Value().([]stackitem.Item)
	if len(items) == 0 {
		panic("at least one element is required")
	}
	var res []byte
	if items[0].Type() == stackitem.InteropT {
		for _, v := range items {
			p, ok := v.(*stackitem.Interop).Value().(blsPoint)
			if !ok {
				panic("not a bls12381 point")
			}
			res = append(res, p.BytesEth()...)
		}
		return stackitem.NewByteArray(res)
	}
	for _, si := range items {
		if si.Type() != stackitem.ArrayT && si.Type() != stackitem.StructT {
			panic("pair must be Array or Struct")
		}
		pair := si.Value().([]stackitem.Item)
		if len(pair) != 2 {
			panic("pair must contain point and scalar")
		}
		scalar, ok := pair[1].Value().(*big.Int)
		if !ok {
			panic("scalar must be bigint")
		}
		p, ok := pair[0].(*stackitem.Interop).Value().(blsPoint)
		if !ok {
			panic("not a bls12381 point")
		}
		res = append(res, p.BytesEth()...)
		scalarBE := scalar.Bytes()
		scalarBytes := make([]byte, bls12ScalarLength)
		copy(scalarBytes[bls12ScalarLength-len(scalarBE):], scalarBE)
		res = append(res, scalarBytes...)
	}
	return stackitem.NewByteArray(res)
}

func (c *Crypto) bls12381DeserializeList(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12381.SizeOfG1AffineCompressed+bls12381.SizeOfG2AffineCompressed) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12381.SizeOfG1AffineCompressed+bls12381.SizeOfG2AffineCompressed))
	}
	var (
		i             int
		res           = make([]stackitem.Item, 0, l/(bls12381.SizeOfG1AffineCompressed+bls12381.SizeOfG2AffineCompressed))
		currPointSize = bls12381.SizeOfG1AffineCompressed
		nextPointSize = bls12381.SizeOfG2AffineCompressed
	)
	for i < l {
		p := new(blsPoint)
		if err := p.FromBytes(buf[i : i+currPointSize]); err != nil {
			panic(err)
		}
		res = append(res, stackitem.NewInterop(*p))
		i += currPointSize
		currPointSize, nextPointSize = nextPointSize, currPointSize
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381DeserializeEthList(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12G1EncodedLength+bls12G2EncodedLength) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12G1EncodedLength+bls12G2EncodedLength))
	}
	var (
		i             int
		res           = make([]stackitem.Item, 0, l/(bls12G1EncodedLength+bls12G2EncodedLength))
		currPointSize = bls12G1EncodedLength
		nextPointSize = bls12G2EncodedLength
	)
	for i < l {
		p := new(blsPoint)
		if err := p.FromBytesEth(buf[i : i+currPointSize]); err != nil {
			panic(err)
		}
		res = append(res, stackitem.NewInterop(*p))
		i += currPointSize
		currPointSize, nextPointSize = nextPointSize, currPointSize
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381DeserializeG1ScalarPairs(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12381.SizeOfG1AffineCompressed+fr.Bytes) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12381.SizeOfG1AffineCompressed+fr.Bytes))
	}
	res := make([]stackitem.Item, 0, l/(bls12381.SizeOfG1AffineCompressed+fr.Bytes))
	for i := 0; i < l; {
		p := new(blsPoint)
		if err := p.FromBytes(buf[i : i+bls12381.SizeOfG1AffineCompressed]); err != nil {
			panic(err)
		}
		i += bls12381.SizeOfG1AffineCompressed
		scalar, err := scalarFromBytes(buf[i:i+fr.Bytes], false)
		if err != nil {
			panic(fmt.Errorf("can't deserialize scalar: %w", err))
		}
		i += fr.Bytes
		res = append(res, stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(*p),
			stackitem.NewBigInteger(scalar),
		}))
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381DeserializeG2ScalarPairs(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12381.SizeOfG2AffineCompressed+fr.Bytes) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12381.SizeOfG2AffineCompressed+fr.Bytes))
	}
	res := make([]stackitem.Item, 0, l/(bls12381.SizeOfG2AffineCompressed+fr.Bytes))
	for i := 0; i < l; {
		p := new(blsPoint)
		if err := p.FromBytes(buf[i : i+bls12381.SizeOfG2AffineCompressed]); err != nil {
			panic(err)
		}
		i += bls12381.SizeOfG2AffineCompressed
		scalar, err := scalarFromBytes(buf[i:i+fr.Bytes], false)
		if err != nil {
			panic(fmt.Errorf("can't deserialize scalar: %w", err))
		}
		i += fr.Bytes
		res = append(res, stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(*p),
			stackitem.NewBigInteger(scalar),
		}))
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381DeserializeEthG1ScalarPairs(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12G1EncodedLength+bls12ScalarLength) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12G1EncodedLength+bls12ScalarLength))
	}
	res := make([]stackitem.Item, 0, l/(bls12G1EncodedLength+bls12ScalarLength))
	for i := 0; i < l; {
		p := new(blsPoint)
		if err := p.FromBytesEth(buf[i : i+bls12G1EncodedLength]); err != nil {
			panic(err)
		}
		i += bls12G1EncodedLength
		scalar := new(fr.Element).SetBigInt(new(big.Int).SetBytes(buf[i : i+bls12ScalarLength]))
		i += bls12ScalarLength
		res = append(res, stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(*p),
			stackitem.NewBigInteger(scalar.BigInt(new(big.Int))),
		}))
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381DeserializeEthG2ScalarPairs(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		buf = args[0].Value().([]byte)
		l   = len(buf)
	)
	if l == 0 {
		panic("deserializer requires at least one pair")
	}
	if l%(bls12G2EncodedLength+bls12ScalarLength) != 0 {
		panic(fmt.Sprintf("length must be a multiple of %d", bls12G2EncodedLength+bls12ScalarLength))
	}
	res := make([]stackitem.Item, 0, l/(bls12G2EncodedLength+bls12ScalarLength))
	for i := 0; i < l; {
		p := new(blsPoint)
		if err := p.FromBytesEth(buf[i : i+bls12G2EncodedLength]); err != nil {
			panic(err)
		}
		i += bls12G2EncodedLength
		scalar := new(fr.Element).SetBigInt(new(big.Int).SetBytes(buf[i : i+bls12ScalarLength]))
		i += bls12ScalarLength
		res = append(res, stackitem.NewArray([]stackitem.Item{
			stackitem.NewInterop(*p),
			stackitem.NewBigInteger(scalar.BigInt(new(big.Int))),
		}))
	}
	return stackitem.NewArray(res)
}

func (c *Crypto) bls12381Equal(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	a, okA := args[0].(*stackitem.Interop).Value().(blsPoint)
	b, okB := args[1].(*stackitem.Interop).Value().(blsPoint)
	if !okA || !okB {
		panic("some of the arguments are not a bls12381 point")
	}
	res, err := a.EqualsCheckType(b)
	if err != nil {
		panic(err)
	}
	return stackitem.NewBool(res)
}

func (c *Crypto) bls12381Add(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	a, okA := args[0].(*stackitem.Interop).Value().(blsPoint)
	b, okB := args[1].(*stackitem.Interop).Value().(blsPoint)
	if !okA || !okB {
		panic("some of the arguments are not a bls12381 point")
	}

	p, err := blsPointAdd(a, b)
	if err != nil {
		panic(err)
	}
	return stackitem.NewInterop(p)
}

func (c *Crypto) bls12381MultiExp(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	pairs := args[0].Value().([]stackitem.Item)
	if len(pairs) == 0 {
		panic("bls12381 multi exponent requires at least one pair")
	}
	if len(pairs) > bls12381MultiExpMaxPairs {
		panic(fmt.Sprintf("bls12381 multi exponent supports at most %d pairs", bls12381MultiExpMaxPairs))
	}
	var (
		accumulator blsPoint
		useG2       int
	)
	for _, si := range pairs {
		if si.Type() != stackitem.ArrayT && si.Type() != stackitem.StructT {
			panic("bls12381 multi exponent pair must be Array or Struct")
		}
		pair := si.Value().([]stackitem.Item)
		if len(pair) != 2 {
			panic("bls12381 multi exponent pair must contain point and scalar")
		}
		if pair[0].Type() != stackitem.InteropT {
			panic("bls12381 multi exponent requires interop points")
		}
		point, ok := pair[0].(*stackitem.Interop).Value().(blsPoint)
		if !ok {
			panic("bls12381 multi exponent interop must contain blsPoint")
		}
		switch point.point.(type) {
		case *bls12381.G1Jac, *bls12381.G1Affine:
			useG2 = ensureGroupType(useG2, -1)
		case *bls12381.G2Jac, *bls12381.G2Affine:
			useG2 = ensureGroupType(useG2, 1)
		default:
			panic("invalid bls12381 point type")
		}
		scalar, err := pair[1].TryInteger()
		if err != nil {
			panic(fmt.Errorf("invalid multiplier: %w", err))
		}
		if scalar.Sign() == 0 {
			continue
		}
		res, _ := blsPointMul(point, scalar)
		if accumulator.point == nil {
			accumulator = res
		} else {
			accumulator, _ = blsPointAdd(accumulator, res)
		}
	}
	if accumulator.point == nil {
		panic("bls12381 multi exponent requires at least one valid pair")
	}
	return stackitem.NewInterop(accumulator)
}

func ensureGroupType(useG2, isG2 int) int {
	if useG2*isG2 != -1 {
		return isG2
	}
	panic("can't mix groups")
}

func scalarFromBytes(bytes []byte, neg bool) (*big.Int, error) {
	alpha := new(fr.Element)
	if len(bytes) != fr.Bytes {
		return nil, fmt.Errorf("invalid multiplier: 32-bytes scalar is expected, got %d", len(bytes))
	}
	// The input bytes are in the LE form, so we can't use fr.Element.SetBytesCanonical as far
	// as it accepts BE. Confirmed by https://github.com/neo-project/neo/issues/2647#issuecomment-1129849870
	// and by https://github.com/nspcc-dev/neo-go/pull/3043#issuecomment-1733424840.
	v, err := fr.LittleEndian.Element((*[fr.Bytes]byte)(bytes))
	if err != nil {
		return nil, fmt.Errorf("invalid multiplier: failed to decode scalar: %w", err)
	}
	*alpha = v
	if neg {
		alpha.Neg(alpha)
	}
	return alpha.BigInt(new(big.Int)), nil
}

func (c *Crypto) bls12381Mul(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	a, okA := args[0].(*stackitem.Interop).Value().(blsPoint)
	if !okA {
		panic("multiplier is not a bls12381 point")
	}
	mulBytes, err := args[1].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid multiplier: %w", err))
	}
	neg, err := args[2].TryBool()
	if err != nil {
		panic(fmt.Errorf("invalid negative argument: %w", err))
	}
	alpha, err := scalarFromBytes(mulBytes, neg)
	if err != nil {
		panic(err)
	}

	p, err := blsPointMul(a, alpha)
	if err != nil {
		panic(err)
	}
	return stackitem.NewInterop(p)
}

func (c *Crypto) bls12381Pairing(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	a, okA := args[0].(*stackitem.Interop).Value().(blsPoint)
	b, okB := args[1].(*stackitem.Interop).Value().(blsPoint)
	if !okA || !okB {
		panic("some of the arguments are not a bls12381 point")
	}

	p, err := blsPointPairing(a, b)
	if err != nil {
		panic(err)
	}
	return stackitem.NewInterop(p)
}

func (c *Crypto) bls12381PairingList(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	var (
		points = args[0].Value().([]stackitem.Item)
		l      = len(points)
	)
	if l == 0 {
		panic("bls12381 pairing requires at least one pair")
	}
	if l%2 != 0 {
		panic("bls12381 pairing requires an even number of elements")
	}
	if l/2 > bls12381PairingMaxPairs {
		panic(fmt.Errorf("bls12381 pairing supports at most %d pairs", bls12381PairingMaxPairs))
	}
	accumulator := blsPoint{point: new(bls12381.GT).SetOne()}
	for i := 0; i < l; i += 2 {
		if points[i].Type() != stackitem.InteropT || points[i+1].Type() != stackitem.InteropT {
			panic("bls12381 pairing requires interop points")
		}
		a, okA := points[i].(*stackitem.Interop).Value().(blsPoint)
		b, okB := points[i+1].(*stackitem.Interop).Value().(blsPoint)
		if !okA || !okB {
			panic("interop must contain bls12381 point")
		}
		res, err := blsPointPairing(a, b)
		if err != nil {
			panic(fmt.Errorf("can't pair two points: %w", err))
		}
		accumulator, _ = blsPointAdd(accumulator, res)
	}
	return stackitem.NewBool(accumulator.point.(*bls12381.GT).IsOne())
}

func (c *Crypto) keccak256(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	bs, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	return stackitem.NewByteArray(Keccak256(bs).BytesBE())
}

// Metadata implements the Contract interface.
func (c *Crypto) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

// Initialize implements the Contract interface.
func (c *Crypto) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	return nil
}

// InitializeCache implements the Contract interface.
func (c *Crypto) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	return nil
}

// OnPersist implements the Contract interface.
func (c *Crypto) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements the Contract interface.
func (c *Crypto) PostPersist(ic *interop.Context) error {
	return nil
}

// ActiveIn implements the Contract interface.
func (c *Crypto) ActiveIn() *config.Hardfork {
	return nil
}

// Keccak256 hashes the incoming byte slice using the
// keccak256 algorithm.
func Keccak256(data []byte) util.Uint256 {
	var hash util.Uint256
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(data)

	hasher.Sum(hash[:0])
	return hash
}
