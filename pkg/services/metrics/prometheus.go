package metrics

import (
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// PrometheusService https://prometheus.io/docs/guides/go-application.
type PrometheusService Service

// NewPrometheusService creates a new service for gathering prometheus metrics.
func NewPrometheusService(cfg config.BasicService, log *zap.Logger) *Service {
	if log == nil {
		return nil
	}

	addrs := cfg.GetAddresses()
	srvs := make([]*http.Server, len(addrs))
	for i, addr := range addrs {
		srvs[i] = &http.Server{
			Addr:    addr,
			Handler: promhttp.Handler(), // share metrics between multiple prometheus handlers
		}
	}
	return NewService("Prometheus", srvs, cfg, log)
}
