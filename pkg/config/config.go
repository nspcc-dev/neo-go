package config

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"gopkg.in/yaml.v3"
)

const (
	// UserAgentWrapper is a string that user agent string should be wrapped into.
	UserAgentWrapper = "/"
	// UserAgentPrefix is a prefix used to generate user agent string.
	UserAgentPrefix = "NEO-GO:"
	// UserAgentFormat is a formatted string used to generate user agent string.
	UserAgentFormat = UserAgentWrapper + UserAgentPrefix + "%s" + UserAgentWrapper
	// DefaultMaxIteratorResultItems is the default upper bound of traversed
	// iterator items per JSON-RPC response. It covers both session-based and
	// naive iterators.
	DefaultMaxIteratorResultItems = 100
	// DefaultMaxFindResultItems is the default maximum number of resulting
	// contract states items that can be retrieved by `findstates` JSON-RPC handler.
	DefaultMaxFindResultItems = 100
	// DefaultMaxFindStorageResultItems is the default maximum number of resulting
	// contract storage items that can be retrieved by `findstorge` JSON-RPC handler.
	DefaultMaxFindStorageResultItems = 50
	// DefaultMaxNEP11Tokens is the default maximum number of resulting NEP11 tokens
	// that can be traversed by `getnep11balances` JSON-RPC handler.
	DefaultMaxNEP11Tokens = 100
	// DefaultMaxRequestBodyBytes is the default maximum allowed size of HTTP
	// request body in bytes.
	DefaultMaxRequestBodyBytes = 5 * 1024 * 1024
)

// Version is the version of the node, set at the build time.
var Version string

// Config top level struct representing the config
// for the node.
type Config struct {
	ProtocolConfiguration    ProtocolConfiguration    `yaml:"ProtocolConfiguration"`
	ApplicationConfiguration ApplicationConfiguration `yaml:"ApplicationConfiguration"`
}

// GenerateUserAgent creates a user agent string based on the build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(UserAgentFormat, Version)
}

// Blockchain generates a Blockchain configuration based on Protocol and
// Application settings.
func (c Config) Blockchain() Blockchain {
	return Blockchain{
		ProtocolConfiguration: c.ProtocolConfiguration,
		Ledger:                c.ApplicationConfiguration.Ledger,
	}
}

// Load attempts to load the config from the given
// path for the given netMode.
func Load(path string, netMode netmode.Magic) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	return LoadFile(configPath)
}

// LoadFile loads config from the provided path. It also applies backwards compatibility
// fixups if necessary.
func LoadFile(configPath string) (Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, fmt.Errorf("config '%s' doesn't exist", configPath)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read config: %w", err)
	}

	config := Config{
		ApplicationConfiguration: ApplicationConfiguration{
			P2P: P2P{
				PingInterval: 30 * time.Second,
				PingTimeout:  90 * time.Second,
			},
		},
	}
	decoder := yaml.NewDecoder(bytes.NewReader(configData))
	decoder.KnownFields(true)
	err = decoder.Decode(&config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config YAML: %w", err)
	}

	err = config.ProtocolConfiguration.Validate()
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
