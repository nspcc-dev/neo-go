package wallet

import (
	"fmt"
	"path"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

const regenerate = false

const dockerWalletDir = "../../.docker/wallets/"

var (
	// privNetKeys is a list of unencrypted WIFs sorted by wallet number.
	privnetWIFs = []string{
		"KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY",
		"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
		"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",
		"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	}

	passwords = []string{"one", "two", "three", "four"}
)

func getKeys(t *testing.T) []*keys.PublicKey {
	var pubs []*keys.PublicKey

	for i := range privnetWIFs {
		priv, err := keys.NewPrivateKeyFromWIF(privnetWIFs[i])
		require.NoError(t, err)
		pubs = append(pubs, priv.PublicKey())
	}
	return pubs
}

func getAccount(t *testing.T, wif, pass string) *Account {
	acc, err := NewAccountFromWIF(wif)
	require.NoError(t, err)
	require.NoError(t, acc.Encrypt(pass))
	return acc
}

func TestRegenerateSoloWallet(t *testing.T) {
	if !regenerate {
		return
	}
	walletPath := path.Join(dockerWalletDir, "wallet1_solo.json")
	wif := privnetWIFs[0]
	acc1 := getAccount(t, wif, "one")
	acc2 := getAccount(t, wif, "one")
	require.NoError(t, acc2.ConvertMultisig(3, getKeys(t)))

	acc3 := getAccount(t, wif, "one")
	require.NoError(t, acc3.ConvertMultisig(1, keys.PublicKeys{getKeys(t)[0]}))

	w, err := NewWallet(walletPath)
	require.NoError(t, err)
	w.AddAccount(acc1)
	w.AddAccount(acc2)
	w.AddAccount(acc3)
	require.NoError(t, w.savePretty())
	w.Close()
}

func regenerateWallets(t *testing.T, dir string) {
	pubs := getKeys(t)
	for i := range privnetWIFs {
		acc1 := getAccount(t, privnetWIFs[i], passwords[i])
		acc2 := getAccount(t, privnetWIFs[i], passwords[i])
		require.NoError(t, acc2.ConvertMultisig(3, pubs))

		w, err := NewWallet(path.Join(dir, fmt.Sprintf("wallet%d.json", i+1)))
		require.NoError(t, err)
		w.AddAccount(acc1)
		w.AddAccount(acc2)
		require.NoError(t, w.savePretty())
		w.Close()
	}
}

func TestRegeneratePrivnetWallets(t *testing.T) {
	if !regenerate {
		return
	}
	dirs := []string{
		dockerWalletDir,
		"../consensus/testdata",
	}
	for i := range dirs {
		regenerateWallets(t, dirs[i])
	}
}

func TestRegenerateWalletTestdata(t *testing.T) {
	if !regenerate {
		return
	}
	const walletDir = "./testdata/"

	acc1 := getAccount(t, privnetWIFs[0], "one")
	acc2 := getAccount(t, privnetWIFs[0], "one")
	pubs := getKeys(t)
	require.NoError(t, acc2.ConvertMultisig(3, pubs))

	acc3 := getAccount(t, privnetWIFs[1], "two")
	acc3.Default = true

	w, err := NewWallet(path.Join(walletDir, "wallet1.json"))
	require.NoError(t, err)
	w.AddAccount(acc1)
	w.AddAccount(acc2)
	require.NoError(t, w.savePretty())
	w.Close()

	w, err = NewWallet(path.Join(walletDir, "wallet2.json"))
	require.NoError(t, err)
	w.AddAccount(acc1)
	w.AddAccount(acc2)
	w.AddAccount(acc3)
	require.NoError(t, w.savePretty())
	w.Close()
}

func TestRegenerateNotaryWallets(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletDir = "../services/notary/testdata/"
		acc1WIF   = "L1MstxuD8SvS9HuFcV5oYzcdA1xX8D9bD9qPwg8fU5SSywYBecg3"
		acc2WIF   = "L2iGxPvxbyWpYEbCZk2L3PgT7sCQaSDAbBC4MRLAjhs1s2JZ1xs5"
		acc3WIF   = "L1xD2yiUyARX8DAkWa8qGpWpwjqW2u717VzUJyByk6s7HinhRVZv"
		acc4WIF   = "L1ioz93TNt6Nu1aoMpZQ4zgdtgC8ZvJMC6pyHFkrovdR3SFwbn6n"
	)

	acc1 := getAccount(t, acc1WIF, "one")
	acc2 := getAccount(t, acc2WIF, "one")
	acc3 := getAccount(t, acc3WIF, "four")

	w, err := NewWallet(path.Join(walletDir, "notary1.json"))
	require.NoError(t, err)
	w.AddAccount(acc1)
	w.AddAccount(acc2)
	w.AddAccount(acc3)
	require.NoError(t, w.savePretty())
	w.Close()

	acc4 := getAccount(t, acc4WIF, "two")
	w, err = NewWallet(path.Join(walletDir, "notary2.json"))
	require.NoError(t, err)
	w.AddAccount(acc4)
	require.NoError(t, w.savePretty())
	w.Close()
}

func TestRegenerateOracleWallets(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletDir = "../services/oracle/testdata/"
		acc1WIF   = "L38E2tRktb2kWc5j3Kx6Cg3ifVoi4DHhpVZrQormEFTT92C4iSUa"
		acc2WIF   = "KyA8z2MyLCSjJFG3F4SUp85CZ4WJm4qgWihFJZFEDYGEyw8oGcEP"
	)

	acc1 := getAccount(t, acc1WIF, "one")
	w, err := NewWallet(path.Join(walletDir, "oracle1.json"))
	require.NoError(t, err)
	w.AddAccount(acc1)
	require.NoError(t, w.savePretty())
	w.Close()

	acc2 := getAccount(t, acc2WIF, "two")
	w, err = NewWallet(path.Join(walletDir, "oracle2.json"))
	require.NoError(t, err)
	w.AddAccount(acc2)
	require.NoError(t, w.savePretty())
	w.Close()
}
