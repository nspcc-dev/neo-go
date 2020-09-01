package main

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestWalletInit(t *testing.T) {
	tmpDir := os.TempDir()
	e := newExecutor(t, false)
	defer e.Close(t)

	walletPath := path.Join(tmpDir, "wallet.json")
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)
	defer os.Remove(walletPath)

	t.Run("CreateAccount", func(t *testing.T) {
		e.In.WriteString("testname\r")
		e.In.WriteString("testpass\r")
		e.In.WriteString("testpass\r")
		e.Run(t, "neo-go", "wallet", "create", "--wallet", walletPath)

		w, err := wallet.NewWalletFromFile(walletPath)
		require.NoError(t, err)
		defer w.Close()
		require.Len(t, w.Accounts, 1)
		require.Equal(t, w.Accounts[0].Label, "testname")
		require.NoError(t, w.Accounts[0].Decrypt("testpass"))
	})

	t.Run("Import", func(t *testing.T) {
		t.Run("WIF", func(t *testing.T) {
			priv, err := keys.NewPrivateKey()
			require.NoError(t, err)
			e.In.WriteString("test_account\r")
			e.In.WriteString("qwerty\r")
			e.In.WriteString("qwerty\r")
			e.Run(t, "neo-go", "wallet", "import", "--wallet", walletPath,
				"--wif", priv.WIF())

			w, err := wallet.NewWalletFromFile(walletPath)
			require.NoError(t, err)
			defer w.Close()
			acc := w.GetAccount(priv.GetScriptHash())
			require.NotNil(t, acc)
			require.Equal(t, "test_account", acc.Label)
			require.NoError(t, acc.Decrypt("qwerty"))

			t.Run("AlreadyExists", func(t *testing.T) {
				e.In.WriteString("test_account_2\r")
				e.In.WriteString("qwerty2\r")
				e.In.WriteString("qwerty2\r")
				e.RunWithError(t, "neo-go", "wallet", "import",
					"--wallet", walletPath, "--wif", priv.WIF())
			})
		})
		t.Run("EncryptedWIF", func(t *testing.T) {
			acc, err := wallet.NewAccount()
			require.NoError(t, err)
			require.NoError(t, acc.Encrypt("somepass"))

			t.Run("InvalidPassword", func(t *testing.T) {
				e.In.WriteString("password1\r")
				e.RunWithError(t, "neo-go", "wallet", "import", "--wallet", walletPath,
					"--wif", acc.EncryptedWIF)
			})

			e.In.WriteString("somepass\r")
			e.Run(t, "neo-go", "wallet", "import", "--wallet", walletPath,
				"--wif", acc.EncryptedWIF)

			w, err := wallet.NewWalletFromFile(walletPath)
			require.NoError(t, err)
			defer w.Close()
			actual := w.GetAccount(acc.PrivateKey().GetScriptHash())
			require.NotNil(t, actual)
			require.NoError(t, actual.Decrypt("somepass"))
		})
	})
}

func TestWalletExport(t *testing.T) {
	e := newExecutor(t, false)
	defer e.Close(t)

	t.Run("Encrypted", func(t *testing.T) {
		e.Run(t, "neo-go", "wallet", "export",
			"--wallet", validatorWallet, validatorAddr)
		line, err := e.Out.ReadString('\n')
		require.NoError(t, err)
		enc, err := keys.NEP2Encrypt(validatorPriv, "one")
		require.NoError(t, err)
		require.Equal(t, enc, strings.TrimSpace(line))
	})
	t.Run("Decrypted", func(t *testing.T) {
		t.Run("NoAddress", func(t *testing.T) {
			e.RunWithError(t, "neo-go", "wallet", "export",
				"--wallet", validatorWallet, "--decrypt")
		})
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "export",
			"--wallet", validatorWallet, "--decrypt", validatorAddr)
		line, err := e.Out.ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, validatorWIF, strings.TrimSpace(line))
	})
}
