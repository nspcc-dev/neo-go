package tokensale

import (
	"github.com/CityOfZion/neo-go/pkg/vm/api/runtime"
	"github.com/CityOfZion/neo-go/pkg/vm/api/storage"
)

// Main smart contract entry point.
func Main(operation string, args []interface{}) interface{} {
	var (
		trigger = runtime.GetTrigger()
		cfg     = NewTokenConfig()
		ctx     = storage.GetContext()
	)

	// This is used to verify if a transfer of system assets (NEO and Gas)
	// involving this contract's address can proceed.
	if trigger == runtime.Verification() {
		// Check if the invoker is the owner of the contract.
		if runtime.CheckWitness(cfg.Owner) {
			return true
		}
		// Otherwise TODO
		return false
	}
	if trigger == runtime.Application() {
		return handleOperation(operation, ctx, cfg)
	}
	return true
}

func handleOperation(op string, ctx storage.Context, cfg TokenConfig) interface{} {
	// Handle the NEP5 methods.

	if op == "deploy" {
		return deployContract(cfg, ctx)
	}
	if op == "circulation" {
		return cfg.InCirculation(ctx)
	}
	if op == "mintTokens" {
		return doExchange(cfg, ctx)
	}
	if op == "tokenSaleRegister" {
		// TODO
		return true
	}
	if op == "tokenSaleAvailable" {
		// TODO
		return true
	}
	if op == "getAttachments" {
		// TODO
		return true
	}

	return false
}

func doExchange(cfg TokenConfig, ctx storage.Context) bool {
	return true
}

func canExchange(cfg TokenConfig, ctx storage.Context) bool {
	return true
}

func deployContract(cfg TokenConfig, ctx storage.Context) bool {
	if !runtime.CheckWitness(cfg.Owner) {
		return false
	}
	storage.Put(ctx, "initialized", 1)
	storage.Put(ctx, cfg.Owner, cfg.InitialAmount)
	return cfg.AddToCirculation(ctx, cfg.InitialAmount)
}
