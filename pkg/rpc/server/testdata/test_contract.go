package testdata

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

const (
	totalSupply = 1000000
	decimals    = 2
)

func Main(operation string, args []interface{}) interface{} {
	runtime.Notify("contract call", operation, args)
	switch operation {
	case "Put":
		ctx := storage.GetContext()
		storage.Put(ctx, args[0].([]byte), args[1].([]byte))
		return true
	case "totalSupply":
		return totalSupply
	case "decimals":
		return decimals
	case "name":
		return "Rubl"
	case "symbol":
		return "RUB"
	case "balanceOf":
		ctx := storage.GetContext()
		addr := args[0].([]byte)
		if len(addr) != 20 {
			runtime.Log("invalid address")
			return false
		}
		var amount int
		val := storage.Get(ctx, addr)
		if val != nil {
			amount = val.(int)
		}
		runtime.Notify("balanceOf", addr, amount)
		return amount
	case "transfer":
		ctx := storage.GetContext()
		from := args[0].([]byte)
		if len(from) != 20 {
			runtime.Log("invalid 'from' address")
			return false
		}
		to := args[1].([]byte)
		if len(to) != 20 {
			runtime.Log("invalid 'to' address")
			return false
		}
		amount := args[2].(int)
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

		runtime.Notify("transfer", from, to, amount)

		return true
	case "init":
		ctx := storage.GetContext()
		h := runtime.GetExecutingScriptHash()
		amount := totalSupply
		storage.Put(ctx, h, amount)
		runtime.Notify("transfer", []byte{}, h, amount)
		return true
	default:
		panic("invalid operation")
	}
}
