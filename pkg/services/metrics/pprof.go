package metrics

import (
	"net/http"
	"net/http/pprof"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"go.uber.org/zap"
)

// PprofService https://golang.org/pkg/net/http/pprof/.
type PprofService Service

// NewPprofService creates a new service for gathering pprof metrics.
func NewPprofService(cfg config.BasicService, log *zap.Logger) *Service {
	if log == nil {
		return nil
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/debug/pprof/", pprof.Index)
	handler.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	handler.HandleFunc("/debug/pprof/profile", pprof.Profile)
	handler.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	handler.HandleFunc("/debug/pprof/trace", pprof.Trace)

	addrs := cfg.Addresses
	srvs := make([]*http.Server, len(addrs))
	for i, addr := range addrs {
		srvs[i] = &http.Server{
			Addr:    addr,
			Handler: handler,
		}
	}
	return NewService("Pprof", srvs, cfg, log)
}
