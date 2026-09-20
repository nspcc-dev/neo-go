package rpcbroadcaster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRPCClientRunWithUnavailableNode(t *testing.T) {
	closeCh := make(chan struct{})
	close(closeCh)
	client := &RPCClient{
		addr:        "://invalid",
		close:       closeCh,
		finished:    make(chan struct{}),
		responses:   make(chan []any),
		log:         zaptest.NewLogger(t),
		sendTimeout: time.Millisecond,
	}

	require.NotPanics(t, client.run)
}
