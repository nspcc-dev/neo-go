package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolConfigurationValidation(t *testing.T) {
	p := &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
		},
		ValidatorsCount:            1,
		KeepOnlyLatestState:        true,
		P2PStateExchangeExtensions: true,
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
		ValidatorsHistory: map[uint32]int{0: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 4},
		ValidatorsHistory: map[uint32]int{0: 4, 1000: 5},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 4, 1000: 5},
		ValidatorsHistory: map[uint32]int{0: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 1, 999: 4},
		ValidatorsHistory: map[uint32]int{0: 1},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 1, 1000: 4},
		ValidatorsHistory: map[uint32]int{0: 1, 999: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]int{0: 4, 100: 4},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		Hardforks: map[string]uint32{
			"HF_Unknown": 123, // Unknown hard-fork.
		},
	}
	require.Error(t, p.Validate())
	p = &ProtocolConfiguration{
		StandbyCommittee: []string{
			"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2",
			"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e",
			"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699",
			"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62",
		},
		CommitteeHistory:  map[uint32]int{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]int{0: 1, 100: 4},
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
		CommitteeHistory:  map[uint32]int{0: 1, 100: 4},
		ValidatorsHistory: map[uint32]int{0: 1, 200: 4},
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
