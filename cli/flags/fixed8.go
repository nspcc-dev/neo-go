package flags

import (
	"flag"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/urfave/cli/v2"
)

// Fixed8 is a wrapper for a Uint160 with flag.Value methods.
type Fixed8 struct {
	Value fixedn.Fixed8
}

// Fixed8Flag is a flag with type string.
type Fixed8Flag struct {
	Name     string
	Usage    string
	Value    Fixed8
	Aliases  []string
	Required bool
	Hidden   bool
	Action   func(*cli.Context, string) error
}

var (
	_ flag.Value = (*Fixed8)(nil)
	_ cli.Flag   = Fixed8Flag{}
)

// String implements the fmt.Stringer interface.
func (a Fixed8) String() string {
	return a.Value.String()
}

// Set implements the flag.Value interface.
func (a *Fixed8) Set(s string) error {
	f, err := fixedn.Fixed8FromString(s)
	if err != nil {
		return cli.Exit(err, 1)
	}
	a.Value = f
	return nil
}

// Fixed8 casts the address to util.Fixed8.
func (a *Fixed8) Fixed8() fixedn.Fixed8 {
	return a.Value
}

// IsSet checks if flag was set to a non-default value.
func (f Fixed8Flag) IsSet() bool {
	return f.Value.Value != 0
}

// String returns a readable representation of this value
// (for usage defaults).
func (f Fixed8Flag) String() string {
	var names []string
	for _, name := range f.Names() {
		names = append(names, getNameHelp(name))
	}
	return strings.Join(names, ", ") + "\t" + f.Usage
}

// Names returns the names of the flag.
func (f Fixed8Flag) Names() []string {
	return cli.FlagNames(f.Name, f.Aliases)
}

// IsRequired returns whether the flag is required.
func (f Fixed8Flag) IsRequired() bool {
	return f.Required
}

// IsVisible returns true if the flag is not hidden, otherwise false.
func (f Fixed8Flag) IsVisible() bool {
	return !f.Hidden
}

// TakesValue returns true if the flag takes a value, otherwise false.
func (f Fixed8Flag) TakesValue() bool {
	return true
}

// GetUsage returns the usage string for the flag.
func (f Fixed8Flag) GetUsage() string {
	return f.Usage
}

// Apply populates the flag given the flag set and environment.
// Ignores errors.
func (f Fixed8Flag) Apply(set *flag.FlagSet) error {
	for _, name := range f.Names() {
		set.Var(&f.Value, name, f.Usage)
	}
	return nil
}

// Fixed8FromContext returns a parsed util.Fixed8 value provided flag name.
func Fixed8FromContext(ctx *cli.Context, name string) fixedn.Fixed8 {
	return ctx.Generic(name).(*Fixed8).Value
}

// RunAction executes flag action if set.
func (f Fixed8Flag) RunAction(c *cli.Context) error {
	if f.Action != nil {
		return f.Action(c, f.Value.Value.String())
	}
	return nil
}

// GetValue returns the flags value as string representation.
func (f Fixed8Flag) GetValue() string {
	return f.Value.Value.String()
}

// Get returns the flag’s value in the given Context.
func (f Fixed8Flag) Get(ctx *cli.Context) Fixed8 {
	adr := ctx.Generic(f.Name).(*Fixed8)
	return *adr
}
