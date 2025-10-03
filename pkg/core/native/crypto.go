package native

import (
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

const (
	FieldElementLength = 32
	G1EncodedLength    = 64
	PairInputLength    = 192
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

	desc = NewDescriptor("bn254Add", smartcontract.ByteArrayType,
		manifest.NewParameter("input", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bn254Add, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bn254Mul", smartcontract.ByteArrayType,
		manifest.NewParameter("input", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bn254Mul, 1<<19, callflag.NoneFlag, config.HFFaun)
	c.AddMethod(md, desc)

	desc = NewDescriptor("bn254Pairing", smartcontract.ByteArrayType,
		manifest.NewParameter("input", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(c.bn254Pairing, 1<<19, callflag.NoneFlag, config.HFFaun)
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

func (c *Crypto) bn254Add(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	input, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid input argument: %w", err))
	}
	if len(input) != 2*G1EncodedLength {
		panic(fmt.Errorf("invalid BN254 add input length: want %d, got %d", 2*G1EncodedLength, len(input)))
	}
	var res, first, second bn254.G1Affine
	if _, err = first.SetBytes(input[:G1EncodedLength]); err != nil {
		return stackitem.NewByteArray(make([]byte, G1EncodedLength))
	}
	if _, err = second.SetBytes(input[G1EncodedLength:]); err != nil {
		return stackitem.NewByteArray(make([]byte, G1EncodedLength))
	}
	res.Add(&first, &second)
	bytes := res.RawBytes()
	return stackitem.NewByteArray(bytes[:])
}

func (c *Crypto) bn254Mul(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	input, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid input argument: %w", err))
	}
	if len(input) != G1EncodedLength+FieldElementLength {
		panic(fmt.Errorf("invalid BN254 mul input length: want %d, got %d", G1EncodedLength+FieldElementLength, len(input)))
	}
	var (
		res, basePoint bn254.G1Affine
		scalar         fr.Element
	)
	if _, err = basePoint.SetBytes(input[:G1EncodedLength]); err != nil {
		return stackitem.NewByteArray(make([]byte, G1EncodedLength))
	}
	scalar.SetBytes(input[G1EncodedLength:])
	res.ScalarMultiplication(&basePoint, scalar.BigInt(new(big.Int)))
	bytes := res.RawBytes()
	return stackitem.NewByteArray(bytes[:])
}

func (c *Crypto) bn254Pairing(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	input, err := args[0].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid input argument: %w", err))
	}
	if len(input)%PairInputLength != 0 {
		panic(errors.New("invalid BN254 pairing input length"))
	}
	var (
		pairCount = len(input) / PairInputLength
		P         = make([]bn254.G1Affine, 0, pairCount)
		Q         = make([]bn254.G2Affine, 0, pairCount)
		g1        bn254.G1Affine
		g2        bn254.G2Affine
	)
	for i := range pairCount {
		offset := i * PairInputLength
		if _, err = g1.SetBytes(input[offset : offset+G1EncodedLength]); err != nil {
			return stackitem.NewByteArray(make([]byte, FieldElementLength))
		}
		if _, err = g2.SetBytes(input[offset+G1EncodedLength : offset+3*G1EncodedLength]); err != nil {
			return stackitem.NewByteArray(make([]byte, FieldElementLength))
		}
		if g1.IsInfinity() || g2.IsInfinity() {
			continue
		}
		P = append(P, g1)
		Q = append(Q, g2)
	}
	successWord := [FieldElementLength]byte{FieldElementLength - 1: 1}
	if len(P) == 0 {
		return stackitem.NewByteArray(successWord[:])
	}
	if ok, err := bn254.PairingCheck(P, Q); err != nil || !ok {
		return stackitem.NewByteArray(make([]byte, FieldElementLength))
	}
	return stackitem.NewByteArray(successWord[:])
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

func scalarFromBytes(bytes []byte, neg bool) (*fr.Element, error) {
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
	return alpha, nil
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
	alphaBi := new(big.Int)
	alpha.BigInt(alphaBi)

	p, err := blsPointMul(a, alphaBi)
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
