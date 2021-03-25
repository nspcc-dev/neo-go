package interop

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
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
	// DefaultBaseExecFee specifies default multiplier for opcode and syscall prices.
	DefaultBaseExecFee = 30
)

// Context represents context in which interops are executed.
type Context struct {
	Chain         blockchainer.Blockchainer
	Container     hash.Hashable
	Network       uint32
	Natives       []Contract
	Trigger       trigger.Type
	Block         *block.Block
	Tx            *transaction.Transaction
	DAO           *dao.Cached
	Notifications []state.NotificationEvent
	Log           *zap.Logger
	VM            *vm.VM
	Functions     [][]Function
	getContract   func(dao.DAO, util.Uint160) (*state.Contract, error)
}

// NewContext returns new interop context.
func NewContext(trigger trigger.Type, bc blockchainer.Blockchainer, d dao.DAO,
	getContract func(dao.DAO, util.Uint160) (*state.Contract, error), natives []Contract,
	block *block.Block, tx *transaction.Transaction, log *zap.Logger) *Context {
	dao := dao.NewCached(d)
	nes := make([]state.NotificationEvent, 0)
	return &Context{
		Chain:         bc,
		Network:       uint32(bc.GetConfig().Magic),
		Natives:       natives,
		Trigger:       trigger,
		Block:         block,
		Tx:            tx,
		DAO:           dao,
		Notifications: nes,
		Log:           log,
		// Functions is a slice of slices of interops sorted by ID.
		Functions:   [][]Function{},
		getContract: getContract,
	}
}

// Function binds function name, id with the function itself and price,
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

// MethodAndPrice is a native-contract method descriptor.
type MethodAndPrice struct {
	Func          Method
	MD            *manifest.Method
	CPUFee        int64
	StorageFee    int64
	SyscallOffset int
	RequiredFlags callflag.CallFlag
}

// Contract is an interface for all native contracts.
type Contract interface {
	Initialize(*Context) error
	Metadata() *ContractMD
	OnPersist(*Context) error
	PostPersist(*Context) error
}

// ContractMD represents native contract instance.
type ContractMD struct {
	state.NativeContract
	Name    string
	Methods []MethodAndPrice
}

// NewContractMD returns Contract with the specified list of methods.
func NewContractMD(name string, id int32) *ContractMD {
	c := &ContractMD{Name: name}

	c.ID = id

	// NEF is now stored in contract state and affects state dump.
	// Therefore values are taken from C# node.
	c.NEF.Header.Compiler = "neo-core-v3.0"
	c.NEF.Header.Magic = nef.Magic
	c.Hash = state.CreateContractHash(util.Uint160{}, 0, c.Name)
	c.Manifest = *manifest.DefaultManifest(name)

	return c
}

// UpdateHash creates native contract script and updates hash.
func (c *ContractMD) UpdateHash() {
	w := io.NewBufBinWriter()
	for i := range c.Methods {
		offset := w.Len()
		c.Methods[i].MD.Offset = offset
		c.Manifest.ABI.Methods[i].Offset = offset
		emit.Int(w.BinWriter, 0)
		c.Methods[i].SyscallOffset = w.Len()
		emit.Syscall(w.BinWriter, interopnames.SystemContractCallNative)
		emit.Opcodes(w.BinWriter, opcode.RET)
	}
	if w.Err != nil {
		panic(fmt.Errorf("can't create native contract script: %w", w.Err))
	}

	c.NEF.Script = w.Bytes()
	c.NEF.Checksum = c.NEF.CalculateChecksum()
}

// AddMethod adds new method to a native contract.
func (c *ContractMD) AddMethod(md *MethodAndPrice, desc *manifest.Method) {
	md.MD = desc
	desc.Safe = md.RequiredFlags&(callflag.All^callflag.ReadOnly) == 0

	index := sort.Search(len(c.Manifest.ABI.Methods), func(i int) bool {
		md := c.Manifest.ABI.Methods[i]
		if md.Name != desc.Name {
			return md.Name >= desc.Name
		}
		return len(md.Parameters) > len(desc.Parameters)
	})
	c.Manifest.ABI.Methods = append(c.Manifest.ABI.Methods, manifest.Method{})
	copy(c.Manifest.ABI.Methods[index+1:], c.Manifest.ABI.Methods[index:])
	c.Manifest.ABI.Methods[index] = *desc

	// Cache follows the same order.
	c.Methods = append(c.Methods, MethodAndPrice{})
	copy(c.Methods[index+1:], c.Methods[index:])
	c.Methods[index] = *md
}

// GetMethodByOffset returns with the provided offset.
// Offset is offset of `System.Contract.CallNative` syscall.
func (c *ContractMD) GetMethodByOffset(offset int) (MethodAndPrice, bool) {
	for k := range c.Methods {
		if c.Methods[k].SyscallOffset == offset {
			return c.Methods[k], true
		}
	}
	return MethodAndPrice{}, false
}

// GetMethod returns method `name` with specified number of parameters.
func (c *ContractMD) GetMethod(name string, paramCount int) (MethodAndPrice, bool) {
	index := sort.Search(len(c.Methods), func(i int) bool {
		md := c.Methods[i]
		res := strings.Compare(name, md.MD.Name)
		switch res {
		case -1, 1:
			return res == -1
		default:
			return paramCount <= len(md.MD.Parameters)
		}
	})
	if index < len(c.Methods) {
		md := c.Methods[index]
		if md.MD.Name == name && (paramCount == -1 || len(md.MD.Parameters) == paramCount) {
			return md, true
		}
	}
	return MethodAndPrice{}, false
}

// AddEvent adds new event to a native contract.
func (c *ContractMD) AddEvent(name string, ps ...manifest.Parameter) {
	c.Manifest.ABI.Events = append(c.Manifest.ABI.Events, manifest.Event{
		Name:       name,
		Parameters: ps,
	})
}

// IsActive returns true iff the contract was deployed by the specified height.
func (c *ContractMD) IsActive(height uint32) bool {
	history := c.UpdateHistory
	return len(history) != 0 && history[0] <= height
}

// Sort sorts interop functions by id.
func Sort(fs []Function) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}

// GetContract returns contract by its hash in current interop context.
func (ic *Context) GetContract(hash util.Uint160) (*state.Contract, error) {
	return ic.getContract(ic.DAO, hash)
}

// GetFunction returns metadata for interop with the specified id.
func (ic *Context) GetFunction(id uint32) *Function {
	for _, slice := range ic.Functions {
		n := sort.Search(len(slice), func(i int) bool {
			return slice[i].ID >= id
		})
		if n < len(slice) && slice[n].ID == id {
			return &slice[n]
		}
	}
	return nil
}

// BaseExecFee represents factor to multiply syscall prices with.
func (ic *Context) BaseExecFee() int64 {
	if ic.Chain == nil || (ic.Block != nil && ic.Block.Index == 0) {
		return DefaultBaseExecFee
	}
	return ic.Chain.GetPolicer().GetBaseExecFee()
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

// SpawnVM spawns new VM with the specified gas limit and set context.VM field.
func (ic *Context) SpawnVM() *vm.VM {
	v := vm.NewWithTrigger(ic.Trigger)
	v.GasLimit = -1
	v.SyscallHandler = ic.SyscallHandler
	ic.VM = v
	return v
}
