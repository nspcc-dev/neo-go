package metrics

import (
	"context"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Service serves metrics.
type Service struct {
	*http.Server
	config      Config
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
		log.WithFields(log.Fields{
			"endpoint": ms.Addr,
			"service": ms.serviceType,
		}).Info("service running")
		err := ms.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Warnf("%s service couldn't start on configured port", ms.serviceType)
		}
	} else {
		log.Infof("%s service hasn't started since it's disabled", ms.serviceType)
	}
}

// ShutDown stops service.
func (ms *Service) ShutDown() {
	log.WithFields(log.Fields{
		"endpoint": ms.Addr,
		"service": ms.serviceType,
	}).Info("shutting down service")
	err := ms.Shutdown(context.Background())
	if err != nil {
		log.Fatalf("can't shut down %s service",  ms.serviceType)
	}
}
