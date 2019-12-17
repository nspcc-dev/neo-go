package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusService https://prometheus.io/docs/guides/go-application.
type PrometheusService Service

// NewPrometheusService creates new service for gathering prometheus metrics.
func NewPrometheusService(cfg Config) *Service {
	return &Service{
		&http.Server{
			Addr:    cfg.Address + ":" + cfg.Port,
			Handler: promhttp.Handler(),
		}, cfg, "Prometheus",
	}
}
