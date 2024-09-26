package native

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Designate represents a designation contract.
type Designate struct {
	interop.ContractMD
	NEO *NEO

	// initialNodeRoles defines a set of node roles that should be defined at the contract
	// deployment (initialization).
	initialNodeRoles map[noderoles.Role]keys.PublicKeys

	OracleService atomic.Value
	// NotaryService represents a Notary node module.
	NotaryService atomic.Value
	// StateRootService represents a StateRoot node module.
	StateRootService *stateroot.Module
}

type roleData struct {
	nodes  keys.PublicKeys
	addr   util.Uint160
	height uint32
}

type DesignationCache struct {
	// rolesChangedFlag shows whether any of designated nodes were changed within the current block.
	// It is used to notify dependant services about updated node roles during PostPersist.
	rolesChangedFlag bool
	oracles          roleData
	stateVals        roleData
	neofsAlphabet    roleData
	notaries         roleData
}

const (
	designateContractID = -8

	// maxNodeCount is the maximum number of nodes to set the role for.
	maxNodeCount = 32

	// DesignationEventName is the name of the designation event.
	DesignationEventName = "Designation"
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

var (
	_ interop.Contract        = (*Designate)(nil)
	_ dao.NativeContractCache = (*DesignationCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *DesignationCache) Copy() dao.NativeContractCache {
	cp := &DesignationCache{}
	copyDesignationCache(c, cp)
	return cp
}

func copyDesignationCache(src, dst *DesignationCache) {
	*dst = *src
}

func (s *Designate) isValidRole(r noderoles.Role) bool {
	return r == noderoles.Oracle || r == noderoles.StateValidator ||
		r == noderoles.NeoFSAlphabet || r == noderoles.P2PNotary
}

func newDesignate(initialNodeRoles map[noderoles.Role]keys.PublicKeys) *Designate {
	s := &Designate{ContractMD: *interop.NewContractMD(nativenames.Designation, designateContractID)}
	defer s.BuildHFSpecificMD(s.ActiveIn())

	s.initialNodeRoles = initialNodeRoles

	desc := newDescriptor("getDesignatedByRole", smartcontract.ArrayType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("index", smartcontract.IntegerType))
	md := newMethodAndPrice(s.getDesignatedByRole, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = newDescriptor("designateAsRole", smartcontract.VoidType,
		manifest.NewParameter("role", smartcontract.IntegerType),
		manifest.NewParameter("nodes", smartcontract.ArrayType))
	md = newMethodAndPrice(s.designateAsRole, 1<<15, callflag.States|callflag.AllowNotify)
	s.AddMethod(md, desc)

	eDesc := newEventDescriptor(DesignationEventName,
		manifest.NewParameter("Role", smartcontract.IntegerType),
		manifest.NewParameter("BlockIndex", smartcontract.IntegerType))
	eMD := newEvent(eDesc)
	s.AddEvent(eMD)

	return s
}

// Initialize initializes Designation contract. It is called once at native Management's OnPersist
// at the genesis block, and we can't properly fill the cache at this point, as there are no roles
// data in the storage.
func (s *Designate) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	if hf != s.ActiveIn() {
		return nil
	}

	cache := &DesignationCache{}
	ic.DAO.SetCache(s.ID, cache)

	if len(s.initialNodeRoles) != 0 {
		for _, r := range noderoles.Roles {
			pubs, ok := s.initialNodeRoles[r]
			if !ok {
				continue
			}
			err := s.DesignateAsRole(ic, r, pubs)
			if err != nil {
				return fmt.Errorf("failed to initialize Designation role data for role %s: %w", r, err)
			}
		}
	}
	return nil
}

// InitializeCache fills native Designate cache from DAO. It is called at non-zero height, thus
// we can fetch the roles data right from the storage.
func (s *Designate) InitializeCache(blockHeight uint32, d *dao.Simple) error {
	cache := &DesignationCache{}
	roles := []noderoles.Role{noderoles.Oracle, noderoles.NeoFSAlphabet, noderoles.StateValidator, noderoles.P2PNotary}
	for _, r := range roles {
		err := s.updateCachedRoleData(cache, d, r)
		if err != nil {
			return fmt.Errorf("failed to get nodes from storage for %d role: %w", r, err)
		}
	}
	d.SetCache(s.ID, cache)
	return nil
}

// OnPersist implements the Contract interface.
func (s *Designate) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements the Contract interface.
func (s *Designate) PostPersist(ic *interop.Context) error {
	cache := ic.DAO.GetRWCache(s.ID).(*DesignationCache)
	if !cache.rolesChangedFlag {
		return nil
	}

	s.notifyRoleChanged(&cache.oracles, noderoles.Oracle)
	s.notifyRoleChanged(&cache.stateVals, noderoles.StateValidator)
	s.notifyRoleChanged(&cache.neofsAlphabet, noderoles.NeoFSAlphabet)
	s.notifyRoleChanged(&cache.notaries, noderoles.P2PNotary)

	cache.rolesChangedFlag = false
	return nil
}

// Metadata returns contract metadata.
func (s *Designate) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

// ActiveIn implements the Contract interface.
func (s *Designate) ActiveIn() *config.Hardfork {
	return nil
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
	if index > uint64(ic.BlockHeight()+1) { // persisting block should be taken into account.
		panic(ErrInvalidIndex)
	}
	pubs, _, err := s.GetDesignatedByRole(ic.DAO, r, uint32(index))
	if err != nil {
		panic(err)
	}
	return pubsToArray(pubs)
}

func (s *Designate) hashFromNodes(r noderoles.Role, nodes keys.PublicKeys) util.Uint160 {
	if len(nodes) == 0 {
		return util.Uint160{}
	}
	var script []byte
	switch r {
	case noderoles.P2PNotary:
		script, _ = smartcontract.CreateMultiSigRedeemScript(1, nodes.Copy())
	default:
		script, _ = smartcontract.CreateDefaultMultiSigRedeemScript(nodes.Copy())
	}
	return hash.Hash160(script)
}

// updateCachedRoleData fetches the most recent role data from the storage and
// updates the given cache.
func (s *Designate) updateCachedRoleData(cache *DesignationCache, d *dao.Simple, r noderoles.Role) error {
	var v *roleData
	switch r {
	case noderoles.Oracle:
		v = &cache.oracles
	case noderoles.StateValidator:
		v = &cache.stateVals
	case noderoles.NeoFSAlphabet:
		v = &cache.neofsAlphabet
	case noderoles.P2PNotary:
		v = &cache.notaries
	}
	nodeKeys, height, err := s.getDesignatedByRoleFromStorage(d, r, math.MaxUint32)
	if err != nil {
		return err
	}
	v.nodes = nodeKeys
	v.addr = s.hashFromNodes(r, nodeKeys)
	v.height = height
	cache.rolesChangedFlag = true
	return nil
}

func (s *Designate) notifyRoleChanged(v *roleData, r noderoles.Role) {
	switch r {
	case noderoles.Oracle:
		if orc, _ := s.OracleService.Load().(*OracleService); orc != nil && *orc != nil {
			(*orc).UpdateOracleNodes(v.nodes.Copy())
		}
	case noderoles.P2PNotary:
		if ntr, _ := s.NotaryService.Load().(*NotaryService); ntr != nil && *ntr != nil {
			(*ntr).UpdateNotaryNodes(v.nodes.Copy())
		}
	case noderoles.StateValidator:
		if s.StateRootService != nil {
			s.StateRootService.UpdateStateValidators(v.height, v.nodes.Copy())
		}
	default:
	}
}

func getCachedRoleData(cache *DesignationCache, r noderoles.Role) *roleData {
	switch r {
	case noderoles.Oracle:
		return &cache.oracles
	case noderoles.StateValidator:
		return &cache.stateVals
	case noderoles.NeoFSAlphabet:
		return &cache.neofsAlphabet
	case noderoles.P2PNotary:
		return &cache.notaries
	}
	return nil
}

// GetLastDesignatedHash returns the last designated hash of the given role.
func (s *Designate) GetLastDesignatedHash(d *dao.Simple, r noderoles.Role) (util.Uint160, error) {
	if !s.isValidRole(r) {
		return util.Uint160{}, ErrInvalidRole
	}
	cache := d.GetROCache(s.ID).(*DesignationCache)
	if val := getCachedRoleData(cache, r); val != nil {
		return val.addr, nil
	}
	return util.Uint160{}, nil
}

// GetDesignatedByRole returns nodes for role r.
func (s *Designate) GetDesignatedByRole(d *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error) {
	if !s.isValidRole(r) {
		return nil, 0, ErrInvalidRole
	}
	cache := d.GetROCache(s.ID).(*DesignationCache)
	if val := getCachedRoleData(cache, r); val != nil {
		if val.height <= index {
			return val.nodes.Copy(), val.height, nil
		}
	} else {
		// Cache is always valid, thus if there's no cache then there's no designated nodes for this role.
		return nil, 0, nil
	}
	// Cache stores only latest designated nodes, so if the old info is requested, then we still need
	// to search in the storage.
	return s.getDesignatedByRoleFromStorage(d, r, index)
}

// getDesignatedByRoleFromStorage returns nodes for role r from the storage.
func (s *Designate) getDesignatedByRoleFromStorage(d *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error) {
	var (
		ns        NodeList
		bestIndex uint32
		resVal    []byte
		start     = make([]byte, 4)
	)

	binary.BigEndian.PutUint32(start, index)
	d.Seek(s.ID, storage.SeekRange{
		Prefix:    []byte{byte(r)},
		Start:     start,
		Backwards: true,
	}, func(k, v []byte) bool {
		bestIndex = binary.BigEndian.Uint32(k) // If len(k) < 4 the DB is broken and it deserves a panic.
		resVal = v
		// Take just the latest item, it's the one we need.
		return false
	})
	if resVal != nil {
		err := stackitem.DeserializeConvertible(resVal, &ns)
		if err != nil {
			return nil, 0, err
		}
	}
	return keys.PublicKeys(ns), bestIndex, nil
}

func (s *Designate) designateAsRole(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	r, ok := s.getRole(args[0])
	if !ok {
		panic(ErrInvalidRole)
	}
	var ns NodeList
	if err := ns.FromStackItem(args[1]); err != nil {
		panic(err)
	}

	err := s.DesignateAsRole(ic, r, keys.PublicKeys(ns))
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// DesignateAsRole sets nodes for role r.
func (s *Designate) DesignateAsRole(ic *interop.Context, r noderoles.Role, pubs keys.PublicKeys) error {
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

	if ic.Trigger != trigger.OnPersist {
		h := s.NEO.GetCommitteeAddress(ic.DAO)
		if ok, err := runtime.CheckHashedWitness(ic, h); err != nil || !ok {
			return ErrInvalidWitness
		}
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
	slices.SortFunc(pubs, (*keys.PublicKey).Cmp)
	nl := NodeList(pubs)

	err := putConvertibleToDAO(s.ID, ic.DAO, key, &nl)
	if err != nil {
		return err
	}

	cache := ic.DAO.GetRWCache(s.ID).(*DesignationCache)
	err = s.updateCachedRoleData(cache, ic.DAO, r)
	if err != nil {
		return fmt.Errorf("failed to update Designation role data cache: %w", err)
	}

	ic.AddNotification(s.Hash, DesignationEventName, stackitem.NewArray([]stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(int64(r))),
		stackitem.NewBigInteger(big.NewInt(int64(ic.Block.Index))),
	}))
	return nil
}

func (s *Designate) getRole(item stackitem.Item) (noderoles.Role, bool) {
	bi, err := item.TryInteger()
	if err != nil {
		return 0, false
	}
	if !bi.IsUint64() {
		return 0, false
	}
	u := bi.Uint64()
	return noderoles.Role(u), u <= math.MaxUint8 && s.isValidRole(noderoles.Role(u))
}
