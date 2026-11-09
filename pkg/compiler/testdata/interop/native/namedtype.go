package policy

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

type (
	Hardfork    string
	SpecialByte byte
)

const HFIara Hardfork = "Iara"

const Hash = ""

func ActivateHardfork(hf Hardfork) {
	neogointernal.CallWithTokenNoRet(Hash, "activateHardfork", int(contract.States|contract.AllowNotify), string(hf))
}

func SomeMethod() SpecialByte {
	return neogointernal.CallWithToken(Hash, "someMethod", int(contract.NoneFlag)).(SpecialByte)
}
