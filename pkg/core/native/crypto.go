package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
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
	Secp256k1Keccak256 NamedCurveHash = 24
	Secp256r1Keccak256 NamedCurveHash = 25
)

const cryptoContractID = -3

func newCrypto() *Crypto {
	c := &Crypto{ContractMD: *interop.NewContractMD(nativenames.CryptoLib, cryptoContractID)}
	defer c.BuildHFSpecificMD(c.ActiveIn())

	desc := newDescriptor("sha256", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md := newMethodAndPrice(c.sha256, 1<<15, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("ripemd160", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(c.ripemd160, 1<<15, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("murmur32", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType),
		manifest.NewParameter("seed", smartcontract.IntegerType))
	md = newMethodAndPrice(c.murmur32, 1<<13, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("verifyWithECDsa", smartcontract.BoolType,
		manifest.NewParameter("message", smartcontract.ByteArrayType),
		manifest.NewParameter("pubkey", smartcontract.ByteArrayType),
		manifest.NewParameter("signature", smartcontract.ByteArrayType),
		manifest.NewParameter("curveHash", smartcontract.IntegerType))
	md = newMethodAndPrice(c.verifyWithECDsa, 1<<15, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Serialize", smartcontract.ByteArrayType,
		manifest.NewParameter("g", smartcontract.InteropInterfaceType))
	md = newMethodAndPrice(c.bls12381Serialize, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Deserialize", smartcontract.InteropInterfaceType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(c.bls12381Deserialize, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Equal", smartcontract.BoolType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("y", smartcontract.InteropInterfaceType))
	md = newMethodAndPrice(c.bls12381Equal, 1<<5, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Add", smartcontract.InteropInterfaceType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("y", smartcontract.InteropInterfaceType))
	md = newMethodAndPrice(c.bls12381Add, 1<<19, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Mul", smartcontract.InteropInterfaceType,
		manifest.NewParameter("x", smartcontract.InteropInterfaceType),
		manifest.NewParameter("mul", smartcontract.ByteArrayType),
		manifest.NewParameter("neg", smartcontract.BoolType))
	md = newMethodAndPrice(c.bls12381Mul, 1<<21, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("bls12381Pairing", smartcontract.InteropInterfaceType,
		manifest.NewParameter("g1", smartcontract.InteropInterfaceType),
		manifest.NewParameter("g2", smartcontract.InteropInterfaceType))
	md = newMethodAndPrice(c.bls12381Pairing, 1<<23, callflag.NoneFlag)
	c.AddMethod(md, desc)

	desc = newDescriptor("keccak256", smartcontract.ByteArrayType,
		manifest.NewParameter("data", smartcontract.ByteArrayType))
	md = newMethodAndPrice(c.keccak256, 1<<15, callflag.NoneFlag, config.HFCockatrice)
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

func (c *Crypto) verifyWithECDsa(_ *interop.Context, args []stackitem.Item) stackitem.Item {
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
	curve, hasher, err := curveHasherFromStackitem(args[3])
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

func curveHasherFromStackitem(si stackitem.Item) (elliptic.Curve, HashFunc, error) {
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
		return secp256k1.S256(), Keccak256, nil
	case int64(Secp256r1Keccak256):
		return elliptic.P256(), Keccak256, nil
	default:
		return nil, nil, errors.New("unsupported curve/hash type")
	}
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
	if !(okA && okB) {
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
	if !(okA && okB) {
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
	if !(okA && okB) {
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
func (c *Crypto) InitializeCache(blockHeight uint32, d *dao.Simple) error {
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
