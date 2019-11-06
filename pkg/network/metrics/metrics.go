package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Service serves metrics provided by prometheus.
type Service struct {
	*http.Server
	config PrometheusConfig
}

// PrometheusConfig config for Prometheus used for monitoring.
// Additional information about Prometheus could be found here: https://prometheus.io/docs/guides/go-application.
type PrometheusConfig struct {
	Enabled bool   `yaml:"Enabled"`
	Address string `yaml:"Address"`
	Port    string `yaml:"Port"`
}

// NewMetricsService created new service for gathering metrics.
func NewMetricsService(cfg PrometheusConfig) *Service {
	return &Service{
		&http.Server{
			Addr:   cfg.Address + ":" + cfg.Port,
			Handler: promhttp.Handler(),
		}, cfg,
	}
}

// Start runs http service with exposed `/metrics` endpoint on configured port.
func (ms *Service) Start() {
	if ms.config.Enabled {
		err := ms.ListenAndServe()
		if err != nil {
			log.WithFields(log.Fields{
				"endpoint": ms.Addr,
			}).Info("metrics service up and running")
		} else {
			log.Warn("metrics service couldn't start on configured port")
		}
	} else {
		log.Infof("metrics service hasn't started since it's disabled")
	}
}

// ShutDown stops service.
func (ms *Service) ShutDown() {
	log.WithFields(log.Fields{
		"endpoint": ms.config.Port,
	}).Info("shutting down monitoring-service")
	err := ms.Shutdown(context.Background())
	if err != nil {
		log.Fatal("can't shut down monitoring service")
	}
}
