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
					cli.StringFlag{
						Name:  "path, p",
						Usage: "Target location of the wallet file.",
					},
					cli.BoolFlag{
						Name:  "account, a",
						Usage: "Create a new account",
					},
				},
			},
			{
				Name:   "open",
				Usage:  "open a existing NEO wallet",
				Action: openWallet,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "path, p",
						Usage: "Target location of the wallet file.",
					},
				},
			},
		},
	}}
}

func openWallet(ctx *cli.Context) error {
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

	dumpWallet(wall)
	fmt.Printf("wallet successfully created, file location is %s\n", wall.Path())
	return nil
}

func createAccount(ctx *cli.Context, wall *wallet.Wallet) error {
	var (
		rawName,
		rawPhrase,
		rawPhraseCheck []byte
	)
	buf := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the name of the account > ")
	rawName, _ = buf.ReadBytes('\n')
	fmt.Print("Enter passphrase > ")
	rawPhrase, _ = terminal.ReadPassword(int(syscall.Stdin))
	fmt.Print("\nConfirm passphrase > ")
	rawPhraseCheck, _ = terminal.ReadPassword(int(syscall.Stdin))

	// Clean data
	var (
		name        = strings.TrimRight(string(rawName), "\n")
		phrase      = strings.TrimRight(string(rawPhrase), "\n")
		phraseCheck = strings.TrimRight(string(rawPhraseCheck), "\n")
	)

	if phrase != phraseCheck {
		return errPhraseMismatch
	}

	return wall.CreateAccount(name, phrase)
}

func dumpWallet(wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Println("")
	fmt.Println(string(b))
	fmt.Println("")
}
