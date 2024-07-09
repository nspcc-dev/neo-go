package smartcontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

func manifestAddGroup(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	sender, err := flags.ParseAddress(ctx.String("sender"))
	if err != nil {
		return cli.Exit(fmt.Errorf("invalid sender: %w", err), 1)
	}

	nf, _, err := readNEFFile(ctx.String("nef"))
	if err != nil {
		return cli.Exit(fmt.Errorf("can't read NEF file: %w", err), 1)
	}

	mPath := ctx.String("manifest")
	m, _, err := readManifest(mPath, util.Uint160{})
	if err != nil {
		return cli.Exit(fmt.Errorf("can't read contract manifest: %w", err), 1)
	}

	h := state.CreateContractHash(sender, nf.Checksum, m.Name)

	gAcc, w, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't get account to sign group with: %w", err), 1)
	}
	defer w.Close()

	var found bool

	sig := gAcc.PrivateKey().Sign(h.BytesBE())
	pub := gAcc.PublicKey()
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
		return cli.Exit(fmt.Errorf("can't marshal manifest: %w", err), 1)
	}

	err = os.WriteFile(mPath, rawM, os.ModePerm)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't write manifest file: %w", err), 1)
	}
	return nil
}

func readNEFFile(filename string) (*nef.File, []byte, error) {
	if len(filename) == 0 {
		return nil, nil, errors.New("no nef file was provided")
	}

	f, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return nil, nil, fmt.Errorf("can't parse NEF file: %w", err)
	}

	return &nefFile, f, nil
}

// readManifest unmarshalls manifest got from the provided filename and checks
// it for validness against the provided contract hash. If empty hash is specified
// then no hash-related manifest groups check is performed.
func readManifest(filename string, hash util.Uint160) (*manifest.Manifest, []byte, error) {
	if len(filename) == 0 {
		return nil, nil, errNoManifestFile
	}

	manifestBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	m := new(manifest.Manifest)
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return nil, nil, err
	}
	if err := m.IsValid(hash, true); err != nil {
		return nil, nil, fmt.Errorf("manifest is invalid: %w", err)
	}
	return m, manifestBytes, nil
}
