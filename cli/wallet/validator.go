package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
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
	return handleCandidate(ctx, true)
}

func handleUnregister(ctx *cli.Context) error {
	return handleCandidate(ctx, false)
}

func handleCandidate(ctx *cli.Context, register bool) error {
	const (
		regMethod   = "registerCandidate"
		unregMethod = "unregisterCandidate"
	)
	var (
		err    error
		script []byte
		sysGas int64
	)

	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

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
	neoContractHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return err
	}
	unregScript, err := smartcontract.CreateCallWithAssertScript(neoContractHash, unregMethod, acc.PrivateKey().PublicKey().Bytes())
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if !register {
		script = unregScript
	} else {
		script, err = smartcontract.CreateCallWithAssertScript(neoContractHash, regMethod, acc.PrivateKey().PublicKey().Bytes())
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	// Registration price is normally much bigger than MaxGasInvoke, so to
	// determine proper amount of GAS we _always_ run unreg script and then
	// add registration price to it if needed.
	r, err := act.Run(unregScript)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Run failure: %w", err), 1)
	}
	sysGas = r.GasConsumed
	if register {
		// Deregistration will fail, so there is no point in checking State.
		regPrice, err := c.GetCandidateRegisterPrice()
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		sysGas += regPrice
	} else if r.State != vmstate.Halt.String() {
		return cli.NewExitError(fmt.Errorf("unregister transaction failed: %s", r.FaultException), 1)
	}
	res, _, err := act.SendUncheckedRun(script, sysGas, nil, func(t *transaction.Transaction) error {
		t.NetworkFee += int64(gas)
		return nil
	})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to push transaction: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, res.StringLE())
	return nil
}

func handleVote(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	wall, pass, err := readWallet(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	addr := addrFlag.Uint160()
	acc, err := getDecryptedAccount(wall, addr, pass)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var pub *keys.PublicKey
	pubStr := ctx.String("candidate")
	if pubStr != "" {
		pub, err = keys.NewPublicKeyFromString(pubStr)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid public key: '%s'", pubStr), 1)
		}
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var pubArg interface{}
	if pub != nil {
		pubArg = pub.Bytes()
	}

	gas := flags.Fixed8FromContext(ctx, "gas")
	neoContractHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	script, err := smartcontract.CreateCallWithAssertScript(neoContractHash, "vote", addr.BytesBE(), pubArg)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	res, err := c.SignAndPushInvocationTx(script, acc, -1, gas, []rpcclient.SignerAccount{{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
		Signer: transaction.Signer{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc}})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to push invocation transaction: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, res.StringLE())
	return nil
}

// getDecryptedAccount tries to unlock the specified account. If password is nil, it will be requested via terminal.
func getDecryptedAccount(wall *wallet.Wallet, addr util.Uint160, password *string) (*wallet.Account, error) {
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("can't find account for the address: %s", address.Uint160ToString(addr))
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
