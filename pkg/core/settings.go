package core

type (
	// Settings top level struct representing the settings
	// for the node.
	Settings struct {
		ProtocolConfiguration ProtocolConfiguration
	}

	// ProtocolConfiguration represents the protolcol config.
	ProtocolConfiguration struct {
		Magic                   int64
		AddressVersion          int64
		MaxTransactionsPerBlock int64
		StandbyValidators       []string
		SeedList                []string
		SystemFee               SystemFee
	}

	// SystemFee fees related to system.
	SystemFee struct {
		EnrollmentTransaction int64
		IssueTransaction      int64
		PublishTransaction    int64
		RegisterTransaction   int64
	}
)
