package native

import (
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// Native contract interfaces sufficient for Blockchain functioning and
// cross-native interaction.
type (
	// IManagement is an interface required from native ContractManagement
	// contract for interaction with Blockchain and other native contracts.
	IManagement interface {
		interop.Contract
		GetNEP11Contracts(d *dao.Simple) []util.Uint160
		GetNEP17Contracts(d *dao.Simple) []util.Uint160
	}

	// INEO is an interface required from native NeoToken contract for
	// interaction with Blockchain and other native contracts.
	INEO interface {
		interop.Contract
		GetCommitteeAddress(d *dao.Simple) util.Uint160
		GetNextBlockValidatorsInternal(d *dao.Simple) keys.PublicKeys
		BalanceOf(d *dao.Simple, acc util.Uint160) (*big.Int, uint32)
		CalculateBonus(ic *interop.Context, acc util.Uint160, endHeight uint32) (*big.Int, error)
		GetCommitteeMembers(d *dao.Simple) keys.PublicKeys
		ComputeNextBlockValidators(d *dao.Simple) keys.PublicKeys
		GetCandidates(d *dao.Simple) ([]state.Validator, error)

		// Methods required for proper cross-native communication.
		CheckCommittee(ic *interop.Context) bool
		CheckAlmostFullCommittee(ic *interop.Context) bool
		RevokeVotes(ic *interop.Context, h util.Uint160) error
	}

	// IGAS is an interface required from native GasToken contract for
	// interaction with Blockchain and other native contracts.
	IGAS interface {
		interop.Contract
		BalanceOf(d *dao.Simple, acc util.Uint160) *big.Int

		// Methods required for proper cross-native communication.
		Burn(ic *interop.Context, h util.Uint160, amount *big.Int)
		Mint(ic *interop.Context, h util.Uint160, amount *big.Int, callOnPayment bool)
	}

	// IPolicy is an interface required from native PolicyContract contract for
	// interaction with Blockchain and other native contracts.
	IPolicy interface {
		interop.Contract
		// GetStoragePriceInternal returns the current storage price in picoGAS units.
		GetStoragePriceInternal(d *dao.Simple) int64
		GetMaxVerificationGas(d *dao.Simple) int64
		// GetExecFeeFactorInternal returns the current execution fee factor in picoGAS units.
		GetExecFeeFactorInternal(d *dao.Simple) int64
		GetMaxTraceableBlocksInternal(d *dao.Simple) uint32
		GetMillisecondsPerBlockInternal(d *dao.Simple) uint32
		GetMaxValidUntilBlockIncrementFromCache(d *dao.Simple) uint32
		GetAttributeFeeInternal(d *dao.Simple, attrType transaction.AttrType) int64
		CheckPolicy(d *dao.Simple, tx *transaction.Transaction) error
		GetFeePerByteInternal(d *dao.Simple) int64

		// Methods required for proper cross-native communication.
		BlockAccountInternal(ic *interop.Context, hash util.Uint160) bool
		GetMaxValidUntilBlockIncrementInternal(ic *interop.Context) uint32
		CleanWhitelist(ic *interop.Context, hash util.Uint160) error
		interop.PolicyChecker
	}

	// IOracle is an interface required from native OracleContract contract for
	// interaction with Blockchain and other native contracts.
	IOracle interface {
		interop.Contract
		GetOracleResponseScript() []byte
		GetRequests(d *dao.Simple) (map[uint64]*state.OracleRequest, error)
		GetScriptHash(d *dao.Simple) (util.Uint160, error)
		GetRequestInternal(d *dao.Simple, id uint64) (*state.OracleRequest, error)
		SetService(o OracleService)
	}

	// IDesignate is an interface required from native RoleManagement contract
	// for interaction with Blockchain and other native contracts.
	IDesignate interface {
		interop.Contract
		GetDesignatedByRole(d *dao.Simple, r noderoles.Role, index uint32) (keys.PublicKeys, uint32, error)
		GetLastDesignatedHash(d *dao.Simple, r noderoles.Role) (util.Uint160, error)
		SetOracleService(o OracleService)
		SetNotaryService(n NotaryService)
		SetStateRootService(s StateRootService)
		NotifyServices(dao *dao.Simple)
	}

	// INotary is an interface required from native Notary contract for
	// interaction with Blockchain and other native contracts.
	INotary interface {
		interop.Contract
		BalanceOf(dao *dao.Simple, acc util.Uint160) *big.Int
		ExpirationOf(dao *dao.Simple, acc util.Uint160) uint32
		GetMaxNotValidBeforeDelta(dao *dao.Simple) uint32
	}
)

// Contracts is a convenient wrapper around an arbitrary set of native contracts
// providing common helper contract accessors.
type Contracts struct {
	List []interop.Contract
	// persistScript is a vm script which executes "onPersist" method of every native contract.
	persistScript []byte
	// postPersistScript is a vm script which executes "postPersist" method of every native contract.
	postPersistScript []byte
}

// NewContracts initializes a wrapper around the provided set of native
// contracts.
func NewContracts(natives []interop.Contract) *Contracts {
	return &Contracts{
		List: natives,
	}
}

// ByHash returns a native contract with the specified hash.
func (cs *Contracts) ByHash(h util.Uint160) interop.Contract {
	for _, ctr := range cs.List {
		if ctr.Metadata().Hash.Equals(h) {
			return ctr
		}
	}
	return nil
}

// ByName returns a native contract with the specified name.
func (cs *Contracts) ByName(name string) interop.Contract {
	name = strings.ToLower(name)
	for _, ctr := range cs.List {
		if strings.ToLower(ctr.Metadata().Name) == name {
			return ctr
		}
	}
	return nil
}

// GetPersistScript returns a VM script calling "onPersist" syscall for native contracts.
func (cs *Contracts) GetPersistScript() []byte {
	if cs.persistScript != nil {
		return cs.persistScript
	}
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractNativeOnPersist)
	cs.persistScript = w.Bytes()
	return cs.persistScript
}

// GetPostPersistScript returns a VM script calling "postPersist" syscall for native contracts.
func (cs *Contracts) GetPostPersistScript() []byte {
	if cs.postPersistScript != nil {
		return cs.postPersistScript
	}
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractNativePostPersist)
	cs.postPersistScript = w.Bytes()
	return cs.postPersistScript
}

// Management returns native IManagement implementation. It panics if there's no
// contract with proper name in cs.
func (cs *Contracts) Management() IManagement {
	return cs.ByName(nativenames.Management).(IManagement)
}

// NEO returns native INEO contract implementation. It panics if there's no
// contract with proper name in cs.
func (cs *Contracts) NEO() INEO {
	return cs.ByName(nativenames.Neo).(INEO)
}

// GAS returns native IGAS contract implementation. It panics if there's no
// contract with proper name in cs.
func (cs *Contracts) GAS() IGAS {
	return cs.ByName(nativenames.Gas).(IGAS)
}

// Designate returns native IDesignate contract implementation. It panics if
// there's no contract with proper name in cs.
func (cs *Contracts) Designate() IDesignate {
	return cs.ByName(nativenames.Designation).(IDesignate)
}

// Policy returns native IPolicy contract implementation. It panics if there's
// no contract with proper name in cs.
func (cs *Contracts) Policy() IPolicy {
	return cs.ByName(nativenames.Policy).(IPolicy)
}

// Oracle returns native IOracle contract implementation. It returns nil if
// there's no contract with proper name in cs.
func (cs *Contracts) Oracle() IOracle {
	res := cs.ByName(nativenames.Oracle)
	// Oracle contract is optional.
	if res != nil {
		return res.(IOracle)
	}
	return nil
}

// Notary returns native INotary contract implementation. It returns nil if
// there's no contract with proper name in cs.
func (cs *Contracts) Notary() INotary {
	res := cs.ByName(nativenames.Notary)
	// Notary contract is optional.
	if res != nil {
		return res.(INotary)
	}
	return nil
}

// NewDefaultContracts returns a new set of default native contracts.
func NewDefaultContracts(cfg config.ProtocolConfiguration) []interop.Contract {
	mgmt := NewManagement()
	s := newStd()
	c := newCrypto()
	ledger := NewLedger()

	gas := newGAS(int64(cfg.InitialGASSupply))
	neo := newNEO(cfg)
	policy := newPolicy()
	neo.GAS = gas
	neo.Policy = policy
	gas.NEO = neo
	gas.Policy = policy
	mgmt.NEO = neo
	mgmt.Policy = policy
	policy.NEO = neo
	ledger.Policy = policy

	desig := NewDesignate(cfg.Genesis.Roles)
	desig.NEO = neo

	oracle := newOracle()
	oracle.GAS = gas
	oracle.NEO = neo
	oracle.Desig = desig

	notary := newNotary()
	notary.GAS = gas
	notary.NEO = neo
	notary.Desig = desig
	notary.Policy = policy

	treasury := newTreasury()
	treasury.NEO = neo

	return []interop.Contract{
		mgmt,
		s,
		c,
		ledger,
		neo,
		gas,
		policy,
		desig,
		oracle,
		notary,
		treasury,
	}
}
