package interop

import (
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"go.uber.org/zap"
)

// Context represents context in which interops are executed.
type Context struct {
	Chain         blockchainer.Blockchainer
	Trigger       trigger.Type
	Block         *block.Block
	Tx            *transaction.Transaction
	DAO           *dao.Cached
	Notifications []state.NotificationEvent
	Log           *zap.Logger
}

// NewContext returns new interop context.
func NewContext(trigger trigger.Type, bc blockchainer.Blockchainer, d dao.DAO, block *block.Block, tx *transaction.Transaction, log *zap.Logger) *Context {
	dao := dao.NewCached(d)
	nes := make([]state.NotificationEvent, 0)
	return &Context{
		Chain:         bc,
		Trigger:       trigger,
		Block:         block,
		Tx:            tx,
		DAO:           dao,
		Notifications: nes,
		Log:           log,
	}
}

// Function binds function name, id with the function itself and price,
// it's supposed to be inited once for all interopContexts, so it doesn't use
// vm.InteropFuncPrice directly.
type Function struct {
	ID    uint32
	Name  string
	Func  func(*Context, *vm.VM) error
	Price int
}
