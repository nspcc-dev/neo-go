package chain

import (
	"encoding/hex"
	"slices"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	// MaxTraceableBlocks is the default MaxTraceableBlocks setting used for test chains.
	// We don't need a lot of traceable blocks for tests.
	MaxTraceableBlocks = 1000

	// TimePerBlock is the default TimePerBlock setting used for test chains (1s).
	// Usually blocks are created by tests bypassing this setting.
	TimePerBlock = time.Second
)

const singleValidatorWIF = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"

// committeeWIFs is a list of unencrypted WIFs sorted by the public key.
var committeeWIFs = []string{
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	singleValidatorWIF,
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",

	// Provide 2 committee extra members so that the committee address differs from
	// the validators one.
	"L1Tr1iq5oz1jaFaMXP21sHDkJYDDkuLtpvQ4wRf1cjKvJYvnvpAb",
	"Kz6XTUrExy78q8f4MjDHnwz8fYYyUE8iPXwPRAkHa3qN2JcHYm7e",
}

var (
	// committeeAcc is an account used to sign a tx as a committee.
	committeeAcc *wallet.Account

	// multiCommitteeAcc contains committee accounts used in a multi-node setup.
	multiCommitteeAcc []*wallet.Account

	// multiValidatorAcc contains validator accounts used in a multi-node setup.
	multiValidatorAcc []*wallet.Account

	// standByCommittee contains a list of committee public keys to use in config.
	standByCommittee []string
)

// Options contains parameters to customize parameters of the test chain.
type Options struct {
	// Logger allows to customize logging performed by the test chain.
	// If Logger is not set, zaptest.Logger will be used with default configuration.
	Logger *zap.Logger
	// BlockchainConfigHook function is sort of visitor pattern for blockchain configuration.
	// It takes in the default configuration as an argument and can perform any adjustments in it.
	BlockchainConfigHook func(*config.Blockchain)
	// Store allows to customize storage for blockchain data.
	// If Store is not set, MemoryStore is used by default.
	Store storage.Store
	// If SkipRun is false, then the blockchain will be started (if its' construction
	// has succeeded) and will be registered for cleanup when the test completes.
	// If SkipRun is true, it is caller's responsibility to call Run before using
	// the chain and to properly Close the chain when done.
	SkipRun bool
}

func init() {
	committeeAcc, _ = wallet.NewAccountFromWIF(singleValidatorWIF)
	pubs := keys.PublicKeys{committeeAcc.PublicKey()}
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
		pubs[i] = accs[i].PublicKey()
	}

	// Config entry must contain validators first in a specific order.
	standByCommittee = make([]string, len(pubs))
	standByCommittee[0] = pubs[2].StringCompressed()
	standByCommittee[1] = pubs[0].StringCompressed()
	standByCommittee[2] = pubs[3].StringCompressed()
	standByCommittee[3] = pubs[1].StringCompressed()
	standByCommittee[4] = pubs[4].StringCompressed()
	standByCommittee[5] = pubs[5].StringCompressed()

	multiValidatorAcc = make([]*wallet.Account, 4)
	slices.SortFunc(pubs[:4], (*keys.PublicKey).Cmp)

	slices.SortFunc(accs[:4], func(a, b *wallet.Account) int {
		pa := a.PublicKey()
		pb := b.PublicKey()
		return pa.Cmp(pb)
	})
	for i := range multiValidatorAcc {
		multiValidatorAcc[i] = wallet.NewAccountFromPrivateKey(accs[i].PrivateKey())
		err := multiValidatorAcc[i].ConvertMultisig(mv, pubs[:4])
		if err != nil {
			panic(err)
		}
	}

	multiCommitteeAcc = make([]*wallet.Account, len(committeeWIFs))
	slices.SortFunc(pubs, (*keys.PublicKey).Cmp)

	slices.SortFunc(accs, func(a, b *wallet.Account) int {
		pa := a.PublicKey()
		pb := b.PublicKey()
		return pa.Cmp(pb)
	})
	for i := range multiCommitteeAcc {
		multiCommitteeAcc[i] = wallet.NewAccountFromPrivateKey(accs[i].PrivateKey())
		err := multiCommitteeAcc[i].ConvertMultisig(mc, pubs)
		if err != nil {
			panic(err)
		}
	}
}

// NewSingle creates a new blockchain instance with a single validator and
// setups cleanup functions. The configuration used is with netmode.UnitTestNet
// magic and TimePerBlock/MaxTraceableBlocks options defined by constants in
// this package. MemoryStore is used as the backend storage, so all of the chain
// contents is always in RAM. The Signer returned is the validator (and the committee at
// the same time).
func NewSingle(t testing.TB) (*core.Blockchain, neotest.Signer) {
	return NewSingleWithCustomConfig(t, nil)
}

// NewSingleWithCustomConfig is similar to NewSingle, but allows to override the
// default configuration.
func NewSingleWithCustomConfig(t testing.TB, f func(*config.Blockchain)) (*core.Blockchain, neotest.Signer) {
	return NewSingleWithCustomConfigAndStore(t, f, nil, true)
}

// NewSingleWithCustomConfigAndStore is similar to NewSingleWithCustomConfig, but
// also allows to override backend Store being used. The last parameter controls if
// Run method is called on the Blockchain instance. If not, it is its caller's
// responsibility to do that before using the chain and
// to properly Close the chain when done.
func NewSingleWithCustomConfigAndStore(t testing.TB, f func(cfg *config.Blockchain), st storage.Store, run bool) (*core.Blockchain, neotest.Signer) {
	return NewSingleWithOptions(t, &Options{
		BlockchainConfigHook: f,
		Store:                st,
		SkipRun:              !run,
	})
}

// NewSingleWithOptions creates a new blockchain instance with a single validator
// using specified options.
func NewSingleWithOptions(t testing.TB, options *Options) (*core.Blockchain, neotest.Signer) {
	if options == nil {
		options = &Options{}
	}

	cfg := config.Blockchain{
		ProtocolConfiguration: config.ProtocolConfiguration{
			Magic:              netmode.UnitTestNet,
			MaxTraceableBlocks: MaxTraceableBlocks,
			TimePerBlock:       TimePerBlock,
			StandbyCommittee:   []string{hex.EncodeToString(committeeAcc.PublicKey().Bytes())},
			ValidatorsCount:    1,
			VerifyTransactions: true,
		},
	}
	if options.BlockchainConfigHook != nil {
		options.BlockchainConfigHook(&cfg)
	}

	store := options.Store
	if store == nil {
		store = storage.NewMemoryStore()
	}

	logger := options.Logger
	if logger == nil {
		logger = zaptest.NewLogger(t)
	}

	bc, err := core.NewBlockchain(store, cfg, logger)
	require.NoError(t, err)
	if !options.SkipRun {
		go bc.Run()
		t.Cleanup(bc.Close)
	}
	return bc, neotest.NewMultiSigner(committeeAcc)
}

// NewMulti creates a new blockchain instance with four validators and six
// committee members. Otherwise, it does not differ much from NewSingle. The
// second value returned contains the validators Signer, the third -- the committee one.
func NewMulti(t testing.TB) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	return NewMultiWithCustomConfig(t, nil)
}

// NewMultiWithCustomConfig is similar to NewMulti, except it allows to override the
// default configuration.
func NewMultiWithCustomConfig(t testing.TB, f func(*config.Blockchain)) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	return NewMultiWithCustomConfigAndStore(t, f, nil, true)
}

// NewMultiWithCustomConfigAndStore is similar to NewMultiWithCustomConfig, but
// also allows to override backend Store being used. The last parameter controls if
// Run method is called on the Blockchain instance. If not, it is its caller's
// responsibility to do that before using the chain and
// to properly Close the chain when done.
func NewMultiWithCustomConfigAndStore(t testing.TB, f func(*config.Blockchain), st storage.Store, run bool) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	bc, validator, committee, err := NewMultiWithCustomConfigAndStoreNoCheck(t, f, st)
	require.NoError(t, err)
	if run {
		go bc.Run()
		t.Cleanup(bc.Close)
	}
	return bc, validator, committee
}

// NewMultiWithOptions creates a new blockchain instance with four validators and six
// committee members. Otherwise, it does not differ much from NewSingle. The
// second value returned contains the validators Signer, the third -- the committee one.
func NewMultiWithOptions(t testing.TB, options *Options) (*core.Blockchain, neotest.Signer, neotest.Signer) {
	bc, validator, committee, err := NewMultiWithOptionsNoCheck(t, options)
	require.NoError(t, err)
	return bc, validator, committee
}

// NewMultiWithCustomConfigAndStoreNoCheck is similar to NewMultiWithCustomConfig,
// but do not perform Blockchain run and do not check Blockchain constructor error.
func NewMultiWithCustomConfigAndStoreNoCheck(t testing.TB, f func(*config.Blockchain), st storage.Store) (*core.Blockchain, neotest.Signer, neotest.Signer, error) {
	return NewMultiWithOptionsNoCheck(t, &Options{
		BlockchainConfigHook: f,
		Store:                st,
		SkipRun:              true,
	})
}

// NewMultiWithOptionsNoCheck is similar to NewMultiWithOptions, but does not verify blockchain constructor error.
// It will start blockchain only if construction has completed successfully.
func NewMultiWithOptionsNoCheck(t testing.TB, options *Options) (*core.Blockchain, neotest.Signer, neotest.Signer, error) {
	if options == nil {
		options = &Options{}
	}

	cfg, err := config.Load(config.DefaultConfigPath, netmode.UnitTestNet)
	if err != nil {
		return nil, nil, nil, err
	}
	bcfg := cfg.Blockchain()
	if options.BlockchainConfigHook != nil {
		options.BlockchainConfigHook(&bcfg)
	}

	store := options.Store
	if store == nil {
		store = storage.NewMemoryStore()
	}

	logger := options.Logger
	if logger == nil {
		logger = zaptest.NewLogger(t)
	}

	bc, err := core.NewBlockchain(store, bcfg, logger)
	if err == nil && !options.SkipRun {
		go bc.Run()
		t.Cleanup(bc.Close)
	}
	return bc, neotest.NewMultiSigner(multiValidatorAcc...), neotest.NewMultiSigner(multiCommitteeAcc...), err
}
