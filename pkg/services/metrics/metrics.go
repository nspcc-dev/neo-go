package metrics

import (
	"context"
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"go.uber.org/zap"
)

// Service serves metrics.
type Service struct {
	*http.Server
	config      config.BasicService
	log         *zap.Logger
	serviceType string
}

// Start runs http service with the exposed endpoint on the configured port.
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

// ShutDown stops the service.
func (ms *Service) ShutDown() {
	if !ms.config.Enabled {
		return
	}
	ms.log.Info("shutting down service", zap.String("endpoint", ms.Addr))
	err := ms.Shutdown(context.Background())
	if err != nil {
		ms.log.Error("can't shut service down", zap.Error(err))
	}
}
