package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

var (
	errNoPath         = errors.New("wallet path is mandatory and should be passed using (--wallet, -w) flags")
	errPhraseMismatch = errors.New("the entered pass-phrases do not match. Maybe you have misspelled them")
	errNoStdin        = errors.New("can't read wallet from stdin for this command")
)

var (
	walletPathFlag = cli.StringFlag{
		Name:  "wallet, w",
		Usage: "Target location of the wallet file ('-' to read from stdin).",
	}
	wifFlag = cli.StringFlag{
		Name:  "wif",
		Usage: "WIF to import",
	}
	decryptFlag = cli.BoolFlag{
		Name:  "decrypt, d",
		Usage: "Decrypt encrypted keys.",
	}
	outFlag = cli.StringFlag{
		Name:  "out",
		Usage: "file to put JSON transaction to",
	}
	inFlag = cli.StringFlag{
		Name:  "in",
		Usage: "file with JSON transaction",
	}
	fromAddrFlag = flags.AddressFlag{
		Name:  "from",
		Usage: "Address to send an asset from",
	}
	toAddrFlag = flags.AddressFlag{
		Name:  "to",
		Usage: "Address to send an asset to",
	}
	forceFlag = cli.BoolFlag{
		Name:  "force",
		Usage: "Do not ask for a confirmation",
	}
)

// NewCommands returns 'wallet' command.
func NewCommands() []cli.Command {
	claimFlags := []cli.Flag{
		walletPathFlag,
		flags.AddressFlag{
			Name:  "address, a",
			Usage: "Address to claim GAS for",
		},
	}
	claimFlags = append(claimFlags, options.RPC...)
	signFlags := []cli.Flag{
		walletPathFlag,
		outFlag,
		inFlag,
		flags.AddressFlag{
			Name:  "address, a",
			Usage: "Address to use",
		},
	}
	signFlags = append(signFlags, options.RPC...)
	return []cli.Command{{
		Name:  "wallet",
		Usage: "create, open and manage a NEO wallet",
		Subcommands: []cli.Command{
			{
				Name:   "claim",
				Usage:  "claim GAS",
				Action: claimGas,
				Flags:  claimFlags,
			},
			{
				Name:   "init",
				Usage:  "create a new wallet",
				Action: createWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					cli.BoolFlag{
						Name:  "account, a",
						Usage: "Create a new account",
					},
				},
			},
			{
				Name:   "change-password",
				Usage:  "change password for accounts",
				Action: changePassword,
				Flags: []cli.Flag{
					walletPathFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "address to change password for",
					},
				},
			},
			{
				Name:   "convert",
				Usage:  "convert addresses from existing NEO2 NEP6-wallet to NEO3 format",
				Action: convertWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					cli.StringFlag{
						Name:  "out, o",
						Usage: "where to write converted wallet",
					},
				},
			},
			{
				Name:   "create",
				Usage:  "add an account to the existing wallet",
				Action: addAccount,
				Flags: []cli.Flag{
					walletPathFlag,
				},
			},
			{
				Name:   "dump",
				Usage:  "check and dump an existing NEO wallet",
				Action: dumpWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					decryptFlag,
				},
			},
			{
				Name:   "dump-keys",
				Usage:  "dump public keys for account",
				Action: dumpKeys,
				Flags: []cli.Flag{
					walletPathFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "address to print public keys for",
					},
				},
			},
			{
				Name:      "export",
				Usage:     "export keys for address",
				UsageText: "export --wallet <path> [--decrypt] [<address>]",
				Action:    exportKeys,
				Flags: []cli.Flag{
					walletPathFlag,
					decryptFlag,
				},
			},
			{
				Name:      "import",
				Usage:     "import WIF of a standard signature contract",
				UsageText: "import --wallet <path> --wif <wif> [--name <account_name>]",
				Action:    importWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					wifFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
					cli.StringFlag{
						Name:  "contract",
						Usage: "Verification script for custom contracts",
					},
				},
			},
			{
				Name:  "import-multisig",
				Usage: "import multisig contract",
				UsageText: "import-multisig --wallet <path> --wif <wif> [--name <account_name>] --min <n>" +
					" [<pubkey1> [<pubkey2> [...]]]",
				Action: importMultisig,
				Flags: []cli.Flag{
					walletPathFlag,
					wifFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
					cli.IntFlag{
						Name:  "min, m",
						Usage: "Minimal number of signatures",
					},
				},
			},
			{
				Name:      "import-deployed",
				Usage:     "import deployed contract",
				UsageText: "import-deployed --wallet <path> --wif <wif> --contract <hash> [--name <account_name>]",
				Action:    importDeployed,
				Flags: append([]cli.Flag{
					walletPathFlag,
					wifFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
					flags.AddressFlag{
						Name:  "contract, c",
						Usage: "Contract hash or address",
					},
				}, options.RPC...),
			},
			{
				Name:      "remove",
				Usage:     "remove an account from the wallet",
				UsageText: "remove --wallet <path> [--force] --address <addr>",
				Action:    removeAccount,
				Flags: []cli.Flag{
					walletPathFlag,
					forceFlag,
					flags.AddressFlag{
						Name:  "address, a",
						Usage: "Account address or hash in LE form to be removed",
					},
				},
			},
			{
				Name:      "sign",
				Usage:     "cosign transaction with multisig/contract/additional account",
				UsageText: "sign --wallet <path> --address <address> --in <file.in> --out <file.out> [-r <endpoint>]",
				Action:    signStoredTransaction,
				Flags:     signFlags,
			},
			{
				Name:        "nep17",
				Usage:       "work with NEP-17 contracts",
				Subcommands: newNEP17Commands(),
			},
			{
				Name:        "nep11",
				Usage:       "work with NEP-11 contracts",
				Subcommands: newNEP11Commands(),
			},
			{
				Name:        "candidate",
				Usage:       "work with candidates",
				Subcommands: newValidatorCommands(),
			},
		},
	}}
}

func claimGas(ctx *cli.Context) error {
	wall, err := readWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addrFlag := ctx.Generic("address").(*flags.Address)
	if !addrFlag.IsSet {
		return cli.NewExitError("address was not provided", 1)
	}
	scriptHash := addrFlag.Uint160()
	acc, err := getDecryptedAccount(ctx, wall, scriptHash)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	neoContractHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	hash, err := c.TransferNEP17(acc, scriptHash, neoContractHash, 0, 0, nil, nil)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Fprintln(ctx.App.Writer, hash.StringLE())
	return nil
}

func changePassword(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		// Check for account presence first before asking for password.
		acc := wall.GetAccount(addrFlag.Uint160())
		if acc == nil {
			return cli.NewExitError("account is missing", 1)
		}
	}

	oldPass, err := input.ReadPassword("Enter password > ")
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Error reading old password: %w", err), 1)
	}

	for i := range wall.Accounts {
		if addrFlag.IsSet && wall.Accounts[i].Address != addrFlag.String() {
			continue
		}
		err := wall.Accounts[i].Decrypt(oldPass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("unable to decrypt account %s: %w", wall.Accounts[i].Address, err), 1)
		}
	}

	pass, err := readNewPassword()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Error reading new password: %w", err), 1)
	}
	for i := range wall.Accounts {
		if addrFlag.IsSet && wall.Accounts[i].Address != addrFlag.String() {
			continue
		}
		err := wall.Accounts[i].Encrypt(pass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	err = wall.Save()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("Error saving the wallet: %w", err), 1)
	}
	return nil
}

func convertWallet(ctx *cli.Context) error {
	wall, err := newWalletV2FromFile(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	newWallet, err := wallet.NewWallet(ctx.String("out"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer newWallet.Close()
	newWallet.Scrypt = wall.Scrypt

	for _, acc := range wall.Accounts {
		pass, err := input.ReadPassword(fmt.Sprintf("Enter passphrase for account %s (label '%s') > ", acc.Address, acc.Label))
		if err != nil {
			return cli.NewExitError(fmt.Errorf("Error reading password: %w", err), 1)
		}
		newAcc, err := acc.convert(pass, wall.Scrypt)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		newWallet.AddAccount(newAcc)
	}
	if err := newWallet.Save(); err != nil {
		return cli.NewExitError(err, -1)
	}
	return nil
}

func addAccount(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	if err := createAccount(wall); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func exportKeys(ctx *cli.Context) error {
	wall, err := readWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var addr string

	decrypt := ctx.Bool("decrypt")
	if ctx.NArg() == 0 && decrypt {
		return cli.NewExitError(errors.New("address must be provided if '--decrypt' flag is used"), 1)
	} else if ctx.NArg() > 0 {
		// check address format just to catch possible typos
		addr = ctx.Args().First()
		_, err := address.StringToUint160(addr)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't parse address: %w", err), 1)
		}
	}

	var wifs []string

loop:
	for _, a := range wall.Accounts {
		if addr != "" && a.Address != addr {
			continue
		}

		for i := range wifs {
			if a.EncryptedWIF == wifs[i] {
				continue loop
			}
		}

		wifs = append(wifs, a.EncryptedWIF)
	}

	for _, wif := range wifs {
		if decrypt {
			pass, err := input.ReadPassword("Enter password > ")
			if err != nil {
				return cli.NewExitError(fmt.Errorf("Error reading password: %w", err), 1)
			}

			pk, err := keys.NEP2Decrypt(wif, pass, wall.Scrypt)
			if err != nil {
				return cli.NewExitError(err, 1)
			}

			wif = pk.WIF()
		}

		fmt.Fprintln(ctx.App.Writer, wif)
	}

	return nil
}

func importMultisig(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	m := ctx.Int("min")
	if ctx.NArg() < m {
		return cli.NewExitError(errors.New("insufficient number of public keys"), 1)
	}

	args := []string(ctx.Args())
	pubs := make([]*keys.PublicKey, len(args))

	for i := range args {
		pubs[i], err = keys.NewPublicKeyFromString(args[i])
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't decode public key %d: %w", i, err), 1)
		}
	}

	acc, err := newAccountFromWIF(ctx.App.Writer, ctx.String("wif"), wall.Scrypt)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if err := acc.ConvertMultisig(m, pubs); err != nil {
		return cli.NewExitError(err, 1)
	}

	if acc.Label == "" {
		acc.Label = ctx.String("name")
	}
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func importDeployed(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	rawHash := ctx.Generic("contract").(*flags.Address)
	if !rawHash.IsSet {
		return cli.NewExitError("contract hash was not provided", 1)
	}

	acc, err := newAccountFromWIF(ctx.App.Writer, ctx.String("wif"), wall.Scrypt)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	cs, err := c.GetContractStateByHash(rawHash.Uint160())
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't fetch contract info: %w", err), 1)
	}
	md := cs.Manifest.ABI.GetMethod(manifest.MethodVerify, -1)
	if md == nil || md.ReturnType != smartcontract.BoolType {
		return cli.NewExitError("contract has no `verify` method with boolean return", 1)
	}
	acc.Address = address.Uint160ToString(cs.Hash)
	acc.Contract.Script = cs.NEF.Script
	acc.Contract.Parameters = acc.Contract.Parameters[:0]
	for _, p := range md.Parameters {
		acc.Contract.Parameters = append(acc.Contract.Parameters, wallet.ContractParam{
			Name: p.Name,
			Type: p.Type,
		})
	}
	acc.Contract.Deployed = true

	if acc.Label == "" {
		acc.Label = ctx.String("name")
	}
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func importWallet(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	acc, err := newAccountFromWIF(ctx.App.Writer, ctx.String("wif"), wall.Scrypt)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctrFlag := ctx.String("contract"); ctrFlag != "" {
		ctr, err := hex.DecodeString(ctrFlag)
		if err != nil {
			return cli.NewExitError("invalid contract", 1)
		}
		acc.Contract.Script = ctr
	}

	if acc.Label == "" {
		acc.Label = ctx.String("name")
	}
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func removeAccount(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addr := ctx.Generic("address").(*flags.Address)
	if !addr.IsSet {
		return cli.NewExitError("valid account address must be provided", 1)
	}
	acc := wall.GetAccount(addr.Uint160())
	if acc == nil {
		return cli.NewExitError("account wasn't found", 1)
	}

	if !ctx.Bool("force") {
		fmt.Fprintf(ctx.App.Writer, "Account %s will be removed. This action is irreversible.\n", addr.Uint160())
		if ok := askForConsent(ctx.App.Writer); !ok {
			return nil
		}
	}

	if err := wall.RemoveAccount(acc.Address); err != nil {
		return cli.NewExitError(fmt.Errorf("error on remove: %w", err), 1)
	} else if err := wall.Save(); err != nil {
		return cli.NewExitError(fmt.Errorf("error while saving wallet: %w", err), 1)
	}
	return nil
}

func askForConsent(w io.Writer) bool {
	response, err := input.ReadLine("Are you sure? [y/N]: ")
	if err == nil {
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		}
	}
	fmt.Fprintln(w, "Cancelled.")
	return false
}

func dumpWallet(ctx *cli.Context) error {
	wall, err := readWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if ctx.Bool("decrypt") {
		pass, err := input.ReadPassword("Enter wallet password > ")
		if err != nil {
			return cli.NewExitError(fmt.Errorf("Error reading password: %w", err), 1)
		}
		for i := range wall.Accounts {
			// Just testing the decryption here.
			err := wall.Accounts[i].Decrypt(pass, wall.Scrypt)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}
	}
	fmtPrintWallet(ctx.App.Writer, wall)
	return nil
}

func dumpKeys(ctx *cli.Context) error {
	wall, err := readWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	accounts := wall.Accounts

	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		acc := wall.GetAccount(addrFlag.Uint160())
		if acc == nil {
			return cli.NewExitError("account is missing", 1)
		}
		accounts = []*wallet.Account{acc}
	}

	hasPrinted := false
	for _, acc := range accounts {
		pub, ok := vm.ParseSignatureContract(acc.Contract.Script)
		if ok {
			if hasPrinted {
				fmt.Fprintln(ctx.App.Writer)
			}
			fmt.Fprintf(ctx.App.Writer, "%s (simple signature contract):\n", acc.Address)
			fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(pub))
			hasPrinted = true
			continue
		}
		n, bs, ok := vm.ParseMultiSigContract(acc.Contract.Script)
		if ok {
			if hasPrinted {
				fmt.Fprintln(ctx.App.Writer)
			}
			fmt.Fprintf(ctx.App.Writer, "%s (%d out of %d multisig contract):\n", acc.Address, n, len(bs))
			for i := range bs {
				fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(bs[i]))
			}
			hasPrinted = true
			continue
		}
		if addrFlag.IsSet {
			return cli.NewExitError(fmt.Errorf("unknown script type for address %s", address.Uint160ToString(addrFlag.Uint160())), 1)
		}
	}
	return nil
}

func createWallet(ctx *cli.Context) error {
	path := ctx.String("wallet")
	if len(path) == 0 {
		return cli.NewExitError(errNoPath, 1)
	}
	wall, err := wallet.NewWallet(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("account") {
		if err := createAccount(wall); err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmtPrintWallet(ctx.App.Writer, wall)
	fmt.Fprintf(ctx.App.Writer, "wallet successfully created, file location is %s\n", wall.Path())
	return nil
}

func readAccountInfo() (string, string, error) {
	name, err := input.ReadLine("Enter the name of the account > ")
	if err != nil {
		return "", "", err
	}
	phrase, err := readNewPassword()
	if err != nil {
		return "", "", err
	}
	return name, phrase, nil
}

func readNewPassword() (string, error) {
	phrase, err := input.ReadPassword("Enter passphrase > ")
	if err != nil {
		return "", fmt.Errorf("Error reading password: %w", err)
	}
	phraseCheck, err := input.ReadPassword("Confirm passphrase > ")
	if err != nil {
		return "", fmt.Errorf("Error reading password: %w", err)
	}

	if phrase != phraseCheck {
		return "", errPhraseMismatch
	}
	return phrase, nil
}

func createAccount(wall *wallet.Wallet) error {
	name, phrase, err := readAccountInfo()
	if err != nil {
		return err
	}
	return wall.CreateAccount(name, phrase)
}

func openWallet(path string) (*wallet.Wallet, error) {
	if len(path) == 0 {
		return nil, errNoPath
	}
	if path == "-" {
		return nil, errNoStdin
	}
	return wallet.NewWalletFromFile(path)
}

func readWallet(path string) (*wallet.Wallet, error) {
	if len(path) == 0 {
		return nil, errNoPath
	}
	if path == "-" {
		w := &wallet.Wallet{}
		if err := json.NewDecoder(os.Stdin).Decode(w); err != nil {
			return nil, fmt.Errorf("js %s", err)
		}
		return w, nil
	}
	return wallet.NewWalletFromFile(path)
}

func newAccountFromWIF(w io.Writer, wif string, scrypt keys.ScryptParams) (*wallet.Account, error) {
	// note: NEP2 strings always have length of 58 even though
	// base58 strings can have different lengths even if slice lengths are equal
	if len(wif) == 58 {
		pass, err := input.ReadPassword("Enter password > ")
		if err != nil {
			return nil, fmt.Errorf("Error reading password: %w", err)
		}

		return wallet.NewAccountFromEncryptedWIF(wif, pass, scrypt)
	}

	acc, err := wallet.NewAccountFromWIF(wif)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(w, "Provided WIF was unencrypted. Wallet can contain only encrypted keys.")
	name, pass, err := readAccountInfo()
	if err != nil {
		return nil, err
	}

	acc.Label = name
	if err := acc.Encrypt(pass, scrypt); err != nil {
		return nil, err
	}

	return acc, nil
}

func addAccountAndSave(w *wallet.Wallet, acc *wallet.Account) error {
	for i := range w.Accounts {
		if w.Accounts[i].Address == acc.Address {
			return fmt.Errorf("address '%s' is already in wallet", acc.Address)
		}
	}

	w.AddAccount(acc)
	return w.Save()
}

func fmtPrintWallet(w io.Writer, wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, string(b))
	fmt.Fprintln(w, "")
}
