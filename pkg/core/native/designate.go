package native

import (
	"encoding/binary"
	"errors"
	"math"
	"sort"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Designate represents designation contract.
type Designate struct {
	interop.ContractMD
	NEO *NEO

	rolesChangedFlag atomic.Value
	oracles          atomic.Value
	stateVals        atomic.Value
	neofsAlphabet    atomic.Value
	notaries         atomic.Value

	// p2pSigExtensionsEnabled defines whether the P2P signature extensions logic is relevant.
	p2pSigExtensionsEnabled bool

	OracleService atomic.Value
	// NotaryService represents Notary node module.
	NotaryService atomic.Value
}

type roleData struct {
	nodes  keys.PublicKeys
	addr   util.Uint160
	height uint32
}

const (
	designateContractID = -6

	// maxNodeCount is the maximum number of nodes to set the role for.
	maxNodeCount = 32
)

// Role represents type of participant.
type Role byte

// Role enumeration.
const (
	RoleStateValidator Role = 4
	RoleOracle         Role = 8
	RoleNeoFSAlphabet  Role = 16
	RoleP2PNotary      Role = 128
)

// Various errors.
var (
	ErrAlreadyDesignated = errors.New("already designated given role at current block")
	ErrEmptyNodeList     = errors.New("node list is empty")
	ErrInvalidIndex      = errors.New("invalid index")
	ErrInvalidRole       = errors.New("invalid role")
	ErrLargeNodeList     = errors.New("node list is too large")
	ErrNoBlock           = errors.New("no persisting block in the context")
)

func (s *Designate) isValidRole(r Role) bool {
	return r == RoleOracle || r == RoleStateValidator ||
		r == RoleNeoFSAlphabet || (s.p2pSigExtensionsEnabled && r == RoleP2PNotary)
}

func newDesignate(p2pSigExtensionsEnabled bool) *Designate {
	s := &Designate{ContractMD: *interop.NewContractMD(nativenames.Designation, designateContractID)}
	s.p2pSigExtensionsEnabled = p2pSigExtensionsEnabled
	defer s.UpdateHash()

	desc := newDescriptor("getDesignatedByRole", smartcontract.ArrayType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("index", smartcontract.IntegerType))
	md := newMethodAndPrice(s.getDesignatedByRole, 1000000, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = newDescriptor("designateAsRole", smartcontract.VoidType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("nodes", smartcontract.ArrayType))
	md = newMethodAndPrice(s.designateAsRole, 0, callflag.States)
	s.AddMethod(md, desc)

	return s
}

// Initialize initializes Oracle contract.
func (s *Designate) Initialize(ic *interop.Context) error {
	return nil
}

// OnPersist implements Contract interface.
func (s *Designate) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (s *Designate) PostPersist(ic *interop.Context) error {
	if !s.rolesChanged() {
		return nil
	}

	if err := s.updateCachedRoleData(&s.oracles, ic.DAO, RoleOracle); err != nil {
		return err
	}
	if err := s.updateCachedRoleData(&s.stateVals, ic.DAO, RoleStateValidator); err != nil {
		return err
	}
	if err := s.updateCachedRoleData(&s.neofsAlphabet, ic.DAO, RoleNeoFSAlphabet); err != nil {
		return err
	}
	if s.p2pSigExtensionsEnabled {
		if err := s.updateCachedRoleData(&s.notaries, ic.DAO, RoleP2PNotary); err != nil {
			return err
		}
	}

	s.rolesChangedFlag.Store(false)
	return nil
}

// Metadata returns contract metadata.
func (s *Designate) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

func (s *Designate) getDesignatedByRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := s.getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	ind, err := args[1].TryInteger()
	if err != nil || !ind.IsUint64() {
		panic(ErrInvalidIndex)
	}
	index := ind.Uint64()
	if index > uint64(ic.Chain.BlockHeight()+1) {
		panic(ErrInvalidIndex)
	}
	pubs, _, err := s.GetDesignatedByRole(ic.DAO, r, uint32(index))
	if err != nil {
		panic(err)
	}
	return pubsToArray(pubs)
}

func (s *Designate) rolesChanged() bool {
	rc := s.rolesChangedFlag.Load()
	return rc == nil || rc.(bool)
}

func (s *Designate) hashFromNodes(r Role, nodes keys.PublicKeys) util.Uint160 {
	if len(nodes) == 0 {
		return util.Uint160{}
	}
	var script []byte
	switch r {
	case RoleOracle, RoleNeoFSAlphabet:
		script, _ = smartcontract.CreateDefaultMultiSigRedeemScript(nodes.Copy())
	case RoleP2PNotary:
		script, _ = smartcontract.CreateMultiSigRedeemScript(1, nodes.Copy())
	default:
		script, _ = smartcontract.CreateMajorityMultiSigRedeemScript(nodes.Copy())
	}
	return hash.Hash160(script)
}

func (s *Designate) updateCachedRoleData(v *atomic.Value, d dao.DAO, r Role) error {
	nodeKeys, height, err := s.GetDesignatedByRole(d, r, math.MaxUint32)
	if err != nil {
		return err
	}
	v.Store(&roleData{
		nodes:  nodeKeys,
		addr:   s.hashFromNodes(r, nodeKeys),
		height: height,
	})
	switch r {
	case RoleOracle:
		if orc, _ := s.OracleService.Load().(services.Oracle); orc != nil {
			orc.UpdateOracleNodes(nodeKeys.Copy())
		}
	case RoleP2PNotary:
		if ntr, _ := s.NotaryService.Load().(services.Notary); ntr != nil {
			ntr.UpdateNotaryNodes(nodeKeys.Copy())
		}
	}
	return nil
}

func (s *Designate) getCachedRoleData(r Role) *roleData {
	var val interface{}
	switch r {
	case RoleOracle:
		val = s.oracles.Load()
	case RoleStateValidator:
		val = s.stateVals.Load()
	case RoleNeoFSAlphabet:
		val = s.neofsAlphabet.Load()
	case RoleP2PNotary:
		val = s.notaries.Load()
	}
	if val != nil {
		return val.(*roleData)
	}
	return nil
}

// GetLastDesignatedHash returns last designated hash of a given role.
func (s *Designate) GetLastDesignatedHash(d dao.DAO, r Role) (util.Uint160, error) {
	if !s.isValidRole(r) {
		return util.Uint160{}, ErrInvalidRole
	}
	if !s.rolesChanged() {
		if val := s.getCachedRoleData(r); val != nil {
			return val.addr, nil
		}
	}
	nodes, _, err := s.GetDesignatedByRole(d, r, math.MaxUint32)
	if err != nil {
		return util.Uint160{}, err
	}
	// We only have hashing defined for oracles now.
	return s.hashFromNodes(r, nodes), nil
}

// GetDesignatedByRole returns nodes for role r.
func (s *Designate) GetDesignatedByRole(d dao.DAO, r Role, index uint32) (keys.PublicKeys, uint32, error) {
	if !s.isValidRole(r) {
		return nil, 0, ErrInvalidRole
	}
	if !s.rolesChanged() {
		if val := s.getCachedRoleData(r); val != nil && val.height <= index {
			return val.nodes.Copy(), val.height, nil
		}
	}
	kvs, err := d.GetStorageItemsWithPrefix(s.ID, []byte{byte(r)})
	if err != nil {
		return nil, 0, err
	}
	var ns NodeList
	var bestIndex uint32
	var resSi state.StorageItem
	for k, si := range kvs {
		if len(k) < 4 {
			continue
		}
		siInd := binary.BigEndian.Uint32([]byte(k))
		if (resSi == nil || siInd > bestIndex) && siInd <= index {
			bestIndex = siInd
			resSi = si
		}
	}
	if resSi != nil {
		reader := io.NewBinReaderFromBuf(resSi)
		ns.DecodeBinary(reader)
		if reader.Err != nil {
			return nil, 0, reader.Err
		}
	}
	return keys.PublicKeys(ns), bestIndex, err
}

func (s *Designate) designateAsRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := s.getRole(args[0])
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
	return stackitem.Null{}
}

// DesignateAsRole sets nodes for role r.
func (s *Designate) DesignateAsRole(ic *interop.Context, r Role, pubs keys.PublicKeys) error {
	length := len(pubs)
	if length == 0 {
		return ErrEmptyNodeList
	}
	if length > maxNodeCount {
		return ErrLargeNodeList
	}
	if !s.isValidRole(r) {
		return ErrInvalidRole
	}
	h := s.NEO.GetCommitteeAddress()
	if ok, err := runtime.CheckHashedWitness(ic, h); err != nil || !ok {
		return ErrInvalidWitness
	}
	if ic.Block == nil {
		return ErrNoBlock
	}
	var key = make([]byte, 5)
	key[0] = byte(r)
	binary.BigEndian.PutUint32(key[1:], ic.Block.Index+1)

	si := ic.DAO.GetStorageItem(s.ID, key)
	if si != nil {
		return ErrAlreadyDesignated
	}
	sort.Sort(pubs)
	s.rolesChangedFlag.Store(true)
	return ic.DAO.PutStorageItem(s.ID, key, NodeList(pubs).Bytes())
}

func (s *Designate) getRole(item stackitem.Item) (Role, bool) {
	bi, err := item.TryInteger()
	if err != nil {
		return 0, false
	}
	if !bi.IsUint64() {
		return 0, false
	}
	u := bi.Uint64()
	return Role(u), u <= math.MaxUint8 && s.isValidRole(Role(u))
}
