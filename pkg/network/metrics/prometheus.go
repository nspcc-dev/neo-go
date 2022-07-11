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

	return &Service{
		Server: &http.Server{
			Addr:    cfg.Address + ":" + cfg.Port,
			Handler: promhttp.Handler(),
		},
		config:      cfg,
		serviceType: "Prometheus",
		log:         log.With(zap.String("service", "Prometheus")),
	}
}
