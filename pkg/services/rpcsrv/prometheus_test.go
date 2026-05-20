package rpcsrv

import (
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestRPCRequestsMetricNaming(t *testing.T) {
	const api = "__test_api__"

	addReqMetric(api)

	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var rpcRequestsTotal *dto.MetricFamily
	for _, mf := range mfs {
		switch mf.GetName() {
		case "neogo_rpc_requests_total":
			rpcRequestsTotal = mf
		default:
			if strings.HasPrefix(mf.GetName(), "neogo_rpc_") && strings.HasSuffix(mf.GetName(), "_time") {
				t.Fatalf("obsolete per-method metric %q is still registered", mf.GetName())
			}
		}
	}

	require.NotNil(t, rpcRequestsTotal)
	require.Equal(t, dto.MetricType_COUNTER, rpcRequestsTotal.GetType())

	var found bool
	for _, m := range rpcRequestsTotal.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "api" && l.GetValue() == api {
				require.GreaterOrEqual(t, m.GetCounter().GetValue(), 1.0)
				found = true
			}
		}
	}
	require.True(t, found)
}
