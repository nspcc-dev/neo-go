package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

func newValidatorCommands() []cli.Command {
	return []cli.Command{
		{
			Name:      "register",
			Usage:     "register as a new candidate",
			UsageText: "register -w <path> -r <rpc> -a <addr>",
			Action:    handleRegister,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				gasFlag,
				flags.AddressFlag{
					Name:  "address, a",
					Usage: "Address to register",
				},
			}, options.RPC...),
		},
		{
			Name:      "unregister",
			Usage:     "unregister self as a candidate",
			UsageText: "unregister -w <path> -r <rpc> -a <addr>",
			Action:    handleUnregister,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				gasFlag,
				flags.AddressFlag{
					Name:  "address, a",
					Usage: "Address to unregister",
				},
			}, options.RPC...),
		},
		{
			Name:      "vote",
			Usage:     "vote for a validator",
			UsageText: "vote -w <path> -r <rpc> [-s <timeout>] [-g gas] -a <addr> [-c <public key>]",
			Description: `Votes for a validator by calling "vote" method of a NEO native
   contract. Do not provide candidate argument to perform unvoting.
`,
			Action: handleVote,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				gasFlag,
				flags.AddressFlag{
					Name:  "address, a",
					Usage: "Address to vote from",
				},
				cli.StringFlag{
					Name:  "candidate, c",
					Usage: "Public key of candidate to vote for",
				},
			}, options.RPC...),
		},
	}
}

func handleRegister(ctx *cli.Context) error {
	return handleNeoAction(ctx, func(contract *neo.Contract, _ util.Uint160, acc *wallet.Account) (*transaction.Transaction, error) {
		return contract.RegisterCandidateUnsigned(acc.PublicKey())
	})
}

func handleUnregister(ctx *cli.Context) error {
	return handleNeoAction(ctx, func(contract *neo.Contract, _ util.Uint160, acc *wallet.Account) (*transaction.Transaction, error) {
		return contract.UnregisterCandidateUnsigned(acc.PublicKey())
	})
}

func handleNeoAction(ctx *cli.Context, mkTx func(*neo.Contract, util.Uint160, *wallet.Account) (*transaction.Transaction, error)) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	addr := addrFlag.Uint160()
	acc, err := getDecryptedAccount(wall, addr, pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	act, err := actor.NewSimple(c, acc)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("RPC actor issue: %w", err), 1)
	}

	gas := flags.Fixed8FromContext(ctx, "gas")
	contract := neo.New(act)
	tx, err := mkTx(contract, addr, acc)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	tx.NetworkFee += int64(gas)
	res, _, err := act.SignAndSend(tx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to sign/send transaction: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, res.StringLE())
	return nil
}

func handleVote(ctx *cli.Context) error {
	return handleNeoAction(ctx, func(contract *neo.Contract, addr util.Uint160, acc *wallet.Account) (*transaction.Transaction, error) {
		var (
			err error
			pub *keys.PublicKey
		)
		pubStr := ctx.String("candidate")
		if pubStr != "" {
			pub, err = keys.NewPublicKeyFromString(pubStr)
			if err != nil {
				return nil, fmt.Errorf("invalid public key: '%s'", pubStr)
			}
		}

		return contract.VoteUnsigned(addr, pub)
	})
}

// getDecryptedAccount tries to get and unlock the specified account if it has a
// key inside (otherwise it's returned as is, without an ability to sign). If
// password is nil, it will be requested via terminal.
func getDecryptedAccount(wall *wallet.Wallet, addr util.Uint160, password *string) (*wallet.Account, error) {
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("can't find account for the address: %s", address.Uint160ToString(addr))
	}

	// No private key available, nothing to decrypt, but it's still a useful account for many purposes.
	if acc.EncryptedWIF == "" {
		return acc, nil
	}

	if password == nil {
		pass, err := input.ReadPassword(EnterPasswordPrompt)
		if err != nil {
			fmt.Println("Error reading password", err)
			return nil, err
		}
		password = &pass
	}
	err := acc.Decrypt(*password, wall.Scrypt)
	if err != nil {
		return nil, err
	}
	return acc, nil
}
