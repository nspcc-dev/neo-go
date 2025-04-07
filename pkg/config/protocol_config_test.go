package config

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProtocolConfigurationValidation(t *testing.T) {
	p := &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
		TimePerBlock:    time.Microsecond,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
		Genesis: Genesis{
			TimePerBlock: time.Microsecond,
		},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		ValidatorsCount: 1,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
		},
		ValidatorsCount: 3,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		ValidatorsCount:   4,
		ValidatorsHistory: map[uint32]uint32{0: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 4},
		ValidatorsHistory: map[uint32]uint32{0: 4, 1000: 5},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 4, 1000: 5},
		ValidatorsHistory: map[uint32]uint32{0: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 999: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 1000: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1, 999: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 4, 100: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 0, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1, 100: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 0, 100: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1, 100: 4},
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee:  []string{},
		CommitteeHistory:  map[uint32]uint32{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1, 100: 4},
	}
	err := p.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration should include StandbyCommittee")
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2"},
	}
	err = p.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "configuration should either have one of ValidatorsCount or ValidatorsHistory, not both")
}

func TestProtocolConfigurationValidation_Hardforks(t *testing.T) {
	p := &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Unknown": 123, // Unknown hard-fork.
		},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
			"Basilisk":      1, // Lower height in higher hard-fork.
		},
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
			"Basilisk":      2, // Same height is OK.
		}, StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
			"Basilisk":      3, // Larger height is OK.
		},
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
		},
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Basilisk": 2,
		},
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount: 1,
	}
	require.NoError(t, p.Validate())
}

func TestGetCommitteeAndCNs(t *testing.T) {
	p := &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]uint32{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]uint32{0: 1, 200: 4},
	}
	require.Equal(t, 1, p.GetCommitteeSize(0))
	require.Equal(t, 1, p.GetCommitteeSize(99))
	require.Equal(t, 4, p.GetCommitteeSize(100))
	require.Equal(t, 4, p.GetCommitteeSize(101))
	require.Equal(t, 4, p.GetCommitteeSize(200))
	require.Equal(t, 4, p.GetCommitteeSize(201))
	require.Equal(t, 1, p.GetNumOfCNs(0))
	require.Equal(t, 1, p.GetNumOfCNs(100))
	require.Equal(t, 1, p.GetNumOfCNs(101))
	require.Equal(t, 1, p.GetNumOfCNs(199))
	require.Equal(t, 4, p.GetNumOfCNs(200))
	require.Equal(t, 4, p.GetNumOfCNs(201))
}

func TestProtocolConfigurationEquals(t *testing.T) {
	p := &ProtocolConfiguration{}
	o := &ProtocolConfiguration{}
	require.True(t, p.Equals(o))
	require.True(t, o.Equals(p))
	require.True(t, p.Equals(p))

	cfg1, err := LoadFile(filepath.Join("..", "..", "config", "protocol.mainnet.yml"))
	require.NoError(t, err)
	cfg2, err := LoadFile(filepath.Join("..", "..", "config", "protocol.testnet.yml"))
	require.NoError(t, err)
	require.False(t, cfg1.ProtocolConfiguration.Equals(&cfg2.ProtocolConfiguration))

	cfg2, err = LoadFile(filepath.Join("..", "..", "config", "protocol.mainnet.yml"))
	require.NoError(t, err)
	p = &cfg1.ProtocolConfiguration
	o = &cfg2.ProtocolConfiguration
	require.True(t, p.Equals(o))

	o.CommitteeHistory = map[uint32]uint32{111: 7}
	p.CommitteeHistory = map[uint32]uint32{111: 7}
	require.True(t, p.Equals(o))
	p.CommitteeHistory[111] = 8
	require.False(t, p.Equals(o))

	o.CommitteeHistory = nil
	p.CommitteeHistory = nil

	p.Hardforks = map[string]uint32{"Fork": 42}
	o.Hardforks = map[string]uint32{"Fork": 42}
	require.True(t, p.Equals(o))
	p.Hardforks = map[string]uint32{"Fork2": 42}
	require.False(t, p.Equals(o))

	p.Hardforks = nil
	o.Hardforks = nil

	p.SeedList = []string{"url1", "url2"}
	o.SeedList = []string{"url1", "url2"}
	require.True(t, p.Equals(o))
	p.SeedList = []string{"url11", "url22"}
	require.False(t, p.Equals(o))

	p.SeedList = nil
	o.SeedList = nil

	p.StandbyCommittee = []string{"key1", "key2"}
	o.StandbyCommittee = []string{"key1", "key2"}
	require.True(t, p.Equals(o))
	p.StandbyCommittee = []string{"key2", "key1"}
	require.False(t, p.Equals(o))

	p.StandbyCommittee = nil
	o.StandbyCommittee = nil

	o.ValidatorsHistory = map[uint32]uint32{111: 0}
	p.ValidatorsHistory = map[uint32]uint32{111: 0}
	require.True(t, p.Equals(o))
	p.ValidatorsHistory = map[uint32]uint32{112: 0}
	require.False(t, p.Equals(o))
}

func TestGenesisExtensionsMarshalYAML(t *testing.T) {
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := pk.PublicKey()

	t.Run("MarshalUnmarshalYAML", func(t *testing.T) {
		g := &Genesis{
			Roles: map[noderoles.Role]keys.PublicKeys{
				noderoles.NeoFSAlphabet: {pub},
				noderoles.P2PNotary:     {pub},
			},
			Transaction: &GenesisTransaction{
				Script:    []byte{1, 2, 3, 4},
				SystemFee: 123,
			},
		}
		testserdes.MarshalUnmarshalYAML(t, g, new(Genesis))
	})

	t.Run("unmarshal config", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			pubStr := pub.StringCompressed()
			script := []byte{1, 2, 3, 4}
			cfgYml := fmt.Sprintf(`ProtocolConfiguration:
  Genesis:
    Transaction:
      Script: "%s"
      SystemFee: 123
    Roles:
      NeoFSAlphabet:
        - %s
        - %s
      Oracle:
        - %s
        - %s`, base64.StdEncoding.EncodeToString(script), pubStr, pubStr, pubStr, pubStr)
			cfg := new(Config)
			require.NoError(t, yaml.Unmarshal([]byte(cfgYml), cfg))
			require.Equal(t, 2, len(cfg.ProtocolConfiguration.Genesis.Roles))
			require.Equal(t, keys.PublicKeys{pub, pub}, cfg.ProtocolConfiguration.Genesis.Roles[noderoles.NeoFSAlphabet])
			require.Equal(t, keys.PublicKeys{pub, pub}, cfg.ProtocolConfiguration.Genesis.Roles[noderoles.Oracle])
			require.Equal(t, &GenesisTransaction{
				Script:    script,
				SystemFee: 123,
			}, cfg.ProtocolConfiguration.Genesis.Transaction)
		})

		t.Run("empty", func(t *testing.T) {
			cfgYml := `ProtocolConfiguration:`
			cfg := new(Config)
			require.NoError(t, yaml.Unmarshal([]byte(cfgYml), cfg))
			require.Nil(t, cfg.ProtocolConfiguration.Genesis.Transaction)
			require.Empty(t, cfg.ProtocolConfiguration.Genesis.Roles)
		})

		t.Run("unknown role", func(t *testing.T) {
			pubStr := pub.StringCompressed()
			cfgYml := fmt.Sprintf(`ProtocolConfiguration:
  Genesis:
    Roles:
      BadRole:
        - %s`, pubStr)
			cfg := new(Config)
			err := yaml.Unmarshal([]byte(cfgYml), cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown node role: BadRole")
		})

		t.Run("last role", func(t *testing.T) {
			pubStr := pub.StringCompressed()
			cfgYml := fmt.Sprintf(`ProtocolConfiguration:
  Genesis:
    Roles:
      last:
        - %s`, pubStr)
			cfg := new(Config)
			err := yaml.Unmarshal([]byte(cfgYml), cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown node role: last")
		})
	})
}
