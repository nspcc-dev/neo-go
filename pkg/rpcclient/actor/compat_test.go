package actor_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
)

func TestRPCActorRPCClientCompat(t *testing.T) {
	_ = actor.RPCActor(&rpcclient.WSClient{})
	_ = actor.RPCActor(&rpcclient.Client{})
}

func TestRPCWaiterRPCClientCompat(t *testing.T) {
	_ = actor.RPCPollingWaiter(&rpcclient.Client{})
	_ = actor.RPCPollingWaiter(&rpcclient.WSClient{})
	_ = actor.RPCEventWaiter(&rpcclient.WSClient{})
}
