package metrics

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

// Service serves metrics.
type Service struct {
	*http.Server
	config      Config
	log         *zap.Logger
	serviceType string
}

// Config config used for monitoring.
type Config struct {
	Enabled bool   `yaml:"Enabled"`
	Address string `yaml:"Address"`
	Port    string `yaml:"Port"`
}

// Start runs http service with exposed endpoint on configured port.
func (ms *Service) Start() {
	if ms.config.Enabled {
		ms.log.Info("service is running", zap.String("endpoint", ms.Addr))
		err := ms.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			ms.log.Warn("service couldn't start on configured port")
		}
	} else {
		ms.log.Info("service hasn't started since it's disabled")
	}
}

// ShutDown stops service.
func (ms *Service) ShutDown() {
	ms.log.Info("shutting down service", zap.String("endpoint", ms.Addr))
	err := ms.Shutdown(context.Background())
	if err != nil {
		ms.log.Panic("can't shut down service")
	}
}
