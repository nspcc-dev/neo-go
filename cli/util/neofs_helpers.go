package util

import (
	"context"
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/urfave/cli/v2"
)

// poolWrapper wraps a NeoFS pool to adapt its Close method to return an error.
type poolWrapper struct {
	*pool.Pool
}

// Close closes the pool and returns nil.
func (p poolWrapper) Close() error {
	p.Pool.Close()
	return nil
}

func initNeoFSPool(ctx *cli.Context, acc *wallet.Account) (user.Signer, poolWrapper, error) {
	rpcNeoFS := ctx.StringSlice(options.NeoFSRPCEndpointFlag)
	signer := user.NewAutoIDSignerRFC6979(acc.PrivateKey().PrivateKey)

	params := pool.DefaultOptions()
	params.SetHealthcheckTimeout(neofs.DefaultHealthcheckTimeout)
	params.SetNodeDialTimeout(neofs.DefaultDialTimeout)
	params.SetNodeStreamTimeout(neofs.DefaultStreamTimeout)
	p, err := pool.New(pool.NewFlatNodeParams(rpcNeoFS), signer, params)
	if err != nil {
		return nil, poolWrapper{}, fmt.Errorf("failed to create NeoFS pool: %w", err)
	}
	pWrapper := poolWrapper{p}
	if err = pWrapper.Dial(context.Background()); err != nil {
		return nil, poolWrapper{}, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}
	return signer, pWrapper, nil
}

// getContainer gets container by ID and checks its magic.
func getContainer(ctx *cli.Context, p poolWrapper, expectedMagic string, maxRetries uint, debug bool) (cid.ID, error) {
	var (
		containerObj   container.Container
		err            error
		containerIDStr = ctx.String("container")
	)
	var containerID cid.ID
	if err = containerID.DecodeString(containerIDStr); err != nil {
		return containerID, fmt.Errorf("failed to decode container ID: %w", err)
	}
	err = retry(func() error {
		containerObj, err = p.ContainerGet(ctx.Context, containerID, client.PrmContainerGet{})
		return err
	}, maxRetries, debug)
	if err != nil {
		return containerID, fmt.Errorf("failed to get container: %w", err)
	}
	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != expectedMagic {
		return containerID, fmt.Errorf("container magic mismatch: expected %s, got %s", expectedMagic, containerMagic)
	}
	return containerID, nil
}
