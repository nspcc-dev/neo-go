package cmdargs

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

const (
	// CosignersSeparator marks the start of cosigners cli args.
	CosignersSeparator = "--"
	// ArrayStartSeparator marks the start of array cli arg.
	ArrayStartSeparator = "["
	// ArrayEndSeparator marks the end of array cli arg.
	ArrayEndSeparator = "]"
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

	if len(data) == 1 {
		return res, nil
	}

	res.Scopes = 0
	scopes := strings.Split(data[1], ",")
	for _, s := range scopes {
		sub := strings.Split(s, ":")
		scope, err := transaction.ScopesFromString(sub[0])
		if err != nil {
			return transaction.Signer{}, err
		}
		if scope == transaction.Global && res.Scopes&^transaction.Global != 0 ||
			scope != transaction.Global && res.Scopes&transaction.Global != 0 {
			return transaction.Signer{}, errors.New("Global scope can not be combined with other scopes")
		}

		res.Scopes |= scope

		switch scope {
		case transaction.CustomContracts:
			if len(sub) == 1 {
				return transaction.Signer{}, errors.New("CustomContracts scope must refer to at least one contract")
			}
			for _, s := range sub[1:] {
				addr, err := flags.ParseAddress(s)
				if err != nil {
					return transaction.Signer{}, err
				}

				res.AllowedContracts = append(res.AllowedContracts, addr)
			}
		case transaction.CustomGroups:
			if len(sub) == 1 {
				return transaction.Signer{}, errors.New("CustomGroups scope must refer to at least one group")
			}
			for _, s := range sub[1:] {
				pub, err := keys.NewPublicKeyFromString(s)
				if err != nil {
					return transaction.Signer{}, err
				}

				res.AllowedGroups = append(res.AllowedGroups, pub)
			}
		}
	}
	return res, nil
}

// GetDataFromContext returns data parameter from context args.
func GetDataFromContext(ctx *cli.Context) (int, interface{}, *cli.ExitError) {
	var (
		data   interface{}
		offset int
		params []smartcontract.Parameter
		err    error
	)
	args := ctx.Args()
	if args.Present() {
		offset, params, err = ParseParams(args, true)
		if err != nil {
			return offset, nil, cli.NewExitError(fmt.Errorf("unable to parse 'data' parameter: %w", err), 1)
		}
		if len(params) > 1 {
			return offset, nil, cli.NewExitError("'data' should be represented as a single parameter", 1)
		}
		if len(params) != 0 {
			data, err = smartcontract.ExpandParameterToEmitable(params[0])
			if err != nil {
				return offset, nil, cli.NewExitError(fmt.Sprintf("failed to convert 'data' to emitable type: %s", err.Error()), 1)
			}
		}
	}
	return offset, data, nil
}

// EnsureNone returns an error if there are any positional arguments present.
// It can be used to check for them in commands that don't accept arguments.
func EnsureNone(ctx *cli.Context) *cli.ExitError {
	if ctx.Args().Present() {
		return cli.NewExitError("additional arguments given while this command expects none", 1)
	}
	return nil
}

// ParseParams extracts array of smartcontract.Parameter from the given args and
// returns the number of handled words, the array itself and an error.
// `calledFromMain` denotes whether the method was called from the outside or
// recursively and used to check if CosignersSeparator and ArrayEndSeparator are
// allowed to be in `args` sequence.
func ParseParams(args []string, calledFromMain bool) (int, []smartcontract.Parameter, error) {
	res := []smartcontract.Parameter{}
	for k := 0; k < len(args); {
		s := args[k]
		switch s {
		case CosignersSeparator:
			if calledFromMain {
				return k + 1, res, nil // `1` to convert index to numWordsRead
			}
			return 0, []smartcontract.Parameter{}, errors.New("invalid array syntax: missing closing bracket")
		case ArrayStartSeparator:
			numWordsRead, array, err := ParseParams(args[k+1:], false)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to parse array: %w", err)
			}
			res = append(res, smartcontract.Parameter{
				Type:  smartcontract.ArrayType,
				Value: array,
			})
			k += 1 + numWordsRead // `1` for opening bracket
		case ArrayEndSeparator:
			if calledFromMain {
				return 0, nil, errors.New("invalid array syntax: missing opening bracket")
			}
			return k + 1, res, nil // `1`to convert index to numWordsRead
		default:
			param, err := smartcontract.NewParameterFromString(s)
			if err != nil {
				// '--' argument is skipped by urfave/cli library, which leads
				// to [--, addr:scope] being transformed to [addr:scope] and
				// interpreted as a parameter if other positional arguments are not present.
				// Here we fallback to parsing cosigners in this specific case to
				// create a better user experience ('-- addr:scope' vs '-- -- addr:scope').
				if k == 0 {
					if _, err := parseCosigner(s); err == nil {
						return 0, nil, nil
					}
				}
				return 0, nil, fmt.Errorf("failed to parse argument #%d: %w", k+1, err)
			}
			res = append(res, *param)
			k++
		}
	}
	if calledFromMain {
		return len(args), res, nil
	}
	return 0, []smartcontract.Parameter{}, errors.New("invalid array syntax: missing closing bracket")
}

// GetSignersAccounts returns the list of signers combined with the corresponding
// accounts from the provided wallet.
func GetSignersAccounts(senderAcc *wallet.Account, wall *wallet.Wallet, signers []transaction.Signer, accScope transaction.WitnessScope) ([]actor.SignerAccount, error) {
	signersAccounts := make([]actor.SignerAccount, 0, len(signers)+1)
	sender, err := address.StringToUint160(senderAcc.Address)
	if err != nil {
		return nil, err
	}
	signersAccounts = append(signersAccounts, actor.SignerAccount{
		Signer: transaction.Signer{
			Account: sender,
			Scopes:  accScope,
		},
		Account: senderAcc,
	})
	for i, s := range signers {
		if s.Account == sender {
			signersAccounts[0].Signer = s
			continue
		}
		signerAcc := wall.GetAccount(s.Account)
		if signerAcc == nil {
			return nil, fmt.Errorf("no account was found in the wallet for signer #%d (%s)", i, address.Uint160ToString(s.Account))
		}
		signersAccounts = append(signersAccounts, actor.SignerAccount{
			Signer:  s,
			Account: signerAcc,
		})
	}
	return signersAccounts, nil
}
