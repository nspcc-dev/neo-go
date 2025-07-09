package rpcclient_test

import (
	"context"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/stretchr/testify/require"
)

func TestInternalClientClose(t *testing.T) {
	icl, err := rpcclient.NewInternal(context.TODO(), func(ctx context.Context, ch chan<- neorpc.Notification) func(*neorpc.Request) (*neorpc.Response, error) {
		return nil
	})
	require.NoError(t, err)
	icl.Close()
	require.NoError(t, icl.GetError())
}

func TestInternalClientCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	bc, rpcSrv, netSrv := testcli.NewTestChain(t, func(c *config.Config) {
		c.ApplicationConfiguration.Consensus.UnlockWallet.Path = "../../cli/testdata/wallet1_solo.json"
	}, true)
	t.Cleanup(func() {
		netSrv.Shutdown()
		rpcSrv.Shutdown()
		bc.Close()
	})
	icl, err := rpcclient.NewInternal(ctx, rpcSrv.RegisterLocal)
	require.NoError(t, err)
	cancel()
	icl.Close()
	require.NoError(t, icl.GetError())
}
