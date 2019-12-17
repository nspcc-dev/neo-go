package metrics

import (
	"net/http"
	"net/http/pprof"
)

// PprofService https://golang.org/pkg/net/http/pprof/.
type PprofService Service

// NewPprofService created new service for gathering pprof metrics.
func NewPprofService(cfg Config) *Service {
	handler := http.NewServeMux()
	handler.HandleFunc("/debug/pprof/", pprof.Index)
	handler.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	handler.HandleFunc("/debug/pprof/profile", pprof.Profile)
	handler.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	handler.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &Service{
		&http.Server{
			Addr:    cfg.Address + ":" + cfg.Port,
			Handler: handler,
		}, cfg, "Pprof",
	}
}
