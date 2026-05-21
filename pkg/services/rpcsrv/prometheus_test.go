package rpcsrv

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestRPCRequestsMetricNaming(t *testing.T) {
	const api = "__test_api__"

	addReqMetric(api, time.Second)

	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var foundMetric, foundHistogramType, foundAPILabel bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "neogo_rpc_requests_time":
			foundMetric = true
			foundHistogramType = mf.GetType().String() == "HISTOGRAM"
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "api" && l.GetValue() == api {
						require.GreaterOrEqual(t, m.GetHistogram().GetSampleCount(), uint64(1))
						foundAPILabel = true
					}
				}
			}
		default:
			if strings.HasPrefix(mf.GetName(), "neogo_rpc_") && strings.HasSuffix(mf.GetName(), "_time") {
				t.Fatalf("obsolete per-method metric %q is still registered", mf.GetName())
			}
			if mf.GetName() == "neogo_rpc_requests_total" {
				t.Fatalf("obsolete counter metric %q is still registered", mf.GetName())
			}
		}
	}

	require.True(t, foundMetric)
	require.True(t, foundHistogramType)
	require.True(t, foundAPILabel)
}
