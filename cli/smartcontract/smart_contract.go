package smartcontract

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	errNoInput             = errors.New("no input file was found, specify an input file with the '--in or -i' flag")
	errNoConfFile          = errors.New("no config file was found, specify a config file with the '--config' or '-c' flag")
	errNoManifestFile      = errors.New("no manifest file was found, specify manifest file with '--manifest' or '-m' flag")
	errNoMethod            = errors.New("no method specified for function invocation command")
	errNoWallet            = errors.New("no wallet parameter found, specify it with the '--wallet or -w' flag")
	errNoScriptHash        = errors.New("no smart contract hash was provided, specify one as the first argument")
	errNoSmartContractName = errors.New("no name was provided, specify the '--name or -n' flag")
	errFileExist           = errors.New("A file with given smart-contract name already exists")

	walletFlag = cli.StringFlag{
		Name:  "wallet, w",
		Usage: "wallet to use to get the key for transaction signing",
	}
	addressFlag = flags.AddressFlag{
		Name:  "address, a",
		Usage: "address to use as transaction signee (and gas source)",
	}
	gasFlag = flags.Fixed8Flag{
		Name:  "gas, g",
		Usage: "network fee to add to the transaction (prioritizing it)",
	}
	sysGasFlag = flags.Fixed8Flag{
		Name:  "sysgas, e",
		Usage: "system fee to add to transaction (compensating for execution)",
	}
	outFlag = cli.StringFlag{
		Name:  "out",
		Usage: "file to put JSON transaction to",
	}
	forceFlag = cli.BoolFlag{
		Name:  "force",
		Usage: "force-push the transaction in case of bad VM state after test script invocation",
	}
)

const (
	// smartContractTmpl is written to a file when used with `init` command.
	// %s is parsed to be the smartContractName.
	smartContractTmpl = `package %s

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

var notificationName string

// init initializes notificationName before calling any other smart-contract method
func init() {
	notificationName = "Hello world!"
}

// RuntimeNotify sends runtime notification with "Hello world!" name
func RuntimeNotify(args []interface{}) {
    runtime.Notify(notificationName, args)
}`
)

// NewCommands returns 'contract' command.
func NewCommands() []cli.Command {
	testInvokeScriptFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "in, i",
			Usage: "Input location of the .nef file that needs to be invoked",
		},
	}
	testInvokeScriptFlags = append(testInvokeScriptFlags, options.RPC...)
	invokeFunctionFlags := []cli.Flag{
		walletFlag,
		addressFlag,
		gasFlag,
		sysGasFlag,
		outFlag,
		forceFlag,
	}
	invokeFunctionFlags = append(invokeFunctionFlags, options.RPC...)
	deployFlags := append(invokeFunctionFlags, []cli.Flag{
		cli.StringFlag{
			Name:  "in, i",
			Usage: "Input file for the smart contract (*.nef)",
		},
		cli.StringFlag{
			Name:  "manifest, m",
			Usage: "Manifest input file (*.manifest.json)",
		},
	}...)
	return []cli.Command{{
		Name:  "contract",
		Usage: "compile - debug - deploy smart contracts",
		Subcommands: []cli.Command{
			{
				Name:   "compile",
				Usage:  "compile a smart contract to a .nef file",
				Action: contractCompile,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input file for the smart contract to be compiled",
					},
					cli.StringFlag{
						Name:  "out, o",
						Usage: "Output of the compiled contract",
					},
					cli.BoolFlag{
						Name:  "verbose, v",
						Usage: "Print out additional information after a compiling",
					},
					cli.StringFlag{
						Name:  "debug, d",
						Usage: "Emit debug info in a separate file",
					},
					cli.StringFlag{
						Name:  "manifest, m",
						Usage: "Emit contract manifest (*.manifest.json) file into separate file using configuration input file (*.yml)",
					},
					cli.StringFlag{
						Name:  "config, c",
						Usage: "Configuration input file (*.yml)",
					},
					cli.BoolFlag{
						Name:  "no-standards",
						Usage: "do not check compliance with supported standards",
					},
					cli.BoolFlag{
						Name:  "no-events",
						Usage: "do not check emitted events with the manifest",
					},
					cli.BoolFlag{
						Name:  "no-permissions",
						Usage: "do not check if invoked contracts are allowed in manifest",
					},
				},
			},
			{
				Name:      "deploy",
				Usage:     "deploy a smart contract (.nef with description)",
				UsageText: "neo-go contract deploy -r endpoint -w wallet [-a address] [-g gas] [-e sysgas] --in contract.nef --manifest contract.manifest.json [--out file] [--force] [data]",
				Description: `Deploys given contract into the chain. The gas parameter is for additional
   gas to be added as a network fee to prioritize the transaction. The data 
   parameter is an optional parameter to be passed to '_deploy' method.
`,
				Action: contractDeploy,
				Flags:  deployFlags,
			},
			{
				Name:      "invokefunction",
				Usage:     "invoke deployed contract on the blockchain",
				UsageText: "neo-go contract invokefunction -r endpoint -w wallet [-a address] [-g gas] [-e sysgas] [--out file] [--force] scripthash [method] [arguments...] [--] [signers...]",
				Description: `Executes given (as a script hash) deployed script with the given method,
   arguments and signers. Sender is included in the list of signers by default
   with None witness scope. If you'd like to change default sender's scope, 
   specify it via signers parameter. See testinvokefunction documentation for 
   the details about parameters. It differs from testinvokefunction in that this
   command sends an invocation transaction to the network.
`,
				Action: invokeFunction,
				Flags:  invokeFunctionFlags,
			},
			{
				Name:      "testinvokefunction",
				Usage:     "invoke deployed contract on the blockchain (test mode)",
				UsageText: "neo-go contract testinvokefunction -r endpoint scripthash [method] [arguments...] [--] [signers...]",
				Description: `Executes given (as a script hash) deployed script with the given method,
   arguments and signers (sender is not included by default). If no method is given
   "" is passed to the script, if no arguments are given, an empty array is 
   passed, if no signers are given no array is passed. If signers are specified,
   the first one of them is treated as a sender. All of the given arguments are 
   encapsulated into array before invoking the script. The script thus should 
   follow the regular convention of smart contract arguments (method string and 
   an array of other arguments).

   Arguments always do have regular Neo smart contract parameter types, either
   specified explicitly or being inferred from the value. To specify the type
   manually use "type:value" syntax where the type is one of the following:
   'signature', 'bool', 'int', 'hash160', 'hash256', 'bytes', 'key' or 'string'.
   Array types are also supported: use special space-separated '[' and ']' 
   symbols around array values to denote array bounds. Nested arrays are also 
   supported.

   There is ability to provide an argument of 'bytearray' type via file. Use a 
   special 'filebytes' argument type for this with a filepath specified after
   the colon, e.g. 'filebytes:my_file.txt'.

   Given values are type-checked against given types with the following
   restrictions applied:
    * 'signature' type values should be hex-encoded and have a (decoded)
      length of 64 bytes.
    * 'bool' type values are 'true' and 'false'.
    * 'int' values are decimal integers that can be successfully converted
      from the string.
    * 'hash160' values are Neo addresses and hex-encoded 20-bytes long (after
      decoding) strings.
    * 'hash256' type values should be hex-encoded and have a (decoded)
      length of 32 bytes.
    * 'bytes' type values are any hex-encoded things.
    * 'filebytes' type values are filenames with the argument value inside.
    * 'key' type values are hex-encoded marshalled public keys.
    * 'string' type values are any valid UTF-8 strings. In the value's part of
      the string the colon looses it's special meaning as a separator between
      type and value and is taken literally.

   If no type is explicitly specified, it is inferred from the value using the
   following logic:
    - anything that can be interpreted as a decimal integer gets
      an 'int' type
    - 'true' and 'false' strings get 'bool' type
    - valid Neo addresses and 20 bytes long hex-encoded strings get 'hash160'
      type
    - valid hex-encoded public keys get 'key' type
    - 32 bytes long hex-encoded values get 'hash256' type
    - 64 bytes long hex-encoded values get 'signature' type
    - any other valid hex-encoded values get 'bytes' type
    - anything else is a 'string'

   Backslash character is used as an escape character and allows to use colon in
   an implicitly typed string. For any other characters it has no special
   meaning, to get a literal backslash in the string use the '\\' sequence.

   Examples:
    * 'int:42' is an integer with a value of 42
    * '42' is an integer with a value of 42
    * 'bad' is a string with a value of 'bad'
    * 'dead' is a byte array with a value of 'dead'
    * 'string:dead' is a string with a value of 'dead'
    * 'filebytes:my_data.txt' is bytes decoded from a content of my_data.txt
    * 'AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y' is a hash160 with a value
      of '23ba2703c53263e8d6e522dc32203339dcd8eee9'
    * '\4\2' is an integer with a value of 42
    * '\\4\2' is a string with a value of '\42'
    * 'string:string' is a string with a value of 'string'
    * 'string\:string' is a string with a value of 'string:string'
    * '03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c' is a
      key with a value of '03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c'
    * '[ a b c ]' is an array with strings values 'a', 'b' and 'c'
    * '[ a b [ c d ] e ]' is an array with 4 values: string 'a', string 'b',
      array of two strings 'c' and 'd', string 'e'
    * '[ ]' is an empty array

   Signers represent a set of Uint160 hashes with witness scopes and are used
   to verify hashes in System.Runtime.CheckWitness syscall. First signer is treated
   as a sender. To specify signers use signer[:scope] syntax where
    * 'signer' is a signer's address (as Neo address or hex-encoded 160 bit (20 byte)
               LE value with or without '0x' prefix).
    * 'scope' is a comma-separated set of cosigner's scopes, which could be:
        - 'None' - default witness scope which may be used for the sender
			       to only pay fee for the transaction.
        - 'Global' - allows this witness in all contexts. This cannot be combined
                     with other flags.
        - 'CalledByEntry' - means that this condition must hold: EntryScriptHash 
                            == CallingScriptHash. The witness/permission/signature
                            given on first invocation will automatically expire if
                            entering deeper internal invokes. This can be default
                            safe choice for native NEO/GAS.
        - 'CustomContracts' - define valid custom contract hashes for witness check.
                              Hashes are be provided as hex-encoded LE value string.
                              At lest one hash must be provided. Multiple hashes
                              are separated by ':'.
        - 'CustomGroups' - define custom public keys for group members. Public keys are
                           provided as short-form (1-byte prefix + 32 bytes) hex-encoded
                           values. At least one key must be provided. Multiple keys
                           are separated by ':'.

   If no scopes were specified, 'CalledByEntry' used as default. If no signers were
   specified, no array is passed. Note that scopes are properly handled by 
   neo-go RPC server only. C# implementation does not support scopes capability.

   Examples:
    * 'NNQk4QXsxvsrr3GSozoWBUxEmfag7B6hz5'
    * 'NVquyZHoPirw6zAEPvY1ZezxM493zMWQqs:Global'
    * '0x0000000009070e030d0f0e020d0c06050e030c02'
    * '0000000009070e030d0f0e020d0c06050e030c02:CalledByEntry,` +
					`CustomGroups:0206d7495ceb34c197093b5fc1cccf1996ada05e69ef67e765462a7f5d88ee14d0'
    * '0000000009070e030d0f0e020d0c06050e030c02:CalledByEntry,` +
					`CustomContracts:1011120009070e030d0f0e020d0c06050e030c02:0x1211100009070e030d0f0e020d0c06050e030c02'
`,
				Action: testInvokeFunction,
				Flags:  options.RPC,
			},
			{
				Name:      "testinvokescript",
				Usage:     "Invoke compiled AVM code in NEF format on the blockchain (test mode, not creating a transaction for it)",
				UsageText: "neo-go contract testinvokescript -r endpoint -i input.nef [signers...]",
				Description: `Executes given compiled AVM instructions in NEF format with the given set of
   signers not included sender by default. See testinvokefunction documentation 
   for the details about parameters.
`,
				Action: testInvokeScript,
				Flags:  testInvokeScriptFlags,
			},
			{
				Name:   "init",
				Usage:  "initialize a new smart-contract in a directory with boiler plate code",
				Action: initSmartContract,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "name, n",
						Usage: "name of the smart-contract to be initialized",
					},
					cli.BoolFlag{
						Name:  "skip-details, skip",
						Usage: "skip filling in the projects and contract details",
					},
				},
			},
			{
				Name:   "inspect",
				Usage:  "creates a user readable dump of the program instructions",
				Action: inspect,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "compile, c",
						Usage: "compile input file (it should be go code then)",
					},
					cli.StringFlag{
						Name:  "in, i",
						Usage: "input file of the program (either .go or .nef)",
					},
				},
			},
			{
				Name:   "calc-hash",
				Usage:  "calculates hash of a contract after deployment",
				Action: calcHash,
				Flags: []cli.Flag{
					flags.AddressFlag{
						Name:  "sender, s",
						Usage: "sender script hash or address",
					},
					cli.StringFlag{
						Name:  "in",
						Usage: "path to NEF file",
					},
					cli.StringFlag{
						Name:  "manifest, m",
						Usage: "path to manifest file",
					},
				},
			},
			{
				Name:  "manifest",
				Usage: "manifest-related commands",
				Subcommands: []cli.Command{
					{
						Name:   "add-group",
						Usage:  "adds group to the manifest",
						Action: manifestAddGroup,
						Flags: []cli.Flag{
							walletFlag,
							cli.StringFlag{
								Name:  "sender, s",
								Usage: "deploy transaction sender",
							},
							cli.StringFlag{
								Name:  "account, a",
								Usage: "account to sign group with",
							},
							cli.StringFlag{
								Name:  "nef, n",
								Usage: "path to the NEF file",
							},
							cli.StringFlag{
								Name:  "manifest, m",
								Usage: "path to the manifest",
							},
						},
					},
				},
			},
		},
	}}
}

// initSmartContract initializes a given directory with some boiler plate code.
func initSmartContract(ctx *cli.Context) error {
	contractName := ctx.String("name")
	if contractName == "" {
		return cli.NewExitError(errNoSmartContractName, 1)
	}

	// Check if the file already exists, if yes, exit
	if _, err := os.Stat(contractName); err == nil {
		return cli.NewExitError(errFileExist, 1)
	}

	basePath := contractName
	contractName = filepath.Base(contractName)
	fileName := "main.go"

	// create base directory
	if err := os.Mkdir(basePath, os.ModePerm); err != nil {
		return cli.NewExitError(err, 1)
	}

	m := ProjectConfig{
		Name:               contractName,
		SourceURL:          "http://example.com/",
		SupportedStandards: []string{},
		SafeMethods:        []string{},
		Events: []manifest.Event{
			{
				Name: "Hello world!",
				Parameters: []manifest.Parameter{
					{
						Name: "args",
						Type: smartcontract.ArrayType,
					},
				},
			},
		},
		Permissions: []permission{permission(*manifest.NewPermission(manifest.PermissionWildcard))},
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := ioutil.WriteFile(filepath.Join(basePath, "neo-go.yml"), b, 0644); err != nil {
		return cli.NewExitError(err, 1)
	}

	data := []byte(fmt.Sprintf(smartContractTmpl, contractName))
	if err := ioutil.WriteFile(filepath.Join(basePath, fileName), data, 0644); err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Fprintf(ctx.App.Writer, "Successfully initialized smart contract [%s]\n", contractName)

	return nil
}

func contractCompile(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	manifestFile := ctx.String("manifest")
	confFile := ctx.String("config")
	debugFile := ctx.String("debug")
	if len(confFile) == 0 && (len(manifestFile) != 0 || len(debugFile) != 0) {
		return cli.NewExitError(errNoConfFile, 1)
	}

	o := &compiler.Options{
		Outfile: ctx.String("out"),

		DebugInfo:    debugFile,
		ManifestFile: manifestFile,

		NoStandardCheck:    ctx.Bool("no-standards"),
		NoEventsCheck:      ctx.Bool("no-events"),
		NoPermissionsCheck: ctx.Bool("no-permissions"),
	}

	if len(confFile) != 0 {
		conf, err := ParseContractConfig(confFile)
		if err != nil {
			return err
		}
		o.Name = conf.Name
		o.SourceURL = conf.SourceURL
		o.ContractEvents = conf.Events
		o.ContractSupportedStandards = conf.SupportedStandards
		o.Permissions = make([]manifest.Permission, len(conf.Permissions))
		for i := range conf.Permissions {
			o.Permissions[i] = manifest.Permission(conf.Permissions[i])
		}
		o.SafeMethods = conf.SafeMethods
		o.Overloads = conf.Overloads
	}

	result, err := compiler.CompileAndSave(src, o)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if ctx.Bool("verbose") {
		fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(result))
	}

	return nil
}

func calcHash(ctx *cli.Context) error {
	sender := ctx.Generic("sender").(*flags.Address)
	if !sender.IsSet {
		return cli.NewExitError("sender is not set", 1)
	}

	p := ctx.String("in")
	if p == "" {
		return cli.NewExitError(errors.New("no .nef file was provided"), 1)
	}
	mpath := ctx.String("manifest")
	if mpath == "" {
		return cli.NewExitError(errors.New("no manifest file provided"), 1)
	}
	f, err := ioutil.ReadFile(p)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read .nef file: %w", err), 1)
	}
	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't unmarshal .nef file: %w", err), 1)
	}
	manifestBytes, err := ioutil.ReadFile(mpath)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}
	m := &manifest.Manifest{}
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to restore manifest file: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, "Contract hash:", state.CreateContractHash(sender.Uint160(), nefFile.Checksum, m.Name).StringLE())
	return nil
}

func testInvokeFunction(ctx *cli.Context) error {
	return invokeInternal(ctx, false)
}

func invokeFunction(ctx *cli.Context) error {
	return invokeInternal(ctx, true)
}

func invokeInternal(ctx *cli.Context, signAndPush bool) error {
	var (
		err             error
		exitErr         *cli.ExitError
		operation       string
		params          = make([]smartcontract.Parameter, 0)
		paramsStart     = 1
		cosigners       []transaction.Signer
		cosignersOffset = 0
	)

	args := ctx.Args()
	if !args.Present() {
		return cli.NewExitError(errNoScriptHash, 1)
	}
	script, err := flags.ParseAddress(args[0])
	if err != nil {
		return cli.NewExitError(fmt.Errorf("incorrect script hash: %w", err), 1)
	}
	if len(args) <= 1 {
		return cli.NewExitError(errNoMethod, 1)
	}
	operation = args[1]
	paramsStart++

	if len(args) > paramsStart {
		cosignersOffset, params, err = cmdargs.ParseParams(args[paramsStart:], true)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	cosignersStart := paramsStart + cosignersOffset
	cosigners, exitErr = cmdargs.GetSignersFromContext(ctx, cosignersStart)
	if exitErr != nil {
		return exitErr
	}

	var (
		acc *wallet.Account
		w   *wallet.Wallet
	)
	if signAndPush {
		acc, w, err = getAccFromContext(ctx)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	_, err = invokeWithArgs(ctx, acc, w, script, operation, params, cosigners)
	return err
}

func invokeWithArgs(ctx *cli.Context, acc *wallet.Account, wall *wallet.Wallet, script util.Uint160, operation string, params []smartcontract.Parameter, cosigners []transaction.Signer) (util.Uint160, error) {
	var (
		err               error
		gas, sysgas       fixedn.Fixed8
		cosignersAccounts []client.SignerAccount
		resp              *result.Invoke
		sender            util.Uint160
		signAndPush       = acc != nil
	)
	if signAndPush {
		gas = flags.Fixed8FromContext(ctx, "gas")
		sysgas = flags.Fixed8FromContext(ctx, "sysgas")
		sender, err = address.StringToUint160(acc.Address)
		if err != nil {
			return sender, err
		}
		cosignersAccounts, err = cmdargs.GetSignersAccounts(wall, cosigners)
		if err != nil {
			return sender, cli.NewExitError(fmt.Errorf("failed to calculate network fee: %w", err), 1)
		}
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return sender, err
	}

	resp, err = c.InvokeFunction(script, operation, params, cosigners)
	if err != nil {
		return sender, cli.NewExitError(err, 1)
	}
	if signAndPush && resp.State != "HALT" {
		errText := fmt.Sprintf("Warning: %s VM state returned from the RPC node: %s\n", resp.State, resp.FaultException)
		if !ctx.Bool("force") {
			return sender, cli.NewExitError(errText+". Use --force flag to send the transaction anyway.", 1)
		}
		fmt.Fprintln(ctx.App.Writer, errText+". Sending transaction...")
	}
	if out := ctx.String("out"); out != "" {
		tx, err := c.CreateTxFromScript(resp.Script, acc, resp.GasConsumed+int64(sysgas), int64(gas), cosignersAccounts)
		if err != nil {
			return sender, cli.NewExitError(fmt.Errorf("failed to create tx: %w", err), 1)
		}
		if err := paramcontext.InitAndSave(c.GetNetwork(), tx, acc, out); err != nil {
			return sender, cli.NewExitError(err, 1)
		}
		fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
		return sender, nil
	}
	if signAndPush {
		if len(resp.Script) == 0 {
			return sender, cli.NewExitError(errors.New("no script returned from the RPC node"), 1)
		}
		tx, err := c.CreateTxFromScript(resp.Script, acc, resp.GasConsumed+int64(sysgas), int64(gas), cosignersAccounts)
		if err != nil {
			return sender, cli.NewExitError(fmt.Errorf("failed to create tx: %w", err), 1)
		}
		if !ctx.Bool("force") {
			err := input.ConfirmTx(ctx.App.Writer, tx)
			if err != nil {
				return sender, cli.NewExitError(err, 1)
			}
		}
		txHash, err := c.SignAndPushTx(tx, acc, cosignersAccounts)
		if err != nil {
			return sender, cli.NewExitError(fmt.Errorf("failed to push invocation tx: %w", err), 1)
		}
		fmt.Fprintf(ctx.App.Writer, "Sent invocation transaction %s\n", txHash.StringLE())
	} else {
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return sender, cli.NewExitError(err, 1)
		}

		fmt.Fprintln(ctx.App.Writer, string(b))
	}

	return sender, nil
}

func testInvokeScript(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}

	b, err := ioutil.ReadFile(src)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	nefFile, err := nef.FileFromBytes(b)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to restore .nef file: %w", err), 1)
	}

	signers, exitErr := cmdargs.GetSignersFromContext(ctx, 0)
	if exitErr != nil {
		return exitErr
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	resp, err := c.InvokeScript(nefFile.Script, signers)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	b, err = json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Fprintln(ctx.App.Writer, string(b))

	return nil
}

// ProjectConfig contains project metadata.
type ProjectConfig struct {
	Name               string
	SourceURL          string
	SafeMethods        []string
	SupportedStandards []string
	Events             []manifest.Event
	Permissions        []permission
	Overloads          map[string]string `yaml:"overloads,omitempty"`
}

func inspect(ctx *cli.Context) error {
	in := ctx.String("in")
	compile := ctx.Bool("compile")
	if len(in) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	var (
		b   []byte
		err error
	)
	if compile {
		b, err = compiler.Compile(in, nil)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to compile: %w", err), 1)
		}
	} else {
		f, err := ioutil.ReadFile(in)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to read .nef file: %w", err), 1)
		}
		nefFile, err := nef.FileFromBytes(f)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to restore .nef file: %w", err), 1)
		}
		b = nefFile.Script
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps(ctx.App.Writer)

	return nil
}

func getAccFromContext(ctx *cli.Context) (*wallet.Account, *wallet.Wallet, error) {
	var addr util.Uint160

	wPath := ctx.String("wallet")
	if len(wPath) == 0 {
		return nil, nil, cli.NewExitError(errNoWallet, 1)
	}

	wall, err := wallet.NewWalletFromFile(wPath)
	if err != nil {
		return nil, nil, cli.NewExitError(err, 1)
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		addr = addrFlag.Uint160()
	} else {
		addr = wall.GetChangeAddress()
	}

	acc, err := getUnlockedAccount(wall, addr)
	return acc, wall, err
}

func getUnlockedAccount(wall *wallet.Wallet, addr util.Uint160) (*wallet.Account, error) {
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, cli.NewExitError(fmt.Errorf("wallet contains no account for '%s'", address.Uint160ToString(addr)), 1)
	}

	if acc.PrivateKey() != nil {
		return acc, nil
	}

	rawPass, err := input.ReadPassword(
		fmt.Sprintf("Enter account %s password > ", address.Uint160ToString(addr)))
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("Error reading password: %w", err), 1)
	}
	pass := strings.TrimRight(string(rawPass), "\n")
	err = acc.Decrypt(pass, wall.Scrypt)
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	return acc, nil
}

// contractDeploy deploys contract.
func contractDeploy(ctx *cli.Context) error {
	nefFile, f, err := readNEFFile(ctx.String("in"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	m, manifestBytes, err := readManifest(ctx.String("manifest"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}

	appCallParams := []smartcontract.Parameter{
		{
			Type:  smartcontract.ByteArrayType,
			Value: f,
		},
		{
			Type:  smartcontract.ByteArrayType,
			Value: manifestBytes,
		},
	}
	signOffset, data, err := cmdargs.ParseParams(ctx.Args(), true)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("unable to parse 'data' parameter: %w", err), 1)
	}
	if len(data) > 1 {
		return cli.NewExitError("'data' should be represented as a single parameter", 1)
	}
	if len(data) != 0 {
		appCallParams = append(appCallParams, data[0])
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	mgmtHash, err := c.GetNativeContractHash(nativenames.Management)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get management contract's hash: %w", err), 1)
	}

	acc, w, err := getAccFromContext(ctx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't get sender address: %w", err), 1)
	}

	cosigners, sgnErr := cmdargs.GetSignersFromContext(ctx, signOffset)
	if sgnErr != nil {
		return err
	} else if len(cosigners) == 0 {
		cosigners = []transaction.Signer{{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}}
	}

	sender, extErr := invokeWithArgs(ctx, acc, w, mgmtHash, "deploy", appCallParams, cosigners)
	if extErr != nil {
		return extErr
	}

	hash := state.CreateContractHash(sender, nefFile.Checksum, m.Name)
	fmt.Fprintf(ctx.App.Writer, "Contract: %s\n", hash.StringLE())
	return nil
}

// ParseContractConfig reads contract configuration file (.yaml) and returns unmarshalled ProjectConfig.
func ParseContractConfig(confFile string) (ProjectConfig, error) {
	conf := ProjectConfig{}
	confBytes, err := ioutil.ReadFile(confFile)
	if err != nil {
		return conf, cli.NewExitError(err, 1)
	}

	err = yaml.Unmarshal(confBytes, &conf)
	if err != nil {
		return conf, cli.NewExitError(fmt.Errorf("bad config: %w", err), 1)
	}
	return conf, nil
}
