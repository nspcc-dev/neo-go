package rpcclient

import (
	"context"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/stretchr/testify/require"
)

func TestInternalClientClose(t *testing.T) {
	icl, err := NewInternal(context.TODO(), func(ctx context.Context, ch chan<- neorpc.Notification) func(*neorpc.Request) (*neorpc.Response, error) {
		return nil
	})
	require.NoError(t, err)
	icl.Close()
	require.NoError(t, icl.GetError())
}
