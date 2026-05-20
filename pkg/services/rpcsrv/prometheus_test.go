package rpcsrv

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestRPCRequestsMetricNaming(t *testing.T) {
	const api = "__test_api__"

	addReqMetric(api)

	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var foundMetric, foundCounterType, foundAPILabel bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "neogo_rpc_requests_total":
			foundMetric = true
			foundCounterType = mf.GetType().String() == "COUNTER"
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "api" && l.GetValue() == api {
						require.GreaterOrEqual(t, m.GetCounter().GetValue(), 1.0)
						foundAPILabel = true
					}
				}
			}
		default:
			if strings.HasPrefix(mf.GetName(), "neogo_rpc_") && strings.HasSuffix(mf.GetName(), "_time") {
				t.Fatalf("obsolete per-method metric %q is still registered", mf.GetName())
			}
		}
	}

	require.True(t, foundMetric)
	require.True(t, foundCounterType)
	require.True(t, foundAPILabel)
}
