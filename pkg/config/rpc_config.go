package config

import (
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// RPC is an RPC service configuration information.
	RPC struct {
		Address              string `yaml:"Address"`
		Enabled              bool   `yaml:"Enabled"`
		EnableCORSWorkaround bool   `yaml:"EnableCORSWorkaround"`
		// MaxGasInvoke is the maximum amount of GAS which
		// can be spent during an RPC call.
		MaxGasInvoke           fixedn.Fixed8 `yaml:"MaxGasInvoke"`
		MaxIteratorResultItems int           `yaml:"MaxIteratorResultItems"`
		MaxFindResultItems     int           `yaml:"MaxFindResultItems"`
		MaxNEP11Tokens         int           `yaml:"MaxNEP11Tokens"`
		MaxWebSocketClients    int           `yaml:"MaxWebSocketClients"`
		Port                   uint16        `yaml:"Port"`
		SessionEnabled         bool          `yaml:"SessionEnabled"`
		SessionExpirationTime  int           `yaml:"SessionExpirationTime"`
		SessionBackedByMPT     bool          `yaml:"SessionBackedByMPT"`
		SessionPoolSize        int           `yaml:"SessionPoolSize"`
		StartWhenSynchronized  bool          `yaml:"StartWhenSynchronized"`
		TLSConfig              TLS           `yaml:"TLSConfig"`
	}

	// TLS describes SSL/TLS configuration.
	TLS struct {
		Address  string `yaml:"Address"`
		CertFile string `yaml:"CertFile"`
		Enabled  bool   `yaml:"Enabled"`
		Port     uint16 `yaml:"Port"`
		KeyFile  string `yaml:"KeyFile"`
	}
)
