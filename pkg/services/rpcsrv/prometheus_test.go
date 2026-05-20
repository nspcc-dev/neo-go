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

	var foundCounterMetric, foundCounterType, foundCounterAPILabel bool
	var foundHistogramMetric, foundHistogramType, foundHistogramAPILabel bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "neogo_rpc_requests_total":
			foundCounterMetric = true
			foundCounterType = mf.GetType().String() == "COUNTER"
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "api" && l.GetValue() == api {
						require.GreaterOrEqual(t, m.GetCounter().GetValue(), 1.0)
						foundCounterAPILabel = true
					}
				}
			}
		case "neogo_rpc_request_duration_seconds":
			foundHistogramMetric = true
			foundHistogramType = mf.GetType().String() == "HISTOGRAM"
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "api" && l.GetValue() == api {
						require.GreaterOrEqual(t, m.GetHistogram().GetSampleCount(), uint64(1))
						foundHistogramAPILabel = true
					}
				}
			}
		default:
			if strings.HasPrefix(mf.GetName(), "neogo_rpc_") && strings.HasSuffix(mf.GetName(), "_time") {
				t.Fatalf("obsolete per-method metric %q is still registered", mf.GetName())
			}
		}
	}

	require.True(t, foundCounterMetric)
	require.True(t, foundCounterType)
	require.True(t, foundCounterAPILabel)
	require.True(t, foundHistogramMetric)
	require.True(t, foundHistogramType)
	require.True(t, foundHistogramAPILabel)
}
