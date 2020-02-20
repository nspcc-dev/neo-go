package wallet

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/CityOfZion/neo-go/pkg/wallet"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	errNoPath         = errors.New("target path where the wallet should be stored is mandatory and should be passed using (--path, -p) flags")
	errPhraseMismatch = errors.New("the entered pass-phrases do not match. Maybe you have misspelled them")
)

var (
	walletPathFlag = cli.StringFlag{
		Name:  "path, p",
		Usage: "Target location of the wallet file.",
	}
)

// NewCommands returns 'wallet' command.
func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:  "wallet",
		Usage: "create, open and manage a NEO wallet",
		Subcommands: []cli.Command{
			{
				Name:   "create",
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
				Name:   "dump",
				Usage:  "check and dump an existing NEO wallet",
				Action: dumpWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					cli.BoolFlag{
						Name:  "decrypt, d",
						Usage: "Decrypt encrypted keys.",
					},
				},
			},
		},
	}}
}

func dumpWallet(ctx *cli.Context) error {
	path := ctx.String("path")
	if len(path) == 0 {
		return cli.NewExitError(errNoPath, 1)
	}
	wall, err := wallet.NewWalletFromFile(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if ctx.Bool("decrypt") {
		pass, err := readPassword("Enter wallet password > ")
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		for i := range wall.Accounts {
			// Just testing the decryption here.
			err := wall.Accounts[i].Decrypt(pass)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}
	}
	fmtPrintWallet(wall)
	return nil
}

func createWallet(ctx *cli.Context) error {
	path := ctx.String("path")
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
		if err := createAccount(ctx, wall); err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmtPrintWallet(wall)
	fmt.Printf("wallet successfully created, file location is %s\n", wall.Path())
	return nil
}

func createAccount(ctx *cli.Context, wall *wallet.Wallet) error {
	buf := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the name of the account > ")
	rawName, _ := buf.ReadBytes('\n')
	phrase, err := readPassword("Enter passphrase > ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	phraseCheck, err := readPassword("Confirm passphrase > ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if phrase != phraseCheck {
		return errPhraseMismatch
	}

	name := strings.TrimRight(string(rawName), "\n")
	return wall.CreateAccount(name, phrase)
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	rawPass, err := terminal.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(rawPass), "\n"), nil
}

func fmtPrintWallet(wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Println("")
	fmt.Println(string(b))
	fmt.Println("")
}
