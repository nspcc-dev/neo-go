package rpc

import (
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// Config is an RPC service configuration information.
	Config struct {
		Address              string `yaml:"Address"`
		Enabled              bool   `yaml:"Enabled"`
		EnableCORSWorkaround bool   `yaml:"EnableCORSWorkaround"`
		// MaxGasInvoke is a maximum amount of gas which
		// can be spent during RPC call.
		MaxGasInvoke           fixedn.Fixed8 `yaml:"MaxGasInvoke"`
		MaxIteratorResultItems int           `yaml:"MaxIteratorResultItems"`
		Port                   uint16        `yaml:"Port"`
		TLSConfig              TLSConfig     `yaml:"TLSConfig"`
	}

	// TLSConfig describes SSL/TLS configuration.
	TLSConfig struct {
		Address  string `yaml:"Address"`
		CertFile string `yaml:"CertFile"`
		Enabled  bool   `yaml:"Enabled"`
		Port     uint16 `yaml:"Port"`
		KeyFile  string `yaml:"KeyFile"`
	}
)
