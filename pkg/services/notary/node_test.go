package notary

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func getTestNotary(t *testing.T, bc blockchainer.Blockchainer, walletPath, pass string) (*wallet.Account, *Notary, *mempool.Pool) {
	mainCfg := config.P2PNotary{
		Enabled: true,
		UnlockWallet: config.Wallet{
			Path:     walletPath,
			Password: pass,
		},
	}
	mp := mempool.New(10, 1, true)
	cfg := Config{
		MainCfg: mainCfg,
		Chain:   bc,
		Log:     zaptest.NewLogger(t),
	}
	ntr, err := NewNotary(cfg, netmode.UnitTestNet, mp, nil)
	require.NoError(t, err)

	w, err := wallet.NewWalletFromFile(walletPath)
	require.NoError(t, err)
	require.NoError(t, w.Accounts[0].Decrypt(pass))
	return w.Accounts[0], ntr, mp
}

func TestUpdateNotaryNodes(t *testing.T) {
	bc := fakechain.NewFakeChain()
	acc, ntr, _ := getTestNotary(t, bc, "./testdata/notary1.json", "one")
	randomKey, err := keys.NewPrivateKey()
	require.NoError(t, err)
	// currAcc is nil before UpdateNotaryNodes call
	require.Nil(t, ntr.currAccount)
	// set account for the first time
	ntr.UpdateNotaryNodes(keys.PublicKeys{acc.PrivateKey().PublicKey()})
	require.Equal(t, acc, ntr.currAccount)

	t.Run("account is already set", func(t *testing.T) {
		ntr.UpdateNotaryNodes(keys.PublicKeys{acc.PrivateKey().PublicKey(), randomKey.PublicKey()})
		require.Equal(t, acc, ntr.currAccount)
	})

	t.Run("another account from the same wallet", func(t *testing.T) {
		t.Run("good config password", func(t *testing.T) {
			w, err := wallet.NewWalletFromFile("./testdata/notary1.json")
			require.NoError(t, err)
			require.NoError(t, w.Accounts[1].Decrypt("one"))
			ntr.UpdateNotaryNodes(keys.PublicKeys{w.Accounts[1].PrivateKey().PublicKey()})
			require.Equal(t, w.Accounts[1], ntr.currAccount)
		})
		t.Run("bad config password", func(t *testing.T) {
			w, err := wallet.NewWalletFromFile("./testdata/notary1.json")
			require.NoError(t, err)
			require.NoError(t, w.Accounts[2].Decrypt("four"))
			ntr.UpdateNotaryNodes(keys.PublicKeys{w.Accounts[2].PrivateKey().PublicKey()})
			require.Nil(t, ntr.currAccount)
		})
	})

	t.Run("unknown account", func(t *testing.T) {
		ntr.UpdateNotaryNodes(keys.PublicKeys{randomKey.PublicKey()})
		require.Nil(t, ntr.currAccount)
	})
}
