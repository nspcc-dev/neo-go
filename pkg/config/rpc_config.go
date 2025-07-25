package config

import (
	"fmt"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// RPC is an RPC service configuration information.
	RPC struct {
		BasicService         `yaml:",inline"`
		EnableCORSWorkaround bool `yaml:"EnableCORSWorkaround"`
		// MaxGasInvoke is the maximum amount of GAS which
		// can be spent during an RPC call.
		MaxGasInvoke                fixedn.Fixed8 `yaml:"MaxGasInvoke"`
		MaxIteratorResultItems      int           `yaml:"MaxIteratorResultItems"`
		MaxFindResultItems          int           `yaml:"MaxFindResultItems"`
		MaxFindStorageResultItems   int           `yaml:"MaxFindStoragePageSize"`
		MaxNEP11Tokens              int           `yaml:"MaxNEP11Tokens"`
		MaxRequestBodyBytes         int           `yaml:"MaxRequestBodyBytes"`
		MaxRequestHeaderBytes       int           `yaml:"MaxRequestHeaderBytes"`
		MaxWebSocketClients         int           `yaml:"MaxWebSocketClients"`
		MaxWebSocketFeeds           int           `yaml:"MaxWebSocketFeeds"`
		MempoolSubscriptionsEnabled bool          `yaml:"MempoolSubscriptionsEnabled"`
		SessionEnabled              bool          `yaml:"SessionEnabled"`
		SessionExpansionEnabled     bool          `yaml:"SessionExpansionEnabled"`
		// Deprecated: Use SessionLifetime instead.
		SessionExpirationTime int           `yaml:"SessionExpirationTime"`
		SessionLifetime       time.Duration `yaml:"SessionLifetime"`
		SessionBackedByMPT    bool          `yaml:"SessionBackedByMPT"`
		SessionPoolSize       int           `yaml:"SessionPoolSize"`
		StartWhenSynchronized bool          `yaml:"StartWhenSynchronized"`
		TLSConfig             TLS           `yaml:"TLSConfig"`
	}

	// TLS describes SSL/TLS configuration.
	TLS struct {
		BasicService `yaml:",inline"`
		CertFile     string `yaml:"CertFile"`
		KeyFile      string `yaml:"KeyFile"`
	}
)

// Validate checks RPC for internal consistency. It returns an error if the
// configuration is invalid.
func (cfg *RPC) Validate() error {
	if cfg.SessionExpansionEnabled && !cfg.SessionEnabled {
		return fmt.Errorf("SessionExpansionEnabled requires SessionEnabled")
	}
	if cfg.SessionExpirationTime > 0 && cfg.SessionLifetime > 0 {
		return fmt.Errorf("only one of SessionExpirationTime or SessionLifetime can be set")
	}
	return nil
}
