package wallet_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

// signMsgResult mirrors the signMessageResult struct in message.go.
type signMsgResult struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Salt      string `json:"salt"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

func TestSignAndVerifyMessage(t *testing.T) {
	tmpDir := t.TempDir()
	e := testcli.NewExecutor(t, false)

	walletPath := filepath.Join(tmpDir, "wallet.json")
	// Create a wallet with a single account.
	e.In.WriteString("acc\r")
	e.In.WriteString("pass\r")
	e.In.WriteString("pass\r")
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath, "--account")

	w, err := wallet.NewWalletFromFile(walletPath)
	require.NoError(t, err)
	acc := w.Accounts[0]
	addr := acc.Address

	t.Run("missing wallet", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "wallet", "sign-msg", "hello")
	})
	t.Run("missing message", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.RunWithError(t, "neo-go", "wallet", "sign-msg", "--wallet", walletPath)
	})
	t.Run("too many args", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.RunWithError(t, "neo-go", "wallet", "sign-msg", "--wallet", walletPath, "a", "b")
	})
	t.Run("wrong password", func(t *testing.T) {
		e.In.WriteString("wrongpass\r")
		e.RunWithError(t, "neo-go", "wallet", "sign-msg", "--wallet", walletPath, "--address", addr, "hello")
	})
	t.Run("sign plain text", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))
		require.Equal(t, addr, res.Address)
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("hello")), res.Message)
		require.Equal(t, keys.WalletConnectSaltLen*2, len(res.Salt))
		require.Equal(t, keys.SignatureLen*2, len(res.Signature))

		// Verify with verify-msg using --public-key.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", res.PublicKey,
			"--salt", res.Salt,
			"--signature", res.Signature,
			"hello")
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("sign hex message", func(t *testing.T) {
		hexMsg := hex.EncodeToString([]byte("binary data"))
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "--hex", hexMsg)

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))
		require.Equal(t, base64.StdEncoding.EncodeToString([]byte("binary data")), res.Message)

		// Verify.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", res.PublicKey,
			"--salt", res.Salt,
			"--signature", res.Signature,
			"--hex", hexMsg)
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("sign base64 message", func(t *testing.T) {
		b64Msg := base64.StdEncoding.EncodeToString([]byte("binary data"))
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "--base64", b64Msg)

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))
		require.Equal(t, b64Msg, res.Message)

		// Verify.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", res.PublicKey,
			"--salt", res.Salt,
			"--signature", res.Signature,
			"--base64", b64Msg)
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("verify with wrong signature fails", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))

		wrongSig := make([]byte, keys.SignatureLen)
		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--public-key", res.PublicKey,
			"--salt", res.Salt,
			"--signature", hex.EncodeToString(wrongSig),
			"hello")
	})
	t.Run("verify with wrong message fails", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))

		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--public-key", res.PublicKey,
			"--salt", res.Salt,
			"--signature", res.Signature,
			"wrong message")
	})
	t.Run("verify-msg missing salt or signature", func(t *testing.T) {
		e.RunWithErrorCheck(t, `Required flag "salt" not set`,
			"neo-go", "wallet", "verify-msg",
			"--public-key", "030000000000000000000000000000000000000000000000000000000000000001",
			"--signature", "0000000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000000",
			"hello")
	})
	t.Run("verify-msg missing address and public key", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--salt", "00000000000000000000000000000000",
			"--signature", "0000000000000000000000000000000000000000000000000000000000000000"+
				"0000000000000000000000000000000000000000000000000000000000000000",
			"hello")
	})
	t.Run("verify with wallet address", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		output := e.Out.String()
		var res signMsgResult
		require.NoError(t, json.Unmarshal([]byte(output), &res))

		// Verify using wallet + address instead of public-key.
		// No password needed since the public key is extracted from the
		// verification script without decryption.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--wallet", walletPath,
			"--address", addr,
			"--salt", res.Salt,
			"--signature", res.Signature,
			"hello")
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
}
