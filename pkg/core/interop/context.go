package interop

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

const (
	// DefaultBaseExecFee specifies the default multiplier for opcode and syscall prices.
	DefaultBaseExecFee = 30
	// ContextNonceDataLen is a length of [Context.NonceData] in bytes.
	ContextNonceDataLen = 16
)

// Ledger is the interface to Blockchain required for Context functionality.
type Ledger interface {
	BlockHeight() uint32
	CurrentBlockHash() util.Uint256
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetConfig() config.Blockchain
	GetHeaderHash(uint32) util.Uint256
}

// Context represents context in which interops are executed.
type Context struct {
	Chain            Ledger
	Container        hash.Hashable
	Network          uint32
	Hardforks        map[string]uint32
	Natives          []Contract
	Trigger          trigger.Type
	Block            *block.Block
	NonceData        [ContextNonceDataLen]byte
	Tx               *transaction.Transaction
	DAO              *dao.Simple
	Notifications    []state.NotificationEvent
	Log              *zap.Logger
	VM               *vm.VM
	Functions        []Function
	Invocations      map[util.Uint160]int
	cancelFuncs      []context.CancelFunc
	getContract      func(*dao.Simple, util.Uint160) (*state.Contract, error)
	baseExecFee      int64
	baseStorageFee   int64
	loadToken        func(ic *Context, id int32) error
	GetRandomCounter uint32
	signers          []transaction.Signer
}

// NewContext returns new interop context.
func NewContext(trigger trigger.Type, bc Ledger, d *dao.Simple, baseExecFee, baseStorageFee int64,
	getContract func(*dao.Simple, util.Uint160) (*state.Contract, error), natives []Contract,
	loadTokenFunc func(ic *Context, id int32) error,
	block *block.Block, tx *transaction.Transaction, log *zap.Logger) *Context {
	dao := d.GetPrivate()
	cfg := bc.GetConfig().ProtocolConfiguration
	return &Context{
		Chain:          bc,
		Network:        uint32(cfg.Magic),
		Hardforks:      cfg.Hardforks,
		Natives:        natives,
		Trigger:        trigger,
		Block:          block,
		Tx:             tx,
		DAO:            dao,
		Log:            log,
		Invocations:    make(map[util.Uint160]int),
		getContract:    getContract,
		baseExecFee:    baseExecFee,
		baseStorageFee: baseStorageFee,
		loadToken:      loadTokenFunc,
	}
}

// InitNonceData initializes nonce to be used in `GetRandom` calculations.
func (ic *Context) InitNonceData() {
	if tx, ok := ic.Container.(*transaction.Transaction); ok {
		ic.NonceData = [ContextNonceDataLen]byte(tx.Hash().BytesBE())
	}
	if ic.Block != nil {
		nonce := ic.Block.Nonce
		nonce ^= binary.LittleEndian.Uint64(ic.NonceData[:])
		binary.LittleEndian.PutUint64(ic.NonceData[:], nonce)
	}
}

// UseSigners allows overriding signers used in this context.
func (ic *Context) UseSigners(s []transaction.Signer) {
	ic.signers = s
}

// Signers returns signers witnessing the current execution context.
func (ic *Context) Signers() []transaction.Signer {
	if ic.signers != nil {
		return ic.signers
	}
	if ic.Tx != nil {
		return ic.Tx.Signers
	}
	return nil
}

// Function binds function name, id with the function itself and the price,
// it's supposed to be inited once for all interopContexts, so it doesn't use
// vm.InteropFuncPrice directly.
type Function struct {
	ID   uint32
	Name string
	Func func(*Context) error
	// ParamCount is a number of function parameters.
	ParamCount int
	Price      int64
	// RequiredFlags is a set of flags which must be set during script invocations.
	// Default value is NoneFlag i.e. no flags are required.
	RequiredFlags callflag.CallFlag
}

// Method is a signature for a native method.
type Method = func(ic *Context, args []stackitem.Item) stackitem.Item

// MethodAndPrice is a generic hardfork-independent native contract method descriptor.
type MethodAndPrice struct {
	HFSpecificMethodAndPrice
	ActiveFrom *config.Hardfork
	ActiveTill *config.Hardfork
}

// HFSpecificMethodAndPrice is a hardfork-specific native contract method descriptor.
type HFSpecificMethodAndPrice struct {
	Func          Method
	MD            *manifest.Method
	CPUFee        int64
	StorageFee    int64
	SyscallOffset int
	RequiredFlags callflag.CallFlag
}

// Event is a generic hardfork-independent native contract event descriptor.
type Event struct {
	HFSpecificEvent
	ActiveFrom *config.Hardfork
	ActiveTill *config.Hardfork
}

// HFSpecificEvent is a hardfork-specific native contract event descriptor.
type HFSpecificEvent struct {
	MD *manifest.Event
}

// Contract is an interface for all native contracts.
type Contract interface {
	// Initialize performs native contract initialization on contract deploy or update.
	// Active hardfork is passed as the second argument.
	Initialize(*Context, *config.Hardfork, *HFSpecificContractMD) error
	// ActiveIn returns the hardfork native contract is active starting from or nil in case
	// it's always active.
	ActiveIn() *config.Hardfork
	// InitializeCache aimed to initialize contract's cache when the contract has
	// been deployed, but in-memory cached data were lost due to the node reset.
	// It should be called each time after node restart iff the contract was
	// deployed and no Initialize method was called.
	InitializeCache(blockHeight uint32, d *dao.Simple) error
	// Metadata returns generic native contract metadata.
	Metadata() *ContractMD
	OnPersist(*Context) error
	PostPersist(*Context) error
}

// ContractMD represents a generic hardfork-independent native contract instance.
type ContractMD struct {
	ID   int32
	Hash util.Uint160
	Name string
	// methods is a generic set of contract methods with activation hardforks. Any HF-dependent part of included methods
	// (offsets, in particular) must not be used, there's a mdCache field for that.
	methods []MethodAndPrice
	// events is a generic set of contract events with activation hardforks. Any HF-dependent part of events must not be
	// used, there's a mdCache field for that.
	events []Event
	// ActiveHFs is a map of hardforks that contract should react to. Contract update should be called for active
	// hardforks. Note, that unlike the C# implementation, this map doesn't include contract's activation hardfork.
	// This map is being initialized on contract creation and used as a read-only, hence, not protected
	// by mutex.
	ActiveHFs map[config.Hardfork]struct{}

	// mdCache contains hardfork-specific ready-to-use contract descriptors. This cache is initialized in the native
	// contracts constructors, and acts as read-only during the whole node lifetime, thus not protected by mutex.
	mdCache map[config.Hardfork]*HFSpecificContractMD

	// onManifestConstruction is a callback for manifest finalization.
	onManifestConstruction func(*manifest.Manifest)
}

// HFSpecificContractMD is a hardfork-specific native contract descriptor.
type HFSpecificContractMD struct {
	state.ContractBase
	Methods []HFSpecificMethodAndPrice
	Events  []HFSpecificEvent
}

// NewContractMD returns Contract with the specified fields set. onManifestConstruction callback every time
// after hardfork-specific manifest creation and aimed to finalize the manifest.
func NewContractMD(name string, id int32, onManifestConstruction ...func(*manifest.Manifest)) *ContractMD {
	c := &ContractMD{Name: name}
	if len(onManifestConstruction) != 0 {
		c.onManifestConstruction = onManifestConstruction[0]
	}

	c.ID = id
	c.Hash = state.CreateNativeContractHash(c.Name)
	c.ActiveHFs = make(map[config.Hardfork]struct{})
	c.mdCache = make(map[config.Hardfork]*HFSpecificContractMD)

	return c
}

// HFSpecificContractMD returns hardfork-specific native contract metadata, i.e. with methods, events and script
// corresponding to the specified hardfork. If hardfork is not specified, then default metadata will be returned
// (methods, events and script that are always active). Calling this method for hardforks older than the contract
// activation hardfork is a no-op.
func (c *ContractMD) HFSpecificContractMD(hf *config.Hardfork) *HFSpecificContractMD {
	var key config.Hardfork
	if hf != nil {
		key = *hf
	}
	md, ok := c.mdCache[key]
	if !ok {
		panic(fmt.Errorf("native contract descriptor cache is not initialized: contract %s, hardfork %s", c.Hash.StringLE(), key))
	}
	if md == nil {
		panic(fmt.Errorf("native contract descriptor cache is nil: contract %s, hardfork %s", c.Hash.StringLE(), key))
	}
	return md
}

// BuildHFSpecificMD generates and caches contract's descriptor for every known hardfork.
func (c *ContractMD) BuildHFSpecificMD(activeIn *config.Hardfork) {
	var start config.Hardfork
	if activeIn != nil {
		start = *activeIn
	}

	for _, hf := range append([]config.Hardfork{config.HFDefault}, config.Hardforks...) {
		switch {
		case hf.Cmp(start) < 0:
			continue
		case hf.Cmp(start) == 0:
			c.buildHFSpecificMD(hf)
		default:
			if _, ok := c.ActiveHFs[hf]; !ok {
				// Intentionally omit HFSpecificContractMD structure copying since mdCache is read-only.
				c.mdCache[hf] = c.mdCache[hf.Prev()]
				continue
			}
			c.buildHFSpecificMD(hf)
		}
	}
}

// buildHFSpecificMD builds hardfork-specific contract descriptor that includes methods and events active starting from
// the specified hardfork or older. It also updates cache with the received value.
func (c *ContractMD) buildHFSpecificMD(hf config.Hardfork) {
	var (
		abiMethods = make([]manifest.Method, 0, len(c.methods))
		methods    = make([]HFSpecificMethodAndPrice, 0, len(c.methods))
		abiEvents  = make([]manifest.Event, 0, len(c.events))
		events     = make([]HFSpecificEvent, 0, len(c.events))
	)
	w := io.NewBufBinWriter()
	for i := range c.methods {
		m := c.methods[i]
		if (m.ActiveFrom != nil && (*m.ActiveFrom).Cmp(hf) > 0) ||
			(m.ActiveTill != nil && (*m.ActiveTill).Cmp(hf) <= 0) {
			continue
		}

		// Perform method descriptor copy to support independent HF-based offset update.
		md := *m.MD
		m.MD = &md
		m.MD.Offset = w.Len()

		emit.Int(w.BinWriter, 0)
		m.SyscallOffset = w.Len()
		emit.Syscall(w.BinWriter, interopnames.SystemContractCallNative)
		emit.Opcodes(w.BinWriter, opcode.RET)

		abiMethods = append(abiMethods, *m.MD)
		methods = append(methods, m.HFSpecificMethodAndPrice)
	}
	if w.Err != nil {
		panic(fmt.Errorf("can't create native contract script: %w", w.Err))
	}
	for i := range c.events {
		e := c.events[i]
		if (e.ActiveFrom != nil && (*e.ActiveFrom).Cmp(hf) > 0) ||
			(e.ActiveTill != nil && (*e.ActiveTill).Cmp(hf) <= 0) {
			continue
		}

		abiEvents = append(abiEvents, *e.MD)
		events = append(events, e.HFSpecificEvent)
	}

	// NEF is now stored in the contract state and affects state dump.
	// Therefore, values are taken from C# node.
	nf := nef.File{
		Header: nef.Header{
			Magic:    nef.Magic,
			Compiler: "neo-core-v3.0",
		},
		Tokens: []nef.MethodToken{}, // avoid `nil` result during JSON marshalling,
		Script: w.Bytes(),
	}
	nf.Checksum = nf.CalculateChecksum()
	m := manifest.DefaultManifest(c.Name)
	m.ABI.Methods = abiMethods
	m.ABI.Events = abiEvents
	if c.onManifestConstruction != nil {
		c.onManifestConstruction(m)
	}
	md := &HFSpecificContractMD{
		ContractBase: state.ContractBase{
			ID:       c.ID,
			Hash:     c.Hash,
			NEF:      nf,
			Manifest: *m,
		},
		Methods: methods,
		Events:  events,
	}

	c.mdCache[hf] = md
}

// AddMethod adds a new method to a native contract.
func (c *ContractMD) AddMethod(md *MethodAndPrice, desc *manifest.Method) {
	md.MD = desc
	desc.Safe = md.RequiredFlags&(callflag.All^callflag.ReadOnly) == 0

	index, _ := slices.BinarySearchFunc(c.methods, *md, func(e, t MethodAndPrice) int {
		return cmp.Or(
			cmp.Compare(e.MD.Name, t.MD.Name),
			cmp.Compare(len(e.MD.Parameters), len(t.MD.Parameters)),
		)
	})
	c.methods = slices.Insert(c.methods, index, *md)

	if md.ActiveFrom != nil {
		c.ActiveHFs[*md.ActiveFrom] = struct{}{}
	}
	if md.ActiveTill != nil {
		c.ActiveHFs[*md.ActiveTill] = struct{}{}
	}
}

// GetMethodByOffset returns method with the provided offset.
// Offset is offset of `System.Contract.CallNative` syscall.
func (c *HFSpecificContractMD) GetMethodByOffset(offset int) (HFSpecificMethodAndPrice, bool) {
	for k := range c.Methods {
		if c.Methods[k].SyscallOffset == offset {
			return c.Methods[k], true
		}
	}
	return HFSpecificMethodAndPrice{}, false
}

// GetMethod returns method `name` with the specified number of parameters.
func (c *HFSpecificContractMD) GetMethod(name string, paramCount int) (HFSpecificMethodAndPrice, bool) {
	index, ok := slices.BinarySearchFunc(c.Methods, HFSpecificMethodAndPrice{}, func(a, _ HFSpecificMethodAndPrice) int {
		res := strings.Compare(a.MD.Name, name)
		if res != 0 {
			return res
		}
		return cmp.Compare(len(a.MD.Parameters), paramCount)
	})
	// Exact match is possible only for specific paramCount, but if we're
	// searching for _some_ method with this name (-1) we're taking the
	// first one.
	if ok || (index < len(c.Methods) && c.Methods[index].MD.Name == name && paramCount == -1) {
		return c.Methods[index], true
	}
	return HFSpecificMethodAndPrice{}, false
}

// AddEvent adds a new event to the native contract.
func (c *ContractMD) AddEvent(md Event) {
	c.events = append(c.events, md)

	if md.ActiveFrom != nil {
		c.ActiveHFs[*md.ActiveFrom] = struct{}{}
	}
	if md.ActiveTill != nil {
		c.ActiveHFs[*md.ActiveTill] = struct{}{}
	}
}

// Sort sorts interop functions by id.
func Sort(fs []Function) {
	slices.SortFunc(fs, func(a, b Function) int { return cmp.Compare(a.ID, b.ID) })
}

// GetContract returns a contract by its hash in the current interop context.
func (ic *Context) GetContract(hash util.Uint160) (*state.Contract, error) {
	return ic.getContract(ic.DAO, hash)
}

// GetFunction returns metadata for interop with the specified id.
func (ic *Context) GetFunction(id uint32) *Function {
	n, ok := slices.BinarySearchFunc(ic.Functions, Function{}, func(a, _ Function) int {
		return cmp.Compare(a.ID, id)
	})
	if !ok {
		return nil
	}
	return &ic.Functions[n]
}

// BaseExecFee represents factor to multiply syscall prices with.
func (ic *Context) BaseExecFee() int64 {
	return ic.baseExecFee
}

// BaseStorageFee represents price for storing one byte of data in the contract storage.
func (ic *Context) BaseStorageFee() int64 {
	return ic.baseStorageFee
}

// LoadToken wraps externally provided load-token loading function providing it with context,
// this function can then be easily used by VM.
func (ic *Context) LoadToken(id int32) error {
	return ic.loadToken(ic, id)
}

// SyscallHandler handles syscall with id.
func (ic *Context) SyscallHandler(_ *vm.VM, id uint32) error {
	f := ic.GetFunction(id)
	if f == nil {
		return errors.New("syscall not found")
	}
	cf := ic.VM.Context().GetCallFlags()
	if !cf.Has(f.RequiredFlags) {
		return fmt.Errorf("missing call flags: %05b vs %05b", cf, f.RequiredFlags)
	}
	if !ic.VM.AddGas(f.Price * ic.BaseExecFee()) {
		return errors.New("insufficient amount of gas")
	}
	return f.Func(ic)
}

// SpawnVM spawns a new VM with the specified gas limit and set context.VM field.
func (ic *Context) SpawnVM() *vm.VM {
	v := vm.NewWithTrigger(ic.Trigger)
	ic.initVM(v)
	return v
}

func (ic *Context) initVM(v *vm.VM) {
	v.LoadToken = ic.LoadToken
	v.GasLimit = -1
	v.SyscallHandler = ic.SyscallHandler
	v.SetPriceGetter(ic.GetPrice)
	ic.VM = v
}

// ReuseVM resets given VM and allows to reuse it in the current context.
func (ic *Context) ReuseVM(v *vm.VM) {
	v.Reset(ic.Trigger)
	ic.initVM(v)
}

// RegisterCancelFunc adds the given function to the list of functions to be called after the VM
// finishes script execution.
func (ic *Context) RegisterCancelFunc(f context.CancelFunc) {
	if f != nil {
		ic.cancelFuncs = append(ic.cancelFuncs, f)
	}
}

// Finalize calls all registered cancel functions to release the occupied resources.
func (ic *Context) Finalize() {
	for _, f := range ic.cancelFuncs {
		f()
	}
	ic.cancelFuncs = nil
}

// Exec executes loaded VM script and calls registered finalizers to release the occupied resources.
func (ic *Context) Exec() error {
	defer ic.Finalize()
	return ic.VM.Run()
}

// BlockHeight returns the latest persisted and stored block height/index.
// Persisting block index is not taken into account. If Context's block is set,
// then BlockHeight calculations relies on persisting block index.
func (ic *Context) BlockHeight() uint32 {
	if ic.Block != nil {
		return ic.Block.Index - 1 // Persisting block is not yet stored.
	}
	return ic.Chain.BlockHeight()
}

// CurrentBlockHash returns current block hash got from Context's block if it's set.
func (ic *Context) CurrentBlockHash() util.Uint256 {
	if ic.Block != nil {
		return ic.Chain.GetHeaderHash(ic.Block.Index - 1) // Persisting block is not yet stored.
	}
	return ic.Chain.CurrentBlockHash()
}

// GetBlock returns block if it exists and available at the current Context's height.
func (ic *Context) GetBlock(hash util.Uint256) (*block.Block, error) {
	block, err := ic.Chain.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	if block.Index > ic.BlockHeight() { // persisting block is not reachable.
		return nil, storage.ErrKeyNotFound
	}
	return block, nil
}

// IsHardforkEnabled tells whether specified hard-fork enabled at the current context height.
func (ic *Context) IsHardforkEnabled(hf config.Hardfork) bool {
	height, ok := ic.Hardforks[hf.String()]
	if ok {
		return (ic.BlockHeight() + 1) >= height // persisting block should be taken into account.
	}
	// Completely rely on proper hardforks initialisation made by core.NewBlockchain.
	return false
}

// IsHardforkActivation denotes whether current block height is the height of
// specified hardfork activation.
func (ic *Context) IsHardforkActivation(hf config.Hardfork) bool {
	// Completely rely on proper hardforks initialisation made by core.NewBlockchain.
	height, ok := ic.Hardforks[hf.String()]
	return ok && ic.Block.Index == height
}

// AddNotification creates notification event and appends it to the notification list.
func (ic *Context) AddNotification(hash util.Uint160, name string, item *stackitem.Array) {
	ic.Notifications = append(ic.Notifications, state.NotificationEvent{
		ScriptHash: hash,
		Name:       name,
		Item:       item,
	})
}
