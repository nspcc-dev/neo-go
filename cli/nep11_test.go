package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

const (
	// nftOwnerAddr is the owner of NFT-ND HASHY token (../examples/nft-nd/nft.go).
	nftOwnerAddr   = "NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB"
	nftOwnerWallet = "../examples/my_wallet.json"
	nftOwnerPass   = "qwerty"
)

func TestNEP11Import(t *testing.T) {
	e := newExecutor(t, true)

	tmpDir := t.TempDir()
	walletPath := filepath.Join(tmpDir, "walletForImport.json")

	// deploy NFT NeoNameService contract
	nnsContractHash := deployNNSContract(t, e)
	// deploy NFT-D NeoFS Object contract
	nfsContractHash := deployNFSContract(t, e)
	neoContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Neo)
	require.NoError(t, err)
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)

	args := []string{
		"neo-go", "wallet", "nep11", "import",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", walletPath,
	}
	// missing token hash
	e.RunWithError(t, args...)

	// good: non-divisible
	e.Run(t, append(args, "--token", nnsContractHash.StringLE())...)

	// good: divisible
	e.Run(t, append(args, "--token", nfsContractHash.StringLE())...)

	// already exists
	e.RunWithError(t, append(args, "--token", nnsContractHash.StringLE())...)

	// not a NEP-11 token
	e.RunWithError(t, append(args, "--token", neoContractHash.StringLE())...)

	checkInfo := func(t *testing.T, h util.Uint160, name string, symbol string, decimals int) {
		e.checkNextLine(t, "^Name:\\s*"+name)
		e.checkNextLine(t, "^Symbol:\\s*"+symbol)
		e.checkNextLine(t, "^Hash:\\s*"+h.StringLE())
		e.checkNextLine(t, "^Decimals:\\s*"+strconv.Itoa(decimals))
		e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(h))
		e.checkNextLine(t, "^Standard:\\s*"+string(manifest.NEP11StandardName))
	}
	t.Run("Info", func(t *testing.T) {
		t.Run("WithToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep11", "info",
				"--wallet", walletPath, "--token", nnsContractHash.StringLE())
			checkInfo(t, nnsContractHash, "NameService", "NNS", 0)
		})
		t.Run("NoToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep11", "info",
				"--wallet", walletPath)
			checkInfo(t, nnsContractHash, "NameService", "NNS", 0)
			e.checkNextLine(t, "")
			checkInfo(t, nfsContractHash, "NeoFS Object NFT", "NFSO", 2)
		})
	})

	t.Run("Remove", func(t *testing.T) {
		e.In.WriteString("y\r")
		e.Run(t, "neo-go", "wallet", "nep11", "remove",
			"--wallet", walletPath, "--token", nnsContractHash.StringLE())
		e.Run(t, "neo-go", "wallet", "nep11", "info",
			"--wallet", walletPath)
		checkInfo(t, nfsContractHash, "NeoFS Object NFT", "NFSO", 2)
		_, err := e.Out.ReadString('\n')
		require.Equal(t, err, io.EOF)
	})
}

func TestNEP11_ND_OwnerOf_BalanceOf_Transfer(t *testing.T) {
	e := newExecutor(t, true)
	tmpDir := t.TempDir()

	// copy wallet to temp dir in order not to overwrite the original file
	bytesRead, err := os.ReadFile(nftOwnerWallet)
	require.NoError(t, err)
	wall := filepath.Join(tmpDir, "my_wallet.json")
	err = os.WriteFile(wall, bytesRead, 0755)
	require.NoError(t, err)

	// transfer funds to contract owner
	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep17", "transfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--to", nftOwnerAddr,
		"--token", "GAS",
		"--amount", "10000",
		"--force",
		"--from", validatorAddr)
	e.checkTxPersisted(t)

	// deploy NFT HASHY contract
	h := deployNFTContract(t, e)

	mint := func(t *testing.T) []byte {
		// mint 1 HASHY token by transferring 10 GAS to HASHY contract
		e.In.WriteString(nftOwnerPass + "\r")
		e.Run(t, "neo-go", "wallet", "nep17", "transfer",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wall,
			"--to", h.StringLE(),
			"--token", "GAS",
			"--amount", "10",
			"--force",
			"--from", nftOwnerAddr)
		txMint, _ := e.checkTxPersisted(t)

		// get NFT ID from AER
		aer, err := e.Chain.GetAppExecResults(txMint.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(aer))
		require.Equal(t, 2, len(aer[0].Events))
		hashyMintEvent := aer[0].Events[1]
		require.Equal(t, "Transfer", hashyMintEvent.Name)
		tokenID, err := hashyMintEvent.Item.Value().([]stackitem.Item)[3].TryBytes()
		require.NoError(t, err)
		require.NotNil(t, tokenID)
		return tokenID
	}

	tokenID := mint(t)

	// check the balance
	cmdCheckBalance := []string{"neo-go", "wallet", "nep11", "balance",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--address", nftOwnerAddr}
	checkBalanceResult := func(t *testing.T, acc string, amount string) {
		e.checkNextLine(t, "^\\s*Account\\s+"+acc)
		e.checkNextLine(t, "^\\s*HASHY:\\s+HASHY NFT \\("+h.StringLE()+"\\)")
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+amount+"$")
		e.checkEOF(t)
	}
	// balance check: by symbol, token is not imported
	e.RunWithError(t, append(cmdCheckBalance, "--token", "HASHY")...)
	// balance check: by hash, ok
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, nftOwnerAddr, "1")

	// import token
	e.Run(t, "neo-go", "wallet", "nep11", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wall,
		"--token", h.StringLE())

	// balance check: by symbol, ok
	e.Run(t, append(cmdCheckBalance, "--token", "HASHY")...)
	checkBalanceResult(t, nftOwnerAddr, "1")

	// balance check: all accounts
	e.Run(t, "neo-go", "wallet", "nep11", "balance",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wall,
		"--token", h.StringLE())
	checkBalanceResult(t, nftOwnerAddr, "1")

	// remove token from wallet
	e.In.WriteString("y\r")
	e.Run(t, "neo-go", "wallet", "nep11", "remove",
		"--wallet", wall, "--token", h.StringLE())

	// ownerOf: missing contract hash
	cmdOwnerOf := []string{"neo-go", "wallet", "nep11", "ownerOf",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdOwnerOf...)
	cmdOwnerOf = append(cmdOwnerOf, "--token", h.StringLE())

	// ownerOf: missing token ID
	e.RunWithError(t, cmdOwnerOf...)
	cmdOwnerOf = append(cmdOwnerOf, "--id", hex.EncodeToString(tokenID))

	// ownerOf: good
	e.Run(t, cmdOwnerOf...)
	e.checkNextLine(t, nftOwnerAddr)

	// tokensOf: missing contract hash
	cmdTokensOf := []string{"neo-go", "wallet", "nep11", "tokensOf",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdTokensOf...)
	cmdTokensOf = append(cmdTokensOf, "--token", h.StringLE())

	// tokensOf: missing owner address
	e.RunWithError(t, cmdTokensOf...)
	cmdTokensOf = append(cmdTokensOf, "--address", nftOwnerAddr)

	// tokensOf: good
	e.Run(t, cmdTokensOf...)
	require.Equal(t, hex.EncodeToString(tokenID), e.getNextLine(t))

	// properties: no contract
	cmdProperties := []string{
		"neo-go", "wallet", "nep11", "properties",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdProperties...)
	cmdProperties = append(cmdProperties, "--token", h.StringLE())

	// properties: no token ID
	e.RunWithError(t, cmdProperties...)
	cmdProperties = append(cmdProperties, "--id", hex.EncodeToString(tokenID))

	// properties: ok
	e.Run(t, cmdProperties...)
	require.Equal(t, fmt.Sprintf(`{"name":"HASHY %s"}`, base64.StdEncoding.EncodeToString(tokenID)), e.getNextLine(t))

	// tokensOf: good, several tokens
	tokenID1 := mint(t)
	e.Run(t, cmdTokensOf...)
	fst, snd := tokenID, tokenID1
	if bytes.Compare(tokenID, tokenID1) == 1 {
		fst, snd = snd, fst
	}

	require.Equal(t, hex.EncodeToString(fst), e.getNextLine(t))
	require.Equal(t, hex.EncodeToString(snd), e.getNextLine(t))

	// tokens: missing contract hash
	cmdTokens := []string{"neo-go", "wallet", "nep11", "tokens",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdTokens...)
	cmdTokens = append(cmdTokens, "--token", h.StringLE())

	// tokens: good, several tokens
	e.Run(t, cmdTokens...)
	require.Equal(t, hex.EncodeToString(fst), e.getNextLine(t))
	require.Equal(t, hex.EncodeToString(snd), e.getNextLine(t))

	// balance check: several tokens, ok
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, nftOwnerAddr, "2")

	cmdTransfer := []string{
		"neo-go", "wallet", "nep11", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--to", validatorAddr,
		"--from", nftOwnerAddr,
		"--force",
	}

	// transfer: unimported token with symbol id specified
	e.In.WriteString(nftOwnerPass + "\r")
	e.RunWithError(t, append(cmdTransfer,
		"--token", "HASHY")...)
	cmdTransfer = append(cmdTransfer, "--token", h.StringLE())

	// transfer: no id specified
	e.In.WriteString(nftOwnerPass + "\r")
	e.RunWithError(t, cmdTransfer...)

	// transfer: good
	e.In.WriteString(nftOwnerPass + "\r")
	e.Run(t, append(cmdTransfer, "--id", hex.EncodeToString(tokenID))...)
	e.checkTxPersisted(t)

	// check balance after transfer
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, nftOwnerAddr, "1") // tokenID1

	// transfer: good, to NEP-11-Payable contract, with data
	verifyH := deployVerifyContract(t, e)
	cmdTransfer = []string{
		"neo-go", "wallet", "nep11", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--to", verifyH.StringLE(),
		"--from", nftOwnerAddr,
		"--token", h.StringLE(),
		"--id", hex.EncodeToString(tokenID1),
		"--force",
		"string:some_data",
	}
	e.In.WriteString(nftOwnerPass + "\r")
	e.Run(t, cmdTransfer...)
	tx, _ := e.checkTxPersisted(t)
	// check OnNEP11Payment event
	aer, err := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 2, len(aer[0].Events))
	nftOwnerHash, err := address.StringToUint160(nftOwnerAddr)
	require.NoError(t, err)
	require.Equal(t, state.NotificationEvent{
		ScriptHash: verifyH,
		Name:       "OnNEP11Payment",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(nftOwnerHash.BytesBE()),
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.NewByteArray(tokenID1),
			stackitem.NewByteArray([]byte("some_data")),
		}),
	}, aer[0].Events[1])

	// check balance after transfer
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, nftOwnerAddr, "0")
}

func TestNEP11_D_OwnerOf_BalanceOf_Transfer(t *testing.T) {
	e := newExecutor(t, true)
	tmpDir := t.TempDir()

	// copy wallet to temp dir in order not to overwrite the original file
	bytesRead, err := os.ReadFile(validatorWallet)
	require.NoError(t, err)
	wall := filepath.Join(tmpDir, "my_wallet.json")
	err = os.WriteFile(wall, bytesRead, 0755)
	require.NoError(t, err)

	// deploy NeoFS Object contract
	h := deployNFSContract(t, e)

	mint := func(t *testing.T, containerID, objectID util.Uint256) []byte {
		// mint 1.00 NFSO token by transferring 10 GAS to NFSO contract
		e.In.WriteString(validatorPass + "\r")
		e.Run(t, "neo-go", "wallet", "nep17", "transfer",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wall,
			"--to", h.StringLE(),
			"--token", "GAS",
			"--amount", "10",
			"--force",
			"--from", validatorAddr,
			"--", "[", "hash256:"+containerID.StringLE(), "hash256:"+objectID.StringLE(), "]",
		)
		txMint, _ := e.checkTxPersisted(t)

		// get NFT ID from AER
		aer, err := e.Chain.GetAppExecResults(txMint.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(aer))
		require.Equal(t, 2, len(aer[0].Events))
		nfsoMintEvent := aer[0].Events[1]
		require.Equal(t, "Transfer", nfsoMintEvent.Name)
		tokenID, err := nfsoMintEvent.Item.Value().([]stackitem.Item)[3].TryBytes()
		require.NoError(t, err)
		require.NotNil(t, tokenID)
		return tokenID
	}

	container1ID := util.Uint256{1, 2, 3}
	object1ID := util.Uint256{4, 5, 6}
	token1ID := mint(t, container1ID, object1ID)

	container2ID := util.Uint256{7, 8, 9}
	object2ID := util.Uint256{10, 11, 12}
	token2ID := mint(t, container2ID, object2ID)

	// check properties
	e.Run(t, "neo-go", "wallet", "nep11", "properties",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--token", h.StringLE(),
		"--id", hex.EncodeToString(token1ID))
	jProps := e.getNextLine(t)
	props := make(map[string]string)
	require.NoError(t, json.Unmarshal([]byte(jProps), &props))
	require.Equal(t, base64.StdEncoding.EncodeToString(container1ID.BytesBE()), props["containerID"])
	require.Equal(t, base64.StdEncoding.EncodeToString(object1ID.BytesBE()), props["objectID"])
	e.checkEOF(t)

	// check the balance
	cmdCheckBalance := []string{"neo-go", "wallet", "nep11", "balance",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--address", validatorAddr}
	checkBalanceResult := func(t *testing.T, acc string, amount string, id []byte) {
		e.checkNextLine(t, "^\\s*Account\\s+"+acc)
		if id == nil {
			e.checkNextLine(t, "^\\s*NFSO:\\s+NeoFS Object NFT \\("+h.StringLE()+"\\)")
		} else {
			e.checkNextLine(t, "^\\s*NFSO:\\s+NeoFS Object NFT \\("+h.StringLE()+", "+hex.EncodeToString(id)+"\\)")
		}
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+amount+"$")
		e.checkEOF(t)
	}
	// balance check: by symbol, token is not imported
	e.RunWithError(t, append(cmdCheckBalance, "--token", "NFSO")...)

	// overall NFSO balance check: by hash, ok
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, validatorAddr, "2", nil)

	// particular NFSO balance check: by hash, ok
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE(), "--id", hex.EncodeToString(token2ID))...)
	checkBalanceResult(t, validatorAddr, "1", token2ID)

	// import token
	e.Run(t, "neo-go", "wallet", "nep11", "import",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wall,
		"--token", h.StringLE())

	// overall balance check: by symbol, ok
	e.Run(t, append(cmdCheckBalance, "--token", "NFSO")...)
	checkBalanceResult(t, validatorAddr, "2", nil)

	// particular balance check: by symbol, ok
	e.Run(t, append(cmdCheckBalance, "--token", "NFSO", "--id", hex.EncodeToString(token1ID))...)
	checkBalanceResult(t, validatorAddr, "1", token1ID)

	// remove token from wallet
	e.In.WriteString("y\r")
	e.Run(t, "neo-go", "wallet", "nep11", "remove",
		"--wallet", wall, "--token", h.StringLE())

	// ownerOfD: missing contract hash
	cmdOwnerOf := []string{"neo-go", "wallet", "nep11", "ownerOfD",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdOwnerOf...)
	cmdOwnerOf = append(cmdOwnerOf, "--token", h.StringLE())

	// ownerOfD: missing token ID
	e.RunWithError(t, cmdOwnerOf...)
	cmdOwnerOf = append(cmdOwnerOf, "--id", hex.EncodeToString(token1ID))

	// ownerOfD: good
	e.Run(t, cmdOwnerOf...)
	e.checkNextLine(t, validatorAddr)

	// tokensOf: missing contract hash
	cmdTokensOf := []string{"neo-go", "wallet", "nep11", "tokensOf",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdTokensOf...)
	cmdTokensOf = append(cmdTokensOf, "--token", h.StringLE())

	// tokensOf: missing owner address
	e.RunWithError(t, cmdTokensOf...)
	cmdTokensOf = append(cmdTokensOf, "--address", validatorAddr)

	// tokensOf: good
	e.Run(t, cmdTokensOf...)
	require.Equal(t, hex.EncodeToString(token1ID), e.getNextLine(t))
	require.Equal(t, hex.EncodeToString(token2ID), e.getNextLine(t))
	e.checkEOF(t)

	// properties: no contract
	cmdProperties := []string{
		"neo-go", "wallet", "nep11", "properties",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdProperties...)
	cmdProperties = append(cmdProperties, "--token", h.StringLE())

	// properties: no token ID
	e.RunWithError(t, cmdProperties...)
	cmdProperties = append(cmdProperties, "--id", hex.EncodeToString(token2ID))

	// properties: ok
	e.Run(t, cmdProperties...)
	jProps = e.getNextLine(t)
	props = make(map[string]string)
	require.NoError(t, json.Unmarshal([]byte(jProps), &props))
	require.Equal(t, base64.StdEncoding.EncodeToString(container2ID.BytesBE()), props["containerID"])
	require.Equal(t, base64.StdEncoding.EncodeToString(object2ID.BytesBE()), props["objectID"])
	e.checkEOF(t)

	// tokensOf: good, several tokens
	e.Run(t, cmdTokensOf...)
	fst, snd := token1ID, token2ID
	if bytes.Compare(token1ID, token2ID) == 1 {
		fst, snd = snd, fst
	}

	require.Equal(t, hex.EncodeToString(fst), e.getNextLine(t))
	require.Equal(t, hex.EncodeToString(snd), e.getNextLine(t))

	// tokens: missing contract hash
	cmdTokens := []string{"neo-go", "wallet", "nep11", "tokens",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
	}
	e.RunWithError(t, cmdTokens...)
	cmdTokens = append(cmdTokens, "--token", h.StringLE())

	// tokens: good, several tokens
	e.Run(t, cmdTokens...)
	require.Equal(t, hex.EncodeToString(fst), e.getNextLine(t))
	require.Equal(t, hex.EncodeToString(snd), e.getNextLine(t))

	// balance check: several tokens, ok
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, validatorAddr, "2", nil)

	cmdTransfer := []string{
		"neo-go", "wallet", "nep11", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--to", nftOwnerAddr,
		"--from", validatorAddr,
		"--force",
	}

	// transfer: unimported token with symbol id specified
	e.In.WriteString(validatorPass + "\r")
	e.RunWithError(t, append(cmdTransfer,
		"--token", "NFSO")...)
	cmdTransfer = append(cmdTransfer, "--token", h.StringLE())

	// transfer: no id specified
	e.In.WriteString(validatorPass + "\r")
	e.RunWithError(t, cmdTransfer...)

	// transfer: good
	e.In.WriteString(validatorPass + "\r")
	e.Run(t, append(cmdTransfer, "--id", hex.EncodeToString(token1ID))...)
	e.checkTxPersisted(t)

	// check balance after transfer
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, validatorAddr, "1", nil) // only token2ID expected to be on the balance

	// transfer: good, 1/4 of the balance, to NEP-11-Payable contract, with data
	verifyH := deployVerifyContract(t, e)
	cmdTransfer = []string{
		"neo-go", "wallet", "nep11", "transfer",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", wall,
		"--to", verifyH.StringLE(),
		"--from", validatorAddr,
		"--token", h.StringLE(),
		"--id", hex.EncodeToString(token2ID),
		"--amount", "0.25",
		"--force",
		"string:some_data",
	}
	e.In.WriteString(validatorPass + "\r")
	e.Run(t, cmdTransfer...)
	tx, _ := e.checkTxPersisted(t)
	// check OnNEP11Payment event
	aer, err := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 2, len(aer[0].Events))
	validatorHash, err := address.StringToUint160(validatorAddr)
	require.NoError(t, err)
	require.Equal(t, state.NotificationEvent{
		ScriptHash: verifyH,
		Name:       "OnNEP11Payment",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(validatorHash.BytesBE()),
			stackitem.NewBigInteger(big.NewInt(25)),
			stackitem.NewByteArray(token2ID),
			stackitem.NewByteArray([]byte("some_data")),
		}),
	}, aer[0].Events[1])

	// check balance after transfer
	e.Run(t, append(cmdCheckBalance, "--token", h.StringLE())...)
	checkBalanceResult(t, validatorAddr, "0.75", nil)
}

func deployNFSContract(t *testing.T, e *executor) util.Uint160 {
	return deployContract(t, e, "../examples/nft-d/nft.go", "../examples/nft-d/nft.yml", validatorWallet, validatorAddr, validatorPass)
}

func deployNFTContract(t *testing.T, e *executor) util.Uint160 {
	return deployContract(t, e, "../examples/nft-nd/nft.go", "../examples/nft-nd/nft.yml", nftOwnerWallet, nftOwnerAddr, nftOwnerPass)
}

func deployNNSContract(t *testing.T, e *executor) util.Uint160 {
	return deployContract(t, e, "../examples/nft-nd-nns/", "../examples/nft-nd-nns/nns.yml", validatorWallet, validatorAddr, validatorPass)
}
