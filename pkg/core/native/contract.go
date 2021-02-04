package native

import (
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// reservedContractID represents the upper bound of the reserved IDs for native contracts.
const reservedContractID = -100

// Contracts is a set of registered native contracts.
type Contracts struct {
	Management  *Management
	Ledger      *Ledger
	NEO         *NEO
	GAS         *GAS
	Policy      *Policy
	Oracle      *Oracle
	Designate   *Designate
	NameService *NameService
	Notary      *Notary
	Contracts   []interop.Contract
	// persistScript is vm script which executes "onPersist" method of every native contract.
	persistScript []byte
	// postPersistScript is vm script which executes "postPersist" method of every native contract.
	postPersistScript []byte
}

// ByHash returns native contract with the specified hash.
func (cs *Contracts) ByHash(h util.Uint160) interop.Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().Hash.Equals(h) {
			return ctr
		}
	}
	return nil
}

// ByName returns native contract with the specified name.
func (cs *Contracts) ByName(name string) interop.Contract {
	name = strings.ToLower(name)
	for _, ctr := range cs.Contracts {
		if strings.ToLower(ctr.Metadata().Name) == name {
			return ctr
		}
	}
	return nil
}

// NewContracts returns new set of native contracts with new GAS, NEO, Policy, Oracle,
// Designate and (optional) Notary contracts.
func NewContracts(p2pSigExtensionsEnabled bool) *Contracts {
	cs := new(Contracts)

	mgmt := newManagement()
	cs.Management = mgmt
	cs.Contracts = append(cs.Contracts, mgmt)

	ledger := newLedger()
	cs.Ledger = ledger
	cs.Contracts = append(cs.Contracts, ledger)

	gas := newGAS()
	neo := newNEO()
	neo.GAS = gas
	gas.NEO = neo
	mgmt.NEO = neo

	cs.GAS = gas
	cs.NEO = neo
	cs.Contracts = append(cs.Contracts, neo)
	cs.Contracts = append(cs.Contracts, gas)

	policy := newPolicy()
	policy.NEO = neo
	cs.Policy = policy
	cs.Contracts = append(cs.Contracts, policy)

	desig := newDesignate(p2pSigExtensionsEnabled)
	desig.NEO = neo
	cs.Designate = desig
	cs.Contracts = append(cs.Contracts, desig)

	oracle := newOracle()
	oracle.GAS = gas
	oracle.NEO = neo
	oracle.Desig = desig
	cs.Oracle = oracle
	cs.Contracts = append(cs.Contracts, oracle)

	ns := newNameService()
	ns.NEO = neo
	cs.NameService = ns
	cs.Contracts = append(cs.Contracts, ns)

	if p2pSigExtensionsEnabled {
		notary := newNotary()
		notary.GAS = gas
		notary.NEO = neo
		notary.Desig = desig
		cs.Notary = notary
		cs.Contracts = append(cs.Contracts, notary)
	}

	return cs
}

// GetPersistScript returns VM script calling "onPersist" syscall for native contracts.
func (cs *Contracts) GetPersistScript() []byte {
	if cs.persistScript != nil {
		return cs.persistScript
	}
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractNativeOnPersist)
	cs.persistScript = w.Bytes()
	return cs.persistScript
}

// GetPostPersistScript returns VM script calling "postPersist" syscall for native contracts.
func (cs *Contracts) GetPostPersistScript() []byte {
	if cs.postPersistScript != nil {
		return cs.postPersistScript
	}
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractNativePostPersist)
	cs.postPersistScript = w.Bytes()
	return cs.postPersistScript
}
