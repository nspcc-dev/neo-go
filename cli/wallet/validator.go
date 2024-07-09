package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli/v2"
)

func newValidatorCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "register",
			Usage:     "Register as a new candidate",
			UsageText: "register -w <path> -r <rpc> -a <addr> [-g gas] [-e sysgas] [--out file] [--force] [--await]",
			Action:    handleRegister,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				txctx.GasFlag,
				txctx.SysGasFlag,
				txctx.OutFlag,
				txctx.ForceFlag,
				txctx.AwaitFlag,
				&flags.AddressFlag{
					Name:    "address",
					Aliases: []string{"a"},
					Usage:   "Address to register",
				},
			}, options.RPC...),
		},
		{
			Name:      "unregister",
			Usage:     "Unregister self as a candidate",
			UsageText: "unregister -w <path> -r <rpc> -a <addr> [-g gas] [-e sysgas] [--out file] [--force] [--await]",
			Action:    handleUnregister,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				txctx.GasFlag,
				txctx.SysGasFlag,
				txctx.OutFlag,
				txctx.ForceFlag,
				txctx.AwaitFlag,
				&flags.AddressFlag{
					Name:    "address",
					Aliases: []string{"a"},
					Usage:   "Address to unregister",
				},
			}, options.RPC...),
		},
		{
			Name:      "vote",
			Usage:     "Vote for a validator",
			UsageText: "vote -w <path> -r <rpc> [-s <timeout>] [-g gas] [-e sysgas] -a <addr> [-c <public key>] [--out file] [--force] [--await]",
			Description: `Votes for a validator by calling "vote" method of a NEO native
   contract. Do not provide candidate argument to perform unvoting. If --await flag is 
   included, the command waits for the transaction to be included in a block before exiting.
`,
			Action: handleVote,
			Flags: append([]cli.Flag{
				walletPathFlag,
				walletConfigFlag,
				txctx.GasFlag,
				txctx.SysGasFlag,
				txctx.OutFlag,
				txctx.ForceFlag,
				txctx.AwaitFlag,
				&flags.AddressFlag{
					Name:    "address",
					Aliases: []string{"a"},
					Usage:   "Address to vote from",
				},
				&cli.StringFlag{
					Name:    "candidate",
					Aliases: []string{"c"},
					Usage:   "Public key of candidate to vote for",
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
		return cli.Exit(err, 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.Exit("address was not provided", 1)
	}
	addr := addrFlag.Uint160()
	acc, err := options.GetUnlockedAccount(wall, addr, pass)
	if err != nil {
		return cli.Exit(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	signers, err := cmdargs.GetSignersAccounts(acc, wall, nil, transaction.CalledByEntry)
	if err != nil {
		return cli.Exit(fmt.Errorf("invalid signers: %w", err), 1)
	}
	_, act, exitErr := options.GetRPCWithActor(gctx, ctx, signers)
	if exitErr != nil {
		return exitErr
	}

	contract := neo.New(act)
	tx, err := mkTx(contract, addr, acc)
	if err != nil {
		return cli.Exit(err, 1)
	}
	return txctx.SignAndSend(ctx, act, acc, tx)
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
