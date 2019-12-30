package metrics

import (
	"net/http"
	"net/http/pprof"

	"go.uber.org/zap"
)

// PprofService https://golang.org/pkg/net/http/pprof/.
type PprofService Service

// NewPprofService created new service for gathering pprof metrics.
func NewPprofService(cfg Config, log *zap.Logger) *Service {
	if log == nil {
		return nil
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/debug/pprof/", pprof.Index)
	handler.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	handler.HandleFunc("/debug/pprof/profile", pprof.Profile)
	handler.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	handler.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &Service{
		Server: &http.Server{
			Addr:    cfg.Address + ":" + cfg.Port,
			Handler: handler,
		},
		config:      cfg,
		serviceType: "Pprof",
		log:         log.With(zap.String("service", "Pprof")),
	}
}
