package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/metrics"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
)

const (
	userAgentFormat = "/NEO-GO:%s/"

	// ModeMainNet contains magic code used in the NEO main official network.
	ModeMainNet NetMode = 0x00746e41 // 7630401
	// ModeTestNet contains magic code used in the NEO testing network.
	ModeTestNet NetMode = 0x74746e41 // 1953787457
	// ModePrivNet contains magic code usually used for NEO private networks.
	ModePrivNet NetMode = 56753 // docker privnet
	// ModeUnitTestNet is a stub magic code used for testing purposes.
	ModeUnitTestNet NetMode = 0
)

var (
	// Version the version of the node, set at build time.
	Version string
)

type (
	// Config top level struct representing the config
	// for the node.
	Config struct {
		ProtocolConfiguration    ProtocolConfiguration    `yaml:"ProtocolConfiguration"`
		ApplicationConfiguration ApplicationConfiguration `yaml:"ApplicationConfiguration"`
	}

	// ProtocolConfiguration represents the protocol config.
	ProtocolConfiguration struct {
		Magic                   NetMode   `yaml:"Magic"`
		AddressVersion          byte      `yaml:"AddressVersion"`
		SecondsPerBlock         int       `yaml:"SecondsPerBlock"`
		LowPriorityThreshold    float64   `yaml:"LowPriorityThreshold"`
		MaxTransactionsPerBlock int64     `yaml:"MaxTransactionsPerBlock"`
		StandbyValidators       []string  `yaml:"StandbyValidators"`
		SeedList                []string  `yaml:"SeedList"`
		SystemFee               SystemFee `yaml:"SystemFee"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}

	// SystemFee fees related to system.
	SystemFee struct {
		EnrollmentTransaction int64 `yaml:"EnrollmentTransaction"`
		IssueTransaction      int64 `yaml:"IssueTransaction"`
		PublishTransaction    int64 `yaml:"PublishTransaction"`
		RegisterTransaction   int64 `yaml:"RegisterTransaction"`
	}

	// ApplicationConfiguration config specific to the node.
	ApplicationConfiguration struct {
		LogPath           string                  `yaml:"LogPath"`
		DBConfiguration   storage.DBConfiguration `yaml:"DBConfiguration"`
		Address           string                  `yaml:"Address"`
		NodePort          uint16                  `yaml:"NodePort"`
		Relay             bool                    `yaml:"Relay"`
		DialTimeout       time.Duration           `yaml:"DialTimeout"`
		ProtoTickInterval time.Duration           `yaml:"ProtoTickInterval"`
		MaxPeers          int                     `yaml:"MaxPeers"`
		AttemptConnPeers  int                     `yaml:"AttemptConnPeers"`
		MinPeers          int                     `yaml:"MinPeers"`
		Prometheus        metrics.Config          `yaml:"Prometheus"`
		Pprof             metrics.Config          `yaml:"Pprof"`
		RPC               RPCConfig               `yaml:"RPC"`
		UnlockWallet      WalletConfig            `yaml:"UnlockWallet"`
	}

	// WalletConfig is a wallet info.
	WalletConfig struct {
		Path     string `yaml:"Path"`
		Password string `yaml:"Password"`
	}

	// RPCConfig is an RPC service configuration information (to be moved to the rpc package, see #423).
	RPCConfig struct {
		Enabled              bool   `yaml:"Enabled"`
		EnableCORSWorkaround bool   `yaml:"EnableCORSWorkaround"`
		Address              string `yaml:"Address"`
		Port                 uint16 `yaml:"Port"`
	}

	// NetMode describes the mode the blockchain will operate on.
	NetMode uint32
)

// String implements the stringer interface.
func (n NetMode) String() string {
	switch n {
	case ModePrivNet:
		return "privnet"
	case ModeTestNet:
		return "testnet"
	case ModeMainNet:
		return "mainnet"
	case ModeUnitTestNet:
		return "unit_testnet"
	default:
		return "net unknown"
	}
}

// GenerateUserAgent creates user agent string based on build time environment.
func (c Config) GenerateUserAgent() string {
	return fmt.Sprintf(userAgentFormat, Version)
}

// Load attempts to load the config from the given
// path for the given netMode.
func Load(path string, netMode NetMode) (Config, error) {
	configPath := fmt.Sprintf("%s/protocol.%s.yml", path, netMode)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Config{}, errors.Wrap(err, "Unable to load config")
	}

	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Config{}, errors.Wrap(err, "Unable to read config")
	}

	config := Config{
		ProtocolConfiguration: ProtocolConfiguration{
			SystemFee: SystemFee{},
		},
		ApplicationConfiguration: ApplicationConfiguration{},
	}

	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return Config{}, errors.Wrap(err, "Problem unmarshaling config json data")
	}

	return config, nil
}

// TryGetValue returns the system fee base on transaction type.
func (s SystemFee) TryGetValue(txType transaction.TXType) util.Fixed8 {
	switch txType {
	case transaction.EnrollmentType:
		return util.Fixed8FromInt64(s.EnrollmentTransaction)
	case transaction.IssueType:
		return util.Fixed8FromInt64(s.IssueTransaction)
	case transaction.PublishType:
		return util.Fixed8FromInt64(s.PublishTransaction)
	case transaction.RegisterType:
		return util.Fixed8FromInt64(s.RegisterTransaction)
	default:
		return util.Fixed8FromInt64(0)
	}
}
