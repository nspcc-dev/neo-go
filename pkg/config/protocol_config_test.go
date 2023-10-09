package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
		ValidatorsCount: 1,
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		NativeUpdateHistories: map[string][]uint32{
			"someContract": {0, 10},
		},
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
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
			"Basilisk":      2, // Same height is OK.
		},
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
			"Basilisk":      3, // Larger height is OK.
		},
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Aspidochelone": 2,
		},
	}
	require.NoError(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"Basilisk": 2,
		},
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

	p.NativeUpdateHistories = map[string][]uint32{"Contract": {1, 2, 3}}
	o.NativeUpdateHistories = map[string][]uint32{"Contract": {1, 2, 3}}
	require.True(t, p.Equals(o))
	p.NativeUpdateHistories["Contract"] = []uint32{1, 2, 3, 4}
	require.False(t, p.Equals(o))
	p.NativeUpdateHistories["Contract"] = []uint32{1, 2, 4}
	require.False(t, p.Equals(o))

	p.NativeUpdateHistories = nil
	o.NativeUpdateHistories = nil

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
