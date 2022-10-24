package invoker_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
)

func TestRPCInvokerRPCClientCompat(t *testing.T) {
	_ = invoker.RPCInvoke(&rpcclient.Client{})
	_ = invoker.RPCInvoke(&rpcclient.WSClient{})
	_ = invoker.RPCInvokeHistoric(&rpcclient.Client{})
	_ = invoker.RPCInvokeHistoric(&rpcclient.WSClient{})
	_ = invoker.RPCSessions(&rpcclient.WSClient{})
}
