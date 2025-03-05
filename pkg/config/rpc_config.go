package config

import (
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// RPC is an RPC service configuration information.
	RPC struct {
		BasicService         `yaml:",inline"`
		EnableCORSWorkaround bool `yaml:"EnableCORSWorkaround"`
		// MaxGasInvoke is the maximum amount of GAS which
		// can be spent during an RPC call.
		MaxGasInvoke              fixedn.Fixed8 `yaml:"MaxGasInvoke"`
		MaxIteratorResultItems    int           `yaml:"MaxIteratorResultItems"`
		MaxFindResultItems        int           `yaml:"MaxFindResultItems"`
		MaxFindStorageResultItems int           `yaml:"MaxFindStoragePageSize"`
		MaxNEP11Tokens            int           `yaml:"MaxNEP11Tokens"`
		MaxRequestBodyBytes       int           `yaml:"MaxRequestBodyBytes"`
		MaxRequestHeaderBytes     int           `yaml:"MaxRequestHeaderBytes"`
		MaxWebSocketClients       int           `yaml:"MaxWebSocketClients"`
		MaxWebSocketFeeds         int           `yaml:"MaxWebSocketFeeds"`
		SessionEnabled            bool          `yaml:"SessionEnabled"`
		SessionExpirationTime     int           `yaml:"SessionExpirationTime"`
		SessionBackedByMPT        bool          `yaml:"SessionBackedByMPT"`
		SessionPoolSize           int           `yaml:"SessionPoolSize"`
		StartWhenSynchronized     bool          `yaml:"StartWhenSynchronized"`
		TLSConfig                 TLS           `yaml:"TLSConfig"`
	}

	// TLS describes SSL/TLS configuration.
	TLS struct {
		BasicService `yaml:",inline"`
		CertFile     string `yaml:"CertFile"`
		KeyFile      string `yaml:"KeyFile"`
	}
)
