package native

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"sort"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

	// p2pSigExtensionsEnabled defines whether the P2P signature extensions logic is relevant.
	p2pSigExtensionsEnabled bool

	OracleService atomic.Value
	// NotaryService represents Notary node module.
	NotaryService atomic.Value
	// StateRootService represents StateRoot node module.
	StateRootService *stateroot.Module
}

type roleData struct {
	nodes  keys.PublicKeys
	addr   util.Uint160
	height uint32
}

type DesignationCache struct {
	rolesChangedFlag atomic.Value
	oracles          atomic.Value
	stateVals        atomic.Value
	neofsAlphabet    atomic.Value
	notaries         atomic.Value
}

const (
	designateContractID = -8

	// maxNodeCount is the maximum number of nodes to set the role for.
	maxNodeCount = 32

	// DesignationEventName is the name of a designation event.
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
	_ interop.Contract            = (*Designate)(nil)
	_ storage.NativeContractCache = (*DesignationCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *DesignationCache) Copy() storage.NativeContractCache {
	cp := &DesignationCache{}
	copyDesignationCache(c, cp)
	return cp
}

// Persist implements NativeContractCache interface.
func (c *DesignationCache) Persist(ps storage.NativeContractCache) (storage.NativeContractCache, error) {
	if ps == nil {
		ps = &DesignationCache{}
	}
	psCache, ok := ps.(*DesignationCache)
	if !ok {
		return nil, errors.New("not a Designation native cache")
	}
	copyDesignationCache(c, psCache)
	return psCache, nil
}

func copyDesignationCache(src, dst *DesignationCache) {
	dst.rolesChangedFlag.Store(src.rolesChangedFlag.Load())
	for _, r := range []noderoles.Role{noderoles.StateValidator, noderoles.Oracle, noderoles.NeoFSAlphabet, noderoles.P2PNotary} {
		data := getCachedRoleData(src, r)
		if data != nil {
			var v = &roleData{}
			*v = *data
			switch r {
			case noderoles.StateValidator:
				dst.stateVals.Store(v)
			case noderoles.Oracle:
				dst.oracles.Store(v)
			case noderoles.NeoFSAlphabet:
				dst.neofsAlphabet.Store(v)
			case noderoles.P2PNotary:
				dst.notaries.Store(v)
			}
		}
	}
}

func (s *Designate) isValidRole(r noderoles.Role) bool {
	return r == noderoles.Oracle || r == noderoles.StateValidator ||
		r == noderoles.NeoFSAlphabet || (s.p2pSigExtensionsEnabled && r == noderoles.P2PNotary)
}

func newDesignate(p2pSigExtensionsEnabled bool) *Designate {
	s := &Designate{ContractMD: *interop.NewContractMD(nativenames.Designation, designateContractID)}
	s.p2pSigExtensionsEnabled = p2pSigExtensionsEnabled
	defer s.UpdateHash()

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

	s.AddEvent(DesignationEventName,
		manifest.NewParameter("Role", smartcontract.IntegerType),
		manifest.NewParameter("BlockIndex", smartcontract.IntegerType))

	return s
}

// Initialize initializes Oracle contract.
func (s *Designate) Initialize(ic *interop.Context) error {
	cache := &DesignationCache{}
	cache.rolesChangedFlag.Store(true)
	ic.DAO.Store.SetCache(s.ID, cache)
	return nil
}

// OnPersist implements Contract interface.
func (s *Designate) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (s *Designate) PostPersist(ic *interop.Context) error {
	cache := ic.DAO.Store.GetRWCache(s.ID).(*DesignationCache)
	if !rolesChanged(cache) {
		return nil
	}

	if err := s.updateCachedRoleData(&cache.oracles, ic.DAO, noderoles.Oracle); err != nil {
		return err
	}
	if err := s.updateCachedRoleData(&cache.stateVals, ic.DAO, noderoles.StateValidator); err != nil {
		return err
	}
	if err := s.updateCachedRoleData(&cache.neofsAlphabet, ic.DAO, noderoles.NeoFSAlphabet); err != nil {
		return err
	}
	if s.p2pSigExtensionsEnabled {
		if err := s.updateCachedRoleData(&cache.notaries, ic.DAO, noderoles.P2PNotary); err != nil {
			return err
		}
	}

	cache.rolesChangedFlag.Store(false)
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

func rolesChanged(cache *DesignationCache) bool {
	rc := cache.rolesChangedFlag.Load()
	return rc == nil || rc.(bool)
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

func (s *Designate) updateCachedRoleData(v *atomic.Value, d *dao.Simple, r noderoles.Role) error {
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
	case noderoles.Oracle:
		if orc, _ := s.OracleService.Load().(services.Oracle); orc != nil {
			orc.UpdateOracleNodes(nodeKeys.Copy())
		}
	case noderoles.P2PNotary:
		if ntr, _ := s.NotaryService.Load().(services.Notary); ntr != nil {
			ntr.UpdateNotaryNodes(nodeKeys.Copy())
		}
	case noderoles.StateValidator:
		if s.StateRootService != nil {
			s.StateRootService.UpdateStateValidators(height, nodeKeys.Copy())
		}
	}
	return nil
}

func getCachedRoleData(cache *DesignationCache, r noderoles.Role) *roleData {
	var val interface{}
	switch r {
	case noderoles.Oracle:
		val = cache.oracles.Load()
	case noderoles.StateValidator:
		val = cache.stateVals.Load()
	case noderoles.NeoFSAlphabet:
		val = cache.neofsAlphabet.Load()
	case noderoles.P2PNotary:
		val = cache.notaries.Load()
	}
	if val != nil {
		return val.(*roleData)
	}
	return nil
}

// GetLastDesignatedHash returns last designated hash of a given role.
func (s *Designate) GetLastDesignatedHash(d *dao.Simple, r noderoles.Role) (util.Uint160, error) {
	if !s.isValidRole(r) {
		return util.Uint160{}, ErrInvalidRole
	}
	cache := d.Store.GetROCache(s.ID).(*DesignationCache)
	if !rolesChanged(cache) {
		if val := getCachedRoleData(cache, r); val != nil {
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
func (s *Designate) GetDesignatedByRole(d *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error) {
	if !s.isValidRole(r) {
		return nil, 0, ErrInvalidRole
	}
	cache := d.Store.GetROCache(s.ID).(*DesignationCache)
	if !rolesChanged(cache) {
		if val := getCachedRoleData(cache, r); val != nil && val.height <= index {
			return val.nodes.Copy(), val.height, nil
		}
	}
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
	h := s.NEO.GetCommitteeAddress(ic.DAO)
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
	nl := NodeList(pubs)
	ic.DAO.Store.GetRWCache(s.ID).(*DesignationCache).rolesChangedFlag.Store(true)
	err := putConvertibleToDAO(s.ID, ic.DAO, key, &nl)
	if err != nil {
		return err
	}

	ic.Notifications = append(ic.Notifications, state.NotificationEvent{
		ScriptHash: s.Hash,
		Name:       DesignationEventName,
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewBigInteger(big.NewInt(int64(r))),
			stackitem.NewBigInteger(big.NewInt(int64(ic.Block.Index))),
		}),
	})
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

// InitializeCache invalidates native Designate cache.
func (s *Designate) InitializeCache(d *dao.Simple) {
	cache := &DesignationCache{}
	cache.rolesChangedFlag.Store(true)
	d.Store.SetCache(s.ID, cache)
}
