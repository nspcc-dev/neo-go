package flags

import (
	"flag"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

// Address is a wrapper for a Uint160 with flag.Value methods.
type Address struct {
	IsSet bool
	Value util.Uint160
}

// AddressFlag is a flag with type Uint160.
type AddressFlag struct {
	Name     string
	Usage    string
	Value    Address
	Aliases  []string
	Required bool
	Hidden   bool
	Action   func(*cli.Context, string) error
}

var (
	_ flag.Value = (*Address)(nil)
	_ cli.Flag   = AddressFlag{}
)

// String implements the fmt.Stringer interface.
func (a Address) String() string {
	return address.Uint160ToString(a.Value)
}

// Set implements the flag.Value interface.
func (a *Address) Set(s string) error {
	addr, err := ParseAddress(s)
	if err != nil {
		return cli.Exit(err, 1)
	}
	a.IsSet = true
	a.Value = addr
	return nil
}

// Uint160 casts an address to Uint160.
func (a *Address) Uint160() (u util.Uint160) {
	if !a.IsSet {
		// It is a programmer error to call this method without
		// checking if the value was provided.
		panic("address was not set")
	}
	return a.Value
}

// IsSet checks if flag was set to a non-default value.
func (f AddressFlag) IsSet() bool {
	return f.Value.IsSet
}

// String returns a readable representation of this value
// (for usage defaults).
func (f AddressFlag) String() string {
	var names []string
	for _, name := range f.Names() {
		names = append(names, getNameHelp(name))
	}

	return strings.Join(names, ", ") + "\t" + f.Usage
}

func getNameHelp(name string) string {
	if len(name) == 1 {
		return fmt.Sprintf("-%s value", name)
	}
	return fmt.Sprintf("--%s value", name)
}

// Names returns the names of the flag.
func (f AddressFlag) Names() []string {
	return cli.FlagNames(f.Name, f.Aliases)
}

// IsRequired returns whether the flag is required.
func (f AddressFlag) IsRequired() bool {
	return f.Required
}

// IsVisible returns true if the flag is not hidden, otherwise false.
func (f AddressFlag) IsVisible() bool {
	return !f.Hidden
}

// TakesValue returns true of the flag takes a value, otherwise false.
func (f AddressFlag) TakesValue() bool {
	return true
}

// GetUsage returns the usage string for the flag.
func (f AddressFlag) GetUsage() string {
	return f.Usage
}

// Apply populates the flag given the flag set and environment.
// Ignores errors.
func (f AddressFlag) Apply(set *flag.FlagSet) error {
	for _, name := range f.Names() {
		set.Var(&f.Value, name, f.Usage)
	}
	return nil
}

// RunAction executes flag action if set.
func (f AddressFlag) RunAction(c *cli.Context) error {
	if f.Action != nil {
		return f.Action(c, address.Uint160ToString(f.Value.Value))
	}
	return nil
}

// GetValue returns the flags value as string representation.
func (f AddressFlag) GetValue() string {
	return address.Uint160ToString(f.Value.Value)
}

// Get returns the flag’s value in the given Context.
func (f AddressFlag) Get(ctx *cli.Context) Address {
	adr := ctx.Generic(f.Name).(*Address)
	return *adr
}

// ParseAddress parses a Uint160 from either an LE string or an address.
func ParseAddress(s string) (util.Uint160, error) {
	const uint160size = 2 * util.Uint160Size
	switch len(s) {
	case uint160size, uint160size + 2:
		return util.Uint160DecodeStringLE(strings.TrimPrefix(s, "0x"))
	default:
		return address.StringToUint160(s)
	}
}
