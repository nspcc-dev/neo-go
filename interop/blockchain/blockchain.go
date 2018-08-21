package blockchain

import (
	"github.com/CityOfZion/neo-go-sc/interop/account"
	"github.com/CityOfZion/neo-go-sc/interop/asset"
	"github.com/CityOfZion/neo-go-sc/interop/block"
	"github.com/CityOfZion/neo-go-sc/interop/contract"
	"github.com/CityOfZion/neo-go-sc/interop/header"
	"github.com/CityOfZion/neo-go-sc/interop/transaction"
)

// Package blockchain provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// GetHeight returns the height of te block recorded in the current execution scope.
func GetHeight() int {
	return 0
}

// GetHeader returns the header found by the given hash or index.
func GetHeader(heightOrHash interface{}) header.Header {
	return header.Header{}
}

// GetBlock returns the block found by the given hash or index.
func GetBlock(heightOrHash interface{}) block.Block {
	return block.Block{}
}

// GetTransaction returns the transaction found by the given hash.
func GetTransaction(hash []byte) transaction.Transaction {
	return transaction.Transaction{}
}

// GetContract returns the contract found by the given script hash.
func GetContract(scriptHash []byte) contract.Contract {
	return contract.Contract{}
}

// GetAccount returns the account found by the given script hash.
func GetAccount(scriptHash []byte) account.Account {
	return account.Account{}
}

// GetValidators returns a slice of validator addresses.
func GetValidators() [][]byte {
	return nil
}

// GetAsset returns the asset found by the given asset id.
func GetAsset(assetID []byte) asset.Asset {
	return asset.Asset{}
}
