package config

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nspcc-dev/neo-go/config"
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
	// DefaultMaxRequestHeaderBytes is the maximum permitted size of the headers
	// in an HTTP request.
	DefaultMaxRequestHeaderBytes = http.DefaultMaxHeaderBytes
	// DefaultConfigPath is the default path to the config directory.
	DefaultConfigPath = "./config"
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
// path for the given netMode. If relativePath is not empty, relative paths in the
// config will be updated based on the provided relative path.
func Load(path string, netMode netmode.Magic, relativePath ...string) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	return LoadFile(configPath, relativePath...)
}

// LoadFile loads config from the provided path. It also applies backwards compatibility
// fixups if necessary. If relativePath is not empty, relative paths in the config will
// be updated based on the provided relative path.
func LoadFile(configPath string, relativePath ...string) (Config, error) {
	var (
		configData []byte
		err        error
	)
	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		configData, err = getEmbeddedConfig(configPath)
		if err != nil {
			return Config{}, err
		}
	} else {
		configData, err = os.ReadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("unable to read config: %w", err)
		}
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
	if len(relativePath) == 1 && relativePath[0] != "" {
		updateRelativePaths(relativePath[0], &config)
	}

	err = config.ProtocolConfiguration.Validate()
	if err != nil {
		return Config{}, err
	}
	err = config.ApplicationConfiguration.Validate()
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

// getEmbeddedConfig returns the embedded config based on the provided config path.
func getEmbeddedConfig(configPath string) ([]byte, error) {
	switch configPath {
	case fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.MainNet):
		return config.MainNet, nil
	case fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.TestNet):
		return config.TestNet, nil
	case fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.PrivNet):
		return config.PrivNet, nil
	case fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.MainNetNeoFS):
		return config.MainNetNeoFS, nil
	case fmt.Sprintf("%s/protocol.%s.yml", DefaultConfigPath, netmode.TestNetNeoFS):
		return config.TestNetNeoFS, nil
	default:
		return nil, fmt.Errorf("config '%s' doesn't exist and no matching embedded config was found", configPath)
	}
}

// updateRelativePaths updates relative paths in the config structure based on the provided relative path.
func updateRelativePaths(relativePath string, config *Config) {
	updatePath := func(path *string) {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(relativePath, *path)
		}
	}

	updatePath(&config.ApplicationConfiguration.LogPath)
	updatePath(&config.ApplicationConfiguration.DBConfiguration.BoltDBOptions.FilePath)
	updatePath(&config.ApplicationConfiguration.DBConfiguration.LevelDBOptions.DataDirectoryPath)
	updatePath(&config.ApplicationConfiguration.Consensus.UnlockWallet.Path)
	updatePath(&config.ApplicationConfiguration.P2PNotary.UnlockWallet.Path)
	updatePath(&config.ApplicationConfiguration.Oracle.UnlockWallet.Path)
	updatePath(&config.ApplicationConfiguration.StateRoot.UnlockWallet.Path)
}
