package native

import (
	"errors"
	"math"
	"sort"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Designate represents designation contract.
type Designate struct {
	interop.ContractMD
	NEO *NEO

	rolesChangedFlag atomic.Value
	oracleNodes      atomic.Value
	oracleHash       atomic.Value
}

const (
	designateContractID = -5
	designateName       = "Designation"
)

// Role represents type of participant.
type Role byte

// Role enumeration.
const (
	RoleStateValidator Role = 4
	RoleOracle         Role = 8
)

// Various errors.
var (
	ErrInvalidRole   = errors.New("invalid role")
	ErrEmptyNodeList = errors.New("node list is empty")
)

func isValidRole(r Role) bool {
	return r == RoleOracle || r == RoleStateValidator
}

func newDesignate() *Designate {
	s := &Designate{ContractMD: *interop.NewContractMD(designateName)}
	s.ContractID = designateContractID
	s.Manifest.Features = smartcontract.HasStorage

	desc := newDescriptor("getDesignatedByRole", smartcontract.ArrayType,
		manifest.NewParameter("role", smartcontract.IntegerType))
	md := newMethodAndPrice(s.getDesignatedByRole, 0, smartcontract.AllowStates)
	s.AddMethod(md, desc, false)

	desc = newDescriptor("designateAsRole", smartcontract.VoidType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("nodes", smartcontract.ArrayType))
	md = newMethodAndPrice(s.designateAsRole, 0, smartcontract.AllowModifyStates)
	s.AddMethod(md, desc, false)

	return s
}

// Initialize initializes Oracle contract.
func (s *Designate) Initialize(ic *interop.Context) error {
	roles := []Role{RoleStateValidator, RoleOracle}
	for _, r := range roles {
		si := &state.StorageItem{Value: new(NodeList).Bytes()}
		if err := ic.DAO.PutStorageItem(s.ContractID, []byte{byte(r)}, si); err != nil {
			return err
		}
	}

	s.oracleNodes.Store(keys.PublicKeys(nil))
	s.rolesChangedFlag.Store(true)
	return nil
}

// OnPersistEnd updates cached values if they've been changed.
func (s *Designate) OnPersistEnd(d dao.DAO) error {
	if !s.rolesChanged() {
		return nil
	}

	var ns NodeList
	err := getSerializableFromDAO(s.ContractID, d, []byte{byte(RoleOracle)}, &ns)
	if err != nil {
		return err
	}

	s.oracleNodes.Store(keys.PublicKeys(ns))
	script, _ := smartcontract.CreateMajorityMultiSigRedeemScript(keys.PublicKeys(ns).Copy())
	s.oracleHash.Store(hash.Hash160(script))
	s.rolesChangedFlag.Store(false)
	return nil
}

// Metadata returns contract metadata.
func (s *Designate) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

func (s *Designate) getDesignatedByRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	pubs, err := s.GetDesignatedByRole(ic.DAO, r)
	if err != nil {
		panic(err)
	}
	return pubsToArray(pubs)
}

func (s *Designate) rolesChanged() bool {
	rc := s.rolesChangedFlag.Load()
	return rc == nil || rc.(bool)
}

// GetDesignatedByRole returns nodes for role r.
func (s *Designate) GetDesignatedByRole(d dao.DAO, r Role) (keys.PublicKeys, error) {
	if !isValidRole(r) {
		return nil, ErrInvalidRole
	}
	if r == RoleOracle && !s.rolesChanged() {
		return s.oracleNodes.Load().(keys.PublicKeys), nil
	}
	var ns NodeList
	err := getSerializableFromDAO(s.ContractID, d, []byte{byte(r)}, &ns)
	return keys.PublicKeys(ns), err
}

func (s *Designate) designateAsRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	var ns NodeList
	if err := ns.fromStackItem(args[1]); err != nil {
		panic(err)
	}

	err := s.DesignateAsRole(ic, r, keys.PublicKeys(ns))
	if err != nil {
		panic(err)
	}
	return pubsToArray(keys.PublicKeys(ns))
}

// DesignateAsRole sets nodes for role r.
func (s *Designate) DesignateAsRole(ic *interop.Context, r Role, pubs keys.PublicKeys) error {
	if len(pubs) == 0 {
		return ErrEmptyNodeList
	}
	if !isValidRole(r) {
		return ErrInvalidRole
	}
	h := s.NEO.GetCommitteeAddress()
	if ok, err := runtime.CheckHashedWitness(ic, h); err != nil || !ok {
		return ErrInvalidWitness
	}

	sort.Sort(pubs)
	s.rolesChangedFlag.Store(true)
	si := &state.StorageItem{Value: NodeList(pubs).Bytes()}
	return ic.DAO.PutStorageItem(s.ContractID, []byte{byte(r)}, si)
}

func getRole(item stackitem.Item) (Role, bool) {
	bi, err := item.TryInteger()
	if err != nil {
		return 0, false
	}
	if !bi.IsUint64() {
		return 0, false
	}
	u := bi.Uint64()
	return Role(u), u <= math.MaxUint8 && isValidRole(Role(u))
}
