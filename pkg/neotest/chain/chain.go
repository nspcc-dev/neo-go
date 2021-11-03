package chain

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const validatorWIF = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"

// committeeAcc is an account used to sign tx as a committee.
var committeeAcc *wallet.Account

func init() {
	committeeAcc, _ = wallet.NewAccountFromWIF(validatorWIF)
	pubs := keys.PublicKeys{committeeAcc.PrivateKey().PublicKey()}
	err := committeeAcc.ConvertMultisig(1, pubs)
	if err != nil {
		panic(err)
	}
}

// NewSingle creates new blockchain instance with a single validator and
// setups cleanup functions.
func NewSingle(t *testing.T) (*core.Blockchain, neotest.Signer) {
	protoCfg := config.ProtocolConfiguration{
		Magic:              netmode.UnitTestNet,
		SecondsPerBlock:    1,
		StandbyCommittee:   []string{hex.EncodeToString(committeeAcc.PrivateKey().PublicKey().Bytes())},
		ValidatorsCount:    1,
		VerifyBlocks:       true,
		VerifyTransactions: true,
	}

	st := storage.NewMemoryStore()
	log := zaptest.NewLogger(t)
	bc, err := core.NewBlockchain(st, protoCfg, log)
	require.NoError(t, err)
	go bc.Run()
	t.Cleanup(bc.Close)
	return bc, neotest.NewMultiSigner(committeeAcc)
}
