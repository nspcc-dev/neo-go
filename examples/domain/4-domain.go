package domain

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Main is a very useful function.
func Main(operation string, args []interface{}) interface{} {
	ctx := storage.GetContext()

	// Queries the domain owner
	if operation == "query" {
		if checkArgs(args, 1) {
			domainName := args[0].([]byte)
			message := "QueryDomain: " + args[0].(string)
			runtime.Notify(message)

			owner := storage.Get(ctx, domainName)
			if owner != nil {
				runtime.Notify(owner)
				return owner
			}

			runtime.Notify("Domain is not yet registered")
		}
	}

	// Deletes the domain
	if operation == "delete" {
		if checkArgs(args, 1) {
			domainName := args[0].([]byte)
			message := "DeleteDomain: " + args[0].(string)
			runtime.Notify(message)

			owner := storage.Get(ctx, domainName)
			if owner == nil {
				runtime.Notify("Domain is not yet registered")
				return false
			}

			if !runtime.CheckWitness(owner.([]byte)) {
				runtime.Notify("Sender is not the owner, cannot delete")
				return false
			}

			storage.Delete(ctx, domainName)
			return true
		}
	}

	// Registers new domain
	if operation == "register" {
		if checkArgs(args, 2) {
			domainName := args[0].([]byte)
			owner := args[1].([]byte)
			message := "RegisterDomain: " + args[0].(string)
			runtime.Notify(message)

			if !runtime.CheckWitness(owner) {
				runtime.Notify("Owner argument is not the same as the sender")
				return false
			}

			exists := storage.Get(ctx, domainName)
			if exists != nil {
				runtime.Notify("Domain is already registered")
				return false
			}

			storage.Put(ctx, domainName, owner)
			return true
		}
	}

	// Transfers domain from one address to another
	if operation == "transfer" {
		if checkArgs(args, 2) {
			domainName := args[0].([]byte)
			message := "TransferDomain: " + args[0].(string)
			runtime.Notify(message)

			owner := storage.Get(ctx, domainName)
			if owner == nil {
				runtime.Notify("Domain is not yet registered")
				return false
			}

			if !runtime.CheckWitness(owner.([]byte)) {
				runtime.Notify("Sender is not the owner, cannot transfer")
				return false
			}

			toAddress := args[1].([]byte)
			storage.Put(ctx, domainName, toAddress)
			return true
		}
	}

	return false
}

func checkArgs(args []interface{}, length int) bool {
	if len(args) == length {
		return true
	}

	return false
}
