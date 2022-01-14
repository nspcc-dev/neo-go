package chain

import (
	"encoding/hex"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const singleValidatorWIF = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"

// committeeWIFs is a list of unencrypted WIFs sorted by public key.
var committeeWIFs = []string{
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	singleValidatorWIF,
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",

	// Provide 2 committee extra members so that committee address differs from
	// the validators one.
	"L1Tr1iq5oz1jaFaMXP21sHDkJYDDkuLtpvQ4wRf1cjKvJYvnvpAb",
	"Kz6XTUrExy78q8f4MjDHnwz8fYYyUE8iPXwPRAkHa3qN2JcHYm7e",
}

var (
	// committeeAcc is an account used to sign tx as a committee.
	committeeAcc *wallet.Account

	// multiCommitteeAcc contains committee accounts used in a multi-node setup.
	multiCommitteeAcc []*wallet.Account

	// multiValidatorAcc contains validator accounts used in a multi-node setup.
	multiValidatorAcc []*wallet.Account

	// standByCommittee contains list of committee public keys to use in config.
	standByCommittee []string
)

func init() {
	committeeAcc, _ = wallet.NewAccountFromWIF(singleValidatorWIF)
	pubs := keys.PublicKeys{committeeAcc.PrivateKey().PublicKey()}
	err := committeeAcc.ConvertMultisig(1, pubs)
	if err != nil {
		panic(err)
	}

	mc := smartcontract.GetMajorityHonestNodeCount(len(committeeWIFs))
	mv := smartcontract.GetDefaultHonestNodeCount(4)
	accs := make([]*wallet.Account, len(committeeWIFs))
	pubs = make(keys.PublicKeys, len(accs))
	for i := range committeeWIFs {
		accs[i], _ = wallet.NewAccountFromWIF(committeeWIFs[i])
		pubs[i] = accs[i].PrivateKey().PublicKey()
	}

	// Config entry must contain validators first in a specific order.
	standByCommittee = make([]string, len(pubs))
	standByCommittee[0] = hex.EncodeToString(pubs[2].Bytes())
	standByCommittee[1] = hex.EncodeToString(pubs[0].Bytes())
	standByCommittee[2] = hex.EncodeToString(pubs[3].Bytes())
	standByCommittee[3] = hex.EncodeToString(pubs[1].Bytes())
	standByCommittee[4] = hex.EncodeToString(pubs[4].Bytes())
	standByCommittee[5] = hex.EncodeToString(pubs[5].Bytes())

	multiValidatorAcc = make([]*wallet.Account, 4)
	sort.Sort(pubs[:4])

	sort.Slice(accs[:4], func(i, j int) bool {
		p1 := accs[i].PrivateKey().PublicKey()
		p2 := accs[j].PrivateKey().PublicKey()
		return p1.Cmp(p2) == -1
	})
	for i := range multiValidatorAcc {
		multiValidatorAcc[i] = wallet.NewAccountFromPrivateKey(accs[i].PrivateKey())
		err := multiValidatorAcc[i].ConvertMultisig(mv, pubs[:4])
		if err != nil {
			panic(err)
		}
	}

	multiCommitteeAcc = make([]*wallet.Account, len(committeeWIFs))
	sort.Sort(pubs)

	sort.Slice(accs, func(i, j int) bool {
		p1 := accs[i].PrivateKey().PublicKey()
		p2 := accs[j].PrivateKey().PublicKey()
		return p1.Cmp(p2) == -1
	})
	for i := range multiCommitteeAcc {
		multiCommitteeAcc[i] = wallet.NewAccountFromPrivateKey(accs[i].PrivateKey())
		err := multiCommitteeAcc[i].ConvertMultisig(mc, pubs)
		if err != nil {
			panic(err)
		}
	}
}

// NewSingle creates new blockchain instance with a single validator and
// setups cleanup functions.
func NewSingle(t *testing.T) (*core.Blockchain, neotest.Signer) {
	return NewSingleWithCustomConfig(t, nil)
}

// NewSingleWithCustomConfig creates new blockchain instance with custom protocol
// configuration and a single validator. It also setups cleanup functions.
func NewSingleWithCustomConfig(t *testing.T, f func(*config.ProtocolConfiguration)) (*core.Blockchain, neotest.Signer) {
	st := storage.NewMemoryStore()
	return NewSingleWithCustomConfigAndStore(t, f, st, true)
}

func NewSingleWithCustomConfigAndStore(t *testing.T, f func(cfg *config.ProtocolConfiguration), st storage.Store, run bool) (*core.Blockchain, neotest.Signer) {
	protoCfg := config.ProtocolConfiguration{
		Magic:              netmode.UnitTestNet,
		MaxTraceableBlocks: 1000, // We don't need a lot of traceable blocks for tests.
		SecondsPerBlock:    1,
		StandbyCommittee:   []string{hex.EncodeToString(committeeAcc.PrivateKey().PublicKey().Bytes())},
		ValidatorsCount:    1,
		VerifyBlocks:       true,
		VerifyTransactions: true,
	}
	if f != nil {
		f(&protoCfg)
	}
	log := zaptest.NewLogger(t)
	bc, err := core.NewBlockchain(st, protoCfg, log)
	require.NoError(t, err)
	if run {
		go bc.Run()
		t.Cleanup(bc.Close)
	}
	return bc, neotest.NewMultiSigner(committeeAcc)
}

// NewMulti creates new blockchain instance with 4 validators and 6 committee members.
// Second return value is for validator signer, third -- for committee.
func NewMulti(t *testing.T) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	return NewMultiWithCustomConfig(t, nil)
}

// NewMultiWithCustomConfig creates new blockchain instance with custom protocol
// configuration, 4 validators and 6 committee members. Second return value is
// for validator signer, third -- for committee.
func NewMultiWithCustomConfig(t *testing.T, f func(*config.ProtocolConfiguration)) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	protoCfg := config.ProtocolConfiguration{
		Magic:              netmode.UnitTestNet,
		SecondsPerBlock:    1,
		StandbyCommittee:   standByCommittee,
		ValidatorsCount:    4,
		VerifyBlocks:       true,
		VerifyTransactions: true,
	}
	if f != nil {
		f(&protoCfg)
	}

	st := storage.NewMemoryStore()
	log := zaptest.NewLogger(t)
	bc, err := core.NewBlockchain(st, protoCfg, log)
	require.NoError(t, err)
	go bc.Run()
	t.Cleanup(bc.Close)
	return bc, neotest.NewMultiSigner(multiValidatorAcc...), neotest.NewMultiSigner(multiCommitteeAcc...)
}
