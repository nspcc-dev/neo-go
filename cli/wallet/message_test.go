package wallet_test

import (
	"encoding/base64"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

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
	require.NotEmpty(t, w.Accounts)
	acc := w.Accounts[0]
	addr := acc.Address

	// Extract the public key from the verification script (no decryption needed).
	pubKeyBytes, ok := scparser.ParseSignatureContract(acc.Contract.Script)
	require.True(t, ok)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

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

		sigHex := strings.TrimSpace(e.Out.String())
		require.Len(t, sigHex, (keys.SignatureLen+keys.WalletConnectSaltLen)*2)
		_, err := hex.DecodeString(sigHex)
		require.NoError(t, err)

		// Verify with --public-key.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"--signature", sigHex,
			"hello")
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("sign hex message", func(t *testing.T) {
		hexMsg := hex.EncodeToString([]byte("binary data"))
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "--hex", hexMsg)

		sigHex := strings.TrimSpace(e.Out.String())
		require.Len(t, sigHex, (keys.SignatureLen+keys.WalletConnectSaltLen)*2)

		// Verify.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"--signature", sigHex,
			"--hex", hexMsg)
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("sign base64 message", func(t *testing.T) {
		b64Msg := base64.StdEncoding.EncodeToString([]byte("binary data"))
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "--base64", b64Msg)

		sigHex := strings.TrimSpace(e.Out.String())
		require.Len(t, sigHex, (keys.SignatureLen+keys.WalletConnectSaltLen)*2)

		// Verify.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"--signature", sigHex,
			"--base64", b64Msg)
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
	t.Run("verify with wrong signature fails", func(t *testing.T) {
		wrongSig := hex.EncodeToString(make([]byte, keys.SignatureLen+keys.WalletConnectSaltLen))
		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"--signature", wrongSig,
			"hello")
	})
	t.Run("verify with wrong message fails", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		sigHex := strings.TrimSpace(e.Out.String())
		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"--signature", sigHex,
			"wrong message")
	})
	t.Run("verify-msg missing signature", func(t *testing.T) {
		e.RunWithErrorCheck(t, `Required flag "signature" not set`,
			"neo-go", "wallet", "verify-msg",
			"--public-key", pubKeyHex,
			"hello")
	})
	t.Run("verify-msg missing address and public key", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "wallet", "verify-msg",
			"--signature", hex.EncodeToString(make([]byte, keys.SignatureLen+keys.WalletConnectSaltLen)),
			"hello")
	})
	t.Run("verify with wallet address", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "sign-msg",
			"--wallet", walletPath, "--address", addr, "hello")

		sigHex := strings.TrimSpace(e.Out.String())

		// Verify using wallet + address instead of public-key.
		// No password needed since the public key is extracted from the
		// verification script without decryption.
		e.Run(t, "neo-go", "wallet", "verify-msg",
			"--wallet", walletPath,
			"--address", addr,
			"--signature", sigHex,
			"hello")
		require.Contains(t, e.Out.String(), "Signature is correct")
	})
}
