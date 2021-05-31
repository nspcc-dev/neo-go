package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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
		{
			Name:      "getstate",
			Usage:     "print NEO holder account state",
			UsageText: "getstate -a <addr>",
			Action:    getAccountState,
			Flags: append([]cli.Flag{
				flags.AddressFlag{
					Name:  "address, a",
					Usage: "Address to get state of",
				},
			}, options.RPC...),
		},
	}
}

func handleRegister(ctx *cli.Context) error {
	return handleCandidate(ctx, "registerCandidate", 1001*100000000) // registering costs 1000 GAS
}

func handleUnregister(ctx *cli.Context) error {
	return handleCandidate(ctx, "unregisterCandidate", -1)
}

func handleCandidate(ctx *cli.Context, method string, sysGas int64) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	addr := addrFlag.Uint160()
	acc, err := getDecryptedAccount(ctx, wall, addr)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gas := flags.Fixed8FromContext(ctx, "gas")
	neoContractHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return err
	}
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, neoContractHash, method, callflag.States, acc.PrivateKey().PublicKey().Bytes())
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	res, err := c.SignAndPushInvocationTx(w.Bytes(), acc, sysGas, gas, []client.SignerAccount{{
		Signer: transaction.Signer{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc,
	}})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to push transaction: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, res.StringLE())
	return nil
}

func handleVote(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	addr := addrFlag.Uint160()
	acc, err := getDecryptedAccount(ctx, wall, addr)
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
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, neoContractHash, "vote", callflag.States, addr.BytesBE(), pubArg)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)

	res, err := c.SignAndPushInvocationTx(w.Bytes(), acc, -1, gas, []client.SignerAccount{{
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

func getDecryptedAccount(ctx *cli.Context, wall *wallet.Wallet, addr util.Uint160) (*wallet.Account, error) {
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("can't find account for the address: %s", address.Uint160ToString(addr))
	}

	if pass, err := input.ReadPassword("Password > "); err != nil {
		fmt.Println("ERROR", pass, err)
		return nil, err
	} else if err := acc.Decrypt(pass); err != nil {
		return nil, err
	}
	return acc, nil
}

func getAccountState(ctx *cli.Context) error {
	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	c, exitErr := options.GetRPCClient(gctx, ctx)
	if exitErr != nil {
		return exitErr
	}

	neoHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get NEO contract hash: %w", err), 1)
	}
	res, err := c.InvokeFunction(neoHash, "getAccountState", []smartcontract.Parameter{
		{
			Type:  smartcontract.Hash160Type,
			Value: addrFlag.Uint160(),
		},
	}, nil)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if res.State != "HALT" {
		return cli.NewExitError(fmt.Errorf("invocation failed: %s", res.FaultException), 1)
	}
	if len(res.Stack) == 0 {
		return cli.NewExitError("result stack is empty", 1)
	}
	st := new(state.NEOBalanceState)
	err = st.FromStackItem(res.Stack[0])
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to convert account state from stackitem: %w", err), 1)
	}
	dec, err := c.NEP17Decimals(neoHash)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get decimals: %w", err), 1)
	}
	voted := "null"
	if st.VoteTo != nil {
		voted = address.Uint160ToString(st.VoteTo.GetScriptHash())
	}
	fmt.Fprintf(ctx.App.Writer, "\tVoted: %s\n", voted)
	fmt.Fprintf(ctx.App.Writer, "\tAmount : %s\n", fixedn.ToString(&st.Balance, int(dec)))
	fmt.Fprintf(ctx.App.Writer, "\tBlock: %d\n", st.BalanceHeight)
	return nil
}
