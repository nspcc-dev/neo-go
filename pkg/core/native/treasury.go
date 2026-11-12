package native

import (
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Treasury represents Treasury native contract.
type Treasury struct {
	interop.ContractMD
	NEO INEO
}

var _ interop.Contract = (*Treasury)(nil)

func newTreasury() *Treasury {
	t := &Treasury{
		ContractMD: *interop.NewContractMD(nativenames.Treasury, nativeids.Treasury, func(m *manifest.Manifest, hardfork config.Hardfork) {
			m.SupportedStandards = []string{manifest.NEP26StandardName, manifest.NEP27StandardName, manifest.NEP30StandardName}
		}),
	}
	defer t.BuildHFSpecificMD(t.ActiveIn())

	desc := NewDescriptor("verify", smartcontract.BoolType)
	md := NewMethodAndPrice(t.verify, 1<<5, callflag.ReadStates)
	t.AddMethod(md, desc)

	desc = NewDescriptor("onNEP11Payment", smartcontract.VoidType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
		manifest.NewParameter("tokenId", smartcontract.ByteArrayType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = NewMethodAndPrice(t.onNEP11PaymentV0, 1<<5, callflag.NoneFlag, config.HFDefault, config.HFHuyao)
	t.AddMethod(md, desc)

	md = NewMethodAndPrice(t.onNEP11PaymentV1, 1<<5, callflag.NoneFlag, config.HFHuyao)
	t.AddMethod(md, desc)

	desc = NewDescriptor("onNEP17Payment", smartcontract.VoidType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = NewMethodAndPrice(t.onNEP17PaymentV0, 1<<5, callflag.NoneFlag, config.HFDefault, config.HFHuyao)
	t.AddMethod(md, desc)

	md = NewMethodAndPrice(t.onNEP17PaymentV1, 1<<5, callflag.NoneFlag, config.HFHuyao)
	t.AddMethod(md, desc)

	return t
}

// OnPersist implements the [interop.Contract] interface.
func (t *Treasury) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements the [interop.Contract] interface.
func (t *Treasury) PostPersist(ic *interop.Context) error {
	return nil
}

// Metadata implements the [interop.Contract] interface.
func (t *Treasury) Metadata() *interop.ContractMD {
	return &t.ContractMD
}

// Initialize implements the [interop.Contract] interface.
func (t *Treasury) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	return nil
}

// InitializeCache implements the [interop.Contract] interface.
func (t *Treasury) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	return nil
}

// ActiveIn implements the [interop.Contract] interface.
func (t *Treasury) ActiveIn() *config.Hardfork {
	return new(config.HFFaun)
}

func (t *Treasury) verify(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBool(t.NEO.CheckCommittee(ic))
}

// onNEP11PaymentV0 is a pre-Huyao implementation of onNEP11Payment that rejects null sender.
func (t *Treasury) onNEP11PaymentV0(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return t.onNEP11PaymentGeneric(args, false)
}

// onNEP11PaymentV1 is an implementation of onNEP11Payment that accepts null sender of minted tokens.
func (t *Treasury) onNEP11PaymentV1(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return t.onNEP11PaymentGeneric(args, true)
}

func (t *Treasury) onNEP11PaymentGeneric(args []stackitem.Item, allowNullFrom bool) stackitem.Item {
	// Arguments must be accessed and parsed to match the reference behaviour, but no real action is performed.
	from := toUint160(args[0], allowNullFrom)
	amount := toBigInt(args[1])
	tokenId := toBytes(args[2])
	data := args[3]

	var _, _, _, _ = from, amount, tokenId, data
	return stackitem.Null{}
}

// onNEP17PaymentV0 is a pre-Huyao implementation of onNEP17Payment that rejects null sender.
func (t *Treasury) onNEP17PaymentV0(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return t.onNEP17PaymentGeneric(args, false)
}

// onNEP17PaymentV1 is an implementation of onNEP17Payment that accepts null sender of minted tokens.
func (t *Treasury) onNEP17PaymentV1(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return t.onNEP17PaymentGeneric(args, true)
}

func (t *Treasury) onNEP17PaymentGeneric(args []stackitem.Item, allowNullFrom bool) stackitem.Item {
	// Arguments must be accessed and parsed to match the reference behaviour, but no real action is performed.
	from := toUint160(args[0], allowNullFrom)
	amount := toBigInt(args[1])
	data := args[2]

	var _, _, _ = from, amount, data
	return stackitem.Null{}
}
