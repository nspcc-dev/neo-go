package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// Service serves metrics.
type Service struct {
	http        []*http.Server
	config      config.BasicService
	log         *zap.Logger
	serviceType string
	started     *atomic.Bool
}

// NewService configures logger and returns new service instance.
func NewService(name string, httpServers []*http.Server, cfg config.BasicService, log *zap.Logger) *Service {
	return &Service{
		http:        httpServers,
		config:      cfg,
		serviceType: name,
		log:         log.With(zap.String("service", name)),
		started:     atomic.NewBool(false),
	}
}

// Start runs http service with the exposed endpoint on the configured port.
func (ms *Service) Start() error {
	if ms.config.Enabled {
		if !ms.started.CompareAndSwap(false, true) {
			ms.log.Info("service already started")
			return nil
		}
		for _, srv := range ms.http {
			ms.log.Info("starting service", zap.String("endpoint", srv.Addr))

			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("failed to listen on %s: %w", srv.Addr, err)
			}
			srv.Addr = ln.Addr().String() // set Addr to the actual address

			go func(s *http.Server) {
				err = s.Serve(ln)
				if !errors.Is(err, http.ErrServerClosed) {
					ms.log.Error("failed to start service", zap.String("endpoint", s.Addr), zap.Error(err))
				}
			}(srv)
		}
	} else {
		ms.log.Info("service hasn't started since it's disabled")
	}
	return nil
}

// ShutDown stops the service.
func (ms *Service) ShutDown() {
	if !ms.config.Enabled {
		return
	}
	if !ms.started.CompareAndSwap(true, false) {
		return
	}
	for _, srv := range ms.http {
		ms.log.Info("shutting down service", zap.String("endpoint", srv.Addr))
		err := srv.Shutdown(context.Background())
		if err != nil {
			ms.log.Error("can't shut service down", zap.String("endpoint", srv.Addr), zap.Error(err))
		}
	}
}
