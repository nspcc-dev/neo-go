package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/twmb/murmur3"
)

// Crypto represents CryptoLib contract.
type Crypto struct {
	interop.ContractMD
}

// NamedCurve identifies named elliptic curves.
type NamedCurve byte

// Various named elliptic curves.
const (
	Secp256k1 NamedCurve = 22
	Secp256r1 NamedCurve = 23
)

const cryptoContractID = -3

func newCrypto() *Crypto {
	c := &Crypto{ContractMD: *interop.NewContractMD(nativenames.CryptoLib, cryptoContractID)}
	defer c.UpdateHash()

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
		manifest.NewParameter("curve", smartcontract.IntegerType))
	md = newMethodAndPrice(c.verifyWithECDsa, 1<<15, callflag.NoneFlag)
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
	hashToCheck := hash.Sha256(msg)
	pubkey, err := args[1].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid pubkey stackitem: %w", err))
	}
	signature, err := args[2].TryBytes()
	if err != nil {
		panic(fmt.Errorf("invalid signature stackitem: %w", err))
	}
	curve, err := curveFromStackitem(args[3])
	if err != nil {
		panic(fmt.Errorf("invalid curve stackitem: %w", err))
	}
	pkey, err := keys.NewPublicKeyFromBytes(pubkey, curve)
	if err != nil {
		panic(fmt.Errorf("failed to decode pubkey: %w", err))
	}
	res := pkey.Verify(signature, hashToCheck.BytesBE())
	return stackitem.NewBool(res)
}

func curveFromStackitem(si stackitem.Item) (elliptic.Curve, error) {
	curve, err := si.TryInteger()
	if err != nil {
		return nil, err
	}
	if !curve.IsUint64() {
		return nil, errors.New("not an int64")
	}
	c := curve.Uint64()
	switch c {
	case uint64(Secp256k1):
		return secp256k1.S256(), nil
	case uint64(Secp256r1):
		return elliptic.P256(), nil
	default:
		return nil, errors.New("unsupported curve type")
	}
}

// Metadata implements the Contract interface.
func (c *Crypto) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

// Initialize implements the Contract interface.
func (c *Crypto) Initialize(ic *interop.Context) error {
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
