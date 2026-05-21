package network

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestP2PCommandsMetricNaming(t *testing.T) {
	const cmd = CMDPing

	addCmdTimeMetric(cmd, time.Second)

	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var foundMetric, foundHistogramType, foundCommandLabel bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "neogo_p2p_commands_time":
			foundMetric = true
			foundHistogramType = mf.GetType().String() == "HISTOGRAM"
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "command" && l.GetValue() == strings.ToLower(cmd.String()) {
						require.GreaterOrEqual(t, m.GetHistogram().GetSampleCount(), uint64(1))
						foundCommandLabel = true
					}
				}
			}
		default:
			if strings.HasPrefix(mf.GetName(), "neogo_p2p_") && strings.HasSuffix(mf.GetName(), "_time") {
				t.Fatalf("obsolete per-command metric %q is still registered", mf.GetName())
			}
		}
	}

	require.True(t, foundMetric)
	require.True(t, foundHistogramType)
	require.True(t, foundCommandLabel)
}
