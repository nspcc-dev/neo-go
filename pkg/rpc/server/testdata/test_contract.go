package testdata

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

const (
	totalSupply = 1000000
	decimals    = 2
)

func Init() bool {
	ctx := storage.GetContext()
	h := runtime.GetExecutingScriptHash()
	amount := totalSupply
	storage.Put(ctx, h, amount)
	runtime.Notify("Transfer", []byte{}, h, amount)
	return true
}

func Transfer(from, to interop.Hash160, amount int, data interface{}) bool {
	ctx := storage.GetContext()
	if len(from) != 20 {
		runtime.Log("invalid 'from' address")
		return false
	}
	if len(to) != 20 {
		runtime.Log("invalid 'to' address")
		return false
	}
	if amount < 0 {
		runtime.Log("invalid amount")
		return false
	}

	var fromBalance int
	val := storage.Get(ctx, from)
	if val != nil {
		fromBalance = val.(int)
	}
	if fromBalance < amount {
		runtime.Log("insufficient funds")
		return false
	}
	fromBalance -= amount
	storage.Put(ctx, from, fromBalance)

	var toBalance int
	val = storage.Get(ctx, to)
	if val != nil {
		toBalance = val.(int)
	}
	toBalance += amount
	storage.Put(ctx, to, toBalance)

	runtime.Notify("Transfer", from, to, amount)
	if management.GetContract(to) != nil {
		contract.Call(to, "onNEP17Payment", contract.All, from, amount, data)
	}
	return true
}

func BalanceOf(account interop.Hash160) int {
	ctx := storage.GetContext()
	if len(account) != 20 {
		panic("invalid address")
	}
	var amount int
	val := storage.Get(ctx, account)
	if val != nil {
		amount = val.(int)
	}
	return amount
}

func Symbol() string {
	return "RUB"
}

func Decimals() int {
	return decimals
}

func TotalSupply() int {
	return totalSupply
}

func PutValue(key []byte, value []byte) bool {
	ctx := storage.GetContext()
	storage.Put(ctx, key, value)
	return true
}
