package wallet

import (
	"crypto/elliptic"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/urfave/cli/v2"
)

// signMessageResult contains the output of the sign-msg command.
type signMessageResult struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Salt      string `json:"salt"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

var (
	signMsgFlags = []cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		&flags.AddressFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "Account address to sign with (default account if not specified)",
		},
		&cli.BoolFlag{
			Name:  "hex",
			Usage: "Treat the message argument as a hex-encoded byte string",
		},
		&cli.BoolFlag{
			Name:  "base64",
			Usage: "Treat the message argument as a base64-encoded byte string",
		},
	}
	verifyMsgFlags = []cli.Flag{
		walletPathFlag,
		walletConfigFlag,
		&flags.AddressFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "Account address to verify the signature for",
		},
		&cli.StringFlag{
			Name:  "public-key",
			Usage: "Public key to verify the signature with (hex-encoded compressed or uncompressed); address takes priority over public-key",
		},
		&cli.StringFlag{
			Name:     "salt",
			Required: true,
			Usage:    "16-byte hex-encoded random salt used during signing",
		},
		&cli.StringFlag{
			Name:     "signature",
			Required: true,
			Usage:    "64-byte hex-encoded signature",
		},
		&cli.BoolFlag{
			Name:  "hex",
			Usage: "Treat the message argument as a hex-encoded byte string",
		},
		&cli.BoolFlag{
			Name:  "base64",
			Usage: "Treat the message argument as a base64-encoded byte string",
		},
	}
)

func signMessage(ctx *cli.Context) error {
	msg, err := readMessageFromCtx(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}

	acc, wall, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer wall.Close()

	if !acc.CanSign() {
		return cli.Exit("account cannot sign: key not unlocked or account is locked", 1)
	}

	sigWithSalt, err := acc.PrivateKey().SignWalletConnect(msg)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to sign message: %w", err), 1)
	}

	sig := sigWithSalt[:keys.SignatureLen]
	salt := sigWithSalt[keys.SignatureLen:]

	res := signMessageResult{
		Address:   acc.Address,
		PublicKey: hex.EncodeToString(acc.PublicKey().Bytes()),
		Salt:      hex.EncodeToString(salt),
		Message:   base64.StdEncoding.EncodeToString(msg),
		Signature: hex.EncodeToString(sig),
	}
	txt, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to marshal result: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, string(txt))
	return nil
}

func verifyMessage(ctx *cli.Context) error {
	msg, err := readMessageFromCtx(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}

	saltHex := ctx.String("salt")
	if len(saltHex) != keys.WalletConnectSaltLen*2 {
		return cli.Exit(fmt.Errorf("invalid salt length: expected %d hex characters (%d bytes), got %d",
			keys.WalletConnectSaltLen*2, keys.WalletConnectSaltLen, len(saltHex)), 1)
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to decode salt: %w", err), 1)
	}

	sigHex := ctx.String("signature")
	if len(sigHex) != keys.SignatureLen*2 {
		return cli.Exit(fmt.Errorf("invalid signature length: expected %d hex characters (%d bytes), got %d",
			keys.SignatureLen*2, keys.SignatureLen, len(sigHex)), 1)
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to decode signature: %w", err), 1)
	}
	sigWithSalt := append(sig, salt...)

	var pub *keys.PublicKey
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		pub, err = getPublicKeyByAddress(ctx, addrFlag)
		if err != nil {
			return cli.Exit(err, 1)
		}
	} else if ctx.IsSet("public-key") {
		pub, err = keys.NewPublicKeyFromString(ctx.String("public-key"))
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to decode public key: %w", err), 1)
		}
	} else {
		return cli.Exit("either --address or --public-key must be specified", 1)
	}

	if pub.VerifyWalletConnect(msg, sigWithSalt) {
		fmt.Fprintln(ctx.App.Writer, "Signature is correct")
		return nil
	}
	return cli.Exit("Signature is incorrect", 1)
}

// getPublicKeyByAddress retrieves the public key for the given address from the
// wallet stored in stdin ('-') or provided via wallet flags. If neither wallet
// flag is set, it returns an error asking the user to use --public-key instead.
// It first tries to extract the public key from the verification script without
// requiring decryption of the private key.
func getPublicKeyByAddress(ctx *cli.Context, addrFlag *flags.Address) (*keys.PublicKey, error) {
	wPath, walletConfigPath := ctx.String("wallet"), ctx.String("wallet-config")
	if len(wPath) == 0 && len(walletConfigPath) == 0 {
		return nil, fmt.Errorf("wallet is required to look up a public key by address; use --public-key instead or provide a wallet")
	}
	wall, _, err := readWallet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet: %w", err)
	}
	defer wall.Close()

	acc := wall.GetAccount(addrFlag.Uint160())
	if acc == nil {
		return nil, fmt.Errorf("account not found in wallet for address %s", addrFlag.String())
	}
	// First, try to extract the public key from the verification script (no decryption needed).
	if acc.Contract != nil {
		if pubBytes, ok := scparser.ParseSignatureContract(acc.Contract.Script); ok {
			pub, err := keys.NewPublicKeyFromBytes(pubBytes, elliptic.P256())
			if err == nil {
				return pub, nil
			}
		}
	}
	// If the key is encrypted, decrypt it to get the public key.
	if acc.EncryptedWIF != "" {
		pass, err := input.ReadPassword(fmt.Sprintf("Enter account %s password > ", addrFlag.String()))
		if err != nil {
			return nil, fmt.Errorf("error reading password: %w", err)
		}
		if err := acc.Decrypt(pass, wall.Scrypt); err != nil {
			return nil, fmt.Errorf("failed to decrypt account: %w", err)
		}
		if acc.CanSign() {
			return acc.PublicKey(), nil
		}
	}
	return nil, fmt.Errorf("account for address %s has no usable public key", addrFlag.String())
}

// readMessageFromCtx reads the message from the command-line context.
// The message is the first positional argument. If --hex is set, it's decoded
// from hex; if --base64 is set, it's decoded from base64.
func readMessageFromCtx(ctx *cli.Context) ([]byte, error) {
	if ctx.NArg() != 1 {
		return nil, fmt.Errorf("exactly one message argument is required")
	}
	msgStr := ctx.Args().First()
	if ctx.Bool("hex") {
		return hex.DecodeString(msgStr)
	}
	if ctx.Bool("base64") {
		return base64.StdEncoding.DecodeString(msgStr)
	}
	return []byte(msgStr), nil
}
