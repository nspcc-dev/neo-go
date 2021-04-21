package cmdargs

import (
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/urfave/cli"
)

// GetSignersFromContext returns signers parsed from context args starting
// from the specified offset.
func GetSignersFromContext(ctx *cli.Context, offset int) ([]transaction.Signer, *cli.ExitError) {
	args := ctx.Args()
	var signers []transaction.Signer
	if args.Present() && len(args) > offset {
		for i, c := range args[offset:] {
			cosigner, err := parseCosigner(c)
			if err != nil {
				return nil, cli.NewExitError(fmt.Errorf("failed to parse signer #%d: %w", i, err), 1)
			}
			signers = append(signers, cosigner)
		}
	}
	return signers, nil
}

func parseCosigner(c string) (transaction.Signer, error) {
	var (
		err error
		res = transaction.Signer{
			Scopes: transaction.CalledByEntry,
		}
	)
	data := strings.SplitN(c, ":", 2)
	s := data[0]
	res.Account, err = flags.ParseAddress(s)
	if err != nil {
		return res, err
	}
	if len(data) > 1 {
		res.Scopes, err = transaction.ScopesFromString(data[1])
		if err != nil {
			return transaction.Signer{}, err
		}
	}
	return res, nil
}
