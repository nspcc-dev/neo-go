package wallet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
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

	// naiveScrypt is naive scrypt parameters used for testing wallets to save
	// time on accounts decryption.
	naiveScrypt = keys.ScryptParams{N: 2, R: 1, P: 1}
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

func getNEP2Account(t *testing.T, wif, pass string) *Account {
	return getAccountWithScrypt(t, wif, pass, keys.NEP2ScryptParams())
}

func getTestingAccount(t *testing.T, wif, pass string) *Account {
	return getAccountWithScrypt(t, wif, pass, naiveScrypt)
}

func getAccountWithScrypt(t *testing.T, wif, pass string, scrypt keys.ScryptParams) *Account {
	acc, err := NewAccountFromWIF(wif)
	require.NoError(t, err)
	require.NoError(t, acc.Encrypt(pass, scrypt))
	return acc
}

func TestRegenerateSoloWallet(t *testing.T) {
	if !regenerate {
		return
	}
	walletPath := filepath.Join(dockerWalletDir, "wallet1_solo.json")
	wif := privnetWIFs[0]
	acc1 := getNEP2Account(t, wif, "one")
	acc2 := getNEP2Account(t, wif, "one")
	require.NoError(t, acc2.ConvertMultisig(3, getKeys(t)))

	acc3 := getNEP2Account(t, wif, "one")
	require.NoError(t, acc3.ConvertMultisig(1, keys.PublicKeys{getKeys(t)[0]}))

	createNEP2Wallet(t, walletPath, acc1, acc2, acc3)
}

func regenerateWallets(t *testing.T, dir string) {
	pubs := getKeys(t)
	for i := range privnetWIFs {
		acc1 := getNEP2Account(t, privnetWIFs[i], passwords[i])
		acc2 := getNEP2Account(t, privnetWIFs[i], passwords[i])
		require.NoError(t, acc2.ConvertMultisig(3, pubs))

		createNEP2Wallet(t, filepath.Join(dir, fmt.Sprintf("wallet%d.json", i+1)), acc1, acc2)
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

	acc1 := getTestingAccount(t, privnetWIFs[0], "one")
	acc2 := getTestingAccount(t, privnetWIFs[0], "one")
	pubs := getKeys(t)
	require.NoError(t, acc2.ConvertMultisig(3, pubs))

	acc3 := getTestingAccount(t, privnetWIFs[1], "two")
	acc3.Default = true

	createTestingWallet(t, filepath.Join(walletDir, "wallet1.json"), acc1, acc2)

	createTestingWallet(t, filepath.Join(walletDir, "wallet2.json"), acc1, acc2, acc3)
}

func TestRegenerateNotaryWallets(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		acc1WIF = "L1MstxuD8SvS9HuFcV5oYzcdA1xX8D9bD9qPwg8fU5SSywYBecg3"
		acc2WIF = "L2iGxPvxbyWpYEbCZk2L3PgT7sCQaSDAbBC4MRLAjhs1s2JZ1xs5"
		acc3WIF = "L1xD2yiUyARX8DAkWa8qGpWpwjqW2u717VzUJyByk6s7HinhRVZv"
		acc4WIF = "L1ioz93TNt6Nu1aoMpZQ4zgdtgC8ZvJMC6pyHFkrovdR3SFwbn6n"
	)
	var walletDir = filepath.Join("..", "services", "notary", "testdata")
	acc1 := getAccountWithScrypt(t, acc1WIF, "one", naiveScrypt)
	acc2 := getAccountWithScrypt(t, acc2WIF, "one", naiveScrypt)
	acc3 := getAccountWithScrypt(t, acc3WIF, "four", naiveScrypt)
	createTestingWallet(t, filepath.Join(walletDir, "notary1.json"), acc1, acc2, acc3)

	acc4 := getAccountWithScrypt(t, acc4WIF, "two", naiveScrypt)
	createTestingWallet(t, filepath.Join(walletDir, "notary2.json"), acc4)
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

	acc1 := getTestingAccount(t, acc1WIF, "one")
	createTestingWallet(t, filepath.Join(walletDir, "oracle1.json"), acc1)

	acc2 := getTestingAccount(t, acc2WIF, "two")
	createTestingWallet(t, filepath.Join(walletDir, "oracle2.json"), acc2)
}

func TestRegenerateExamplesWallet(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletPath = "../../examples/my_wallet.json"
		acc1WIF    = "L46dn46AMZY7NQGZHemAdgcMabKon85eme45hgQkAUQBiRacY8MB"
	)

	acc1 := getNEP2Account(t, acc1WIF, "qwerty")
	acc1.Label = "my_account"
	createNEP2Wallet(t, walletPath, acc1)
}

func TestRegenerateCLITestwallet(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletPath = "../../cli/testdata/testwallet.json"
		accWIF     = "L23LrQNWELytYLvb5c6dXBDdF2DNPL9RRNWPqppv3roxacSnn8CN"
	)

	acc := getAccountWithScrypt(t, accWIF, "testpass", naiveScrypt)
	acc.Label = "kek"
	createTestingWallet(t, walletPath, acc)
}

func TestRegenerateCLITestwalletMulti(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletPath = "../../cli/testdata/testwallet_multi.json"
		accWIF1    = "L2NZQ84s8SuUfyJmtjs7J5a2pZFEVQVVRYDtNsQEyB4RjHnpkorr"
		pass1      = "one"
		accWIF2    = "L4pSqPmvbghcM8NY14CNSUzkK92VmSFLKNwAh9TqLqFhTZkxZgsP"
		pass2      = "two"
		accWIF3    = "L1eNtPQA8bALqvSgGMNpNR3EYFR15WeNdCZDuiwWYcQ6Q6FfuVro"
		pass3      = "three"
	)

	acc1 := getAccountWithScrypt(t, accWIF1, pass1, naiveScrypt)
	acc1.Label = pass1
	acc2 := getAccountWithScrypt(t, accWIF2, pass2, naiveScrypt)
	acc2.Label = pass2
	acc3 := getAccountWithScrypt(t, accWIF3, pass3, naiveScrypt)
	acc3.Label = pass3
	createTestingWallet(t, walletPath, acc1, acc2, acc3)
}

func TestRegenerateCLITestwallet_NEO3(t *testing.T) {
	if !regenerate {
		return
	}
	const walletPath = "../../cli/wallet/testdata/testwallet_NEO3.json"

	pubs := getKeys(t)
	acc1 := getNEP2Account(t, privnetWIFs[0], passwords[0])
	acc2 := getNEP2Account(t, privnetWIFs[0], passwords[0])
	require.NoError(t, acc2.ConvertMultisig(3, pubs))
	createNEP2Wallet(t, walletPath, acc1, acc2)
}

// createNEP2Wallet creates wallet with provided accounts and NEP2 scrypt parameters.
func createNEP2Wallet(t *testing.T, path string, accs ...*Account) {
	createWallet(t, path, keys.NEP2ScryptParams(), accs...)
}

// createTestingWallet creates wallet with provided accounts and scrypt parameters with
// low values which can be used for testing to save time on accounts decryption.
func createTestingWallet(t *testing.T, path string, accs ...*Account) {
	createWallet(t, path, naiveScrypt, accs...)
}

func createWallet(t *testing.T, path string, scryptParams keys.ScryptParams, accs ...*Account) {
	w, err := NewWallet(path)
	require.NoError(t, err)
	w.Scrypt = scryptParams

	if len(accs) == 0 {
		t.Fatal("provide at least 1 account")
	}
	for _, acc := range accs {
		w.AddAccount(acc)
	}
	require.NoError(t, w.SavePretty())
}

func TestRegenerateCLIWallet1_solo(t *testing.T) {
	if !regenerate {
		return
	}
	const (
		walletPath         = "../../cli/testdata/wallet1_solo.json"
		verifyWIF          = "L3W8gi36Y3KPqyR54VJaE1agH9yPvW2hALNZy1BerDwWce9P9xEy"
		verifyNEFPath      = "../../cli/smartcontract/testdata/verify.nef"
		verifyManifestPath = "../../cli/smartcontract/testdata/verify.manifest.json"
	)

	wif := privnetWIFs[0]
	acc1 := getAccountWithScrypt(t, wif, "one", naiveScrypt)
	acc1.Default = true
	acc2 := getAccountWithScrypt(t, wif, "one", naiveScrypt)
	require.NoError(t, acc2.ConvertMultisig(3, getKeys(t)))

	acc3 := getAccountWithScrypt(t, wif, "one", naiveScrypt)
	require.NoError(t, acc3.ConvertMultisig(1, keys.PublicKeys{getKeys(t)[0]}))

	acc4 := getAccountWithScrypt(t, verifyWIF, "pass", naiveScrypt) // deployed verify.go contract
	f, err := os.ReadFile(verifyNEFPath)
	require.NoError(t, err)
	nefFile, err := nef.FileFromBytes(f)
	require.NoError(t, err)
	manifestBytes, err := os.ReadFile(verifyManifestPath)
	require.NoError(t, err)
	m := &manifest.Manifest{}
	require.NoError(t, json.Unmarshal(manifestBytes, m))
	hash := state.CreateContractHash(acc3.PrivateKey().GetScriptHash(), nefFile.Checksum, m.Name)
	acc4.Address = address.Uint160ToString(hash)
	acc4.Contract = &Contract{
		Script:     nefFile.Script,
		Deployed:   true,
		Parameters: []ContractParam{},
	}
	acc4.Label = "verify"
	createTestingWallet(t, walletPath, acc1, acc2, acc3, acc4)
}
