package smartcontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

func manifestAddGroup(ctx *cli.Context) error {
	walletPath := ctx.String("wallet")
	if len(walletPath) == 0 {
		return cli.NewExitError(errNoWallet, 1)
	}

	w, err := wallet.NewWalletFromFile(walletPath)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer w.Close()

	addr, err := flags.ParseAddress(ctx.String("account"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("account is invalid or missing: %w", err), 1)
	}

	sender, err := flags.ParseAddress(ctx.String("sender"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid sender: %w", err), 1)
	}

	nf, _, err := readNEFFile(ctx.String("nef"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read NEF file: %w", err), 1)
	}

	mPath := ctx.String("manifest")
	m, _, err := readManifest(mPath)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read contract manifest: %w", err), 1)
	}

	h := state.CreateContractHash(sender, nf.Checksum, m.Name)

	gAcc, err := getUnlockedAccount(w, addr)
	if err != nil {
		return err
	}

	var found bool

	sig := gAcc.PrivateKey().Sign(h.BytesBE())
	pub := gAcc.PrivateKey().PublicKey()
	for i := range m.Groups {
		if m.Groups[i].PublicKey.Equal(pub) {
			m.Groups[i].Signature = sig
			found = true
			break
		}
	}
	if !found {
		m.Groups = append(m.Groups, manifest.Group{
			PublicKey: pub,
			Signature: sig,
		})
	}

	rawM, err := json.Marshal(m)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't marshal manifest: %w", err), 1)
	}

	err = ioutil.WriteFile(mPath, rawM, os.ModePerm)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't write manifest file: %w", err), 1)
	}
	return nil
}

func readNEFFile(filename string) (*nef.File, []byte, error) {
	if len(filename) == 0 {
		return nil, nil, errors.New("no nef file was provided")
	}

	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return nil, nil, fmt.Errorf("can't parse NEF file: %w", err)
	}

	return &nefFile, f, nil
}

func readManifest(filename string) (*manifest.Manifest, []byte, error) {
	if len(filename) == 0 {
		return nil, nil, errNoManifestFile
	}

	manifestBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	m := new(manifest.Manifest)
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return nil, nil, err
	}
	return m, manifestBytes, nil
}
