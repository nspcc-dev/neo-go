package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// KVPair represents a key-value pair.
type KVPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TraverseMPT collects key-value pairs from the TrieStore and returns them.
func TraverseMPT(store *mpt.TrieStore, encoder *json.Encoder) error {
	prefix := []byte{byte(storage.STStorage)}
	rng := storage.SeekRange{Prefix: prefix}

	store.Seek(rng, func(k, v []byte) bool {
		kvPair := KVPair{
			Key:   hex.EncodeToString(k),
			Value: hex.EncodeToString(v),
		}
		if err := encoder.Encode(kvPair); err != nil {
			fmt.Printf("error encoding key-value pair: %v\n", err)
			return false
		}
		return true
	})
	return nil
}

// traverseMPT handles the CLI command to traverse the MPT and dump key-value pairs.
func traverseMPT(ctx *cli.Context) error {
	logger := zap.NewExample()
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}

	chain, store, err := initBlockChain(cfg, logger)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer store.Close()
	defer chain.Close()

	stateModule := chain.GetStateModule()
	stateRoot := stateModule.CurrentLocalStateRoot()
	stateRootHash := stateRoot

	trieStore := mpt.NewTrieStore(stateRootHash, mpt.ModeAll, store)

	outputFile := ctx.String("out")
	if outputFile == "" {
		outputFile = "kv_pairs.json"
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	startTime := time.Now()
	fmt.Println(startTime)
	err = TraverseMPT(trieStore, encoder)
	if err != nil {
		return cli.Exit(err, 1)
	}

	duration := time.Since(startTime)
	fmt.Printf("MPT key-value pairs successfully dumped to %s in %s\n", outputFile, duration)
	return nil
}
