package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestSize(t *testing.T) {
	txID := "f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a"
	tx := getTestTransaction(txID, t)

	assert.Equal(t, 283, tx.Size())
	assert.Equal(t, 22, util.GetVarSize(tx.Attributes))
	assert.Equal(t, 35, util.GetVarSize(tx.Inputs))
	assert.Equal(t, 121, util.GetVarSize(tx.Outputs))
	assert.Equal(t, 103, util.GetVarSize(tx.Scripts))
}

func getTestBlockchain(t *testing.T) *Blockchain {
	net := config.ModeUnitTestNet
	configPath := "../../config"
	cfg, err := config.Load(configPath, net)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	// adjust datadirectory to point to the correct folder
	cfg.ApplicationConfiguration.DataDirectoryPath = "../rpc/chains/unit_testnet"
	chain, err := NewBlockchainLevelDB(cfg)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	return chain
}

func getTestTransaction(txID string, t *testing.T) *transaction.Transaction {
	chain := getTestBlockchain(t)

	txHash, err := util.Uint256DecodeString(txID)
	if err != nil {
		t.Fatalf("could not decode string %s to Uint256: err =%s", txID, err)
	}

	tx, _, err := chain.GetTransaction(txHash)
	if err != nil {
		t.Fatalf("Could not get transaction with hash=%s: err=%s", txHash, err)
	}
	return tx
}
