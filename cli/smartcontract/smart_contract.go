package smartcontract

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	cliwallet "github.com/nspcc-dev/neo-go/cli/wallet"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/management"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/binding"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

// addressFlagName is a flag name used for address-related operations. It should be
// the same within the smartcontract package, thus, use this constant.
const addressFlagName = "address, a"

var (
	errNoInput                = errors.New("no input file was found, specify an input file with the '--in or -i' flag")
	errNoConfFile             = errors.New("no config file was found, specify a config file with the '--config' or '-c' flag")
	errNoManifestFile         = errors.New("no manifest file was found, specify manifest file with '--manifest' or '-m' flag")
	errNoMethod               = errors.New("no method specified for function invocation command")
	errNoWallet               = errors.New("no wallet parameter found, specify it with the '--wallet' or '-w' flag or specify wallet config file with the '--wallet-config' flag")
	errConflictingWalletFlags = errors.New("--wallet flag conflicts with --wallet-config flag, please, provide one of them to specify wallet location")
	errNoScriptHash           = errors.New("no smart contract hash was provided, specify one as the first argument")
	errNoSmartContractName    = errors.New("no name was provided, specify the '--name or -n' flag")
	errFileExist              = errors.New("A file with given smart-contract name already exists")

	walletFlag = cli.StringFlag{
		Name:  "wallet, w",
		Usage: "wallet to use to get the key for transaction signing; conflicts with --wallet-config flag",
	}
	walletConfigFlag = cli.StringFlag{
		Name:  "wallet-config",
		Usage: "path to wallet config to use to get the key for transaction signing; conflicts with --wallet flag",
	}
	addressFlag = flags.AddressFlag{
		Name:  addressFlagName,
		Usage: "address to use as transaction signee (and gas source)",
	}
)

// ModVersion contains `pkg/interop` module version
// suitable to be used in go.mod.
var ModVersion string

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
func RuntimeNotify(args []any) {
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
		options.Historic,
	}
	testInvokeScriptFlags = append(testInvokeScriptFlags, options.RPC...)
	testInvokeFunctionFlags := []cli.Flag{options.Historic}
	testInvokeFunctionFlags = append(testInvokeFunctionFlags, options.RPC...)
	invokeFunctionFlags := []cli.Flag{
		walletFlag,
		walletConfigFlag,
		addressFlag,
		txctx.GasFlag,
		txctx.SysGasFlag,
		txctx.OutFlag,
		txctx.ForceFlag,
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
				Name:      "compile",
				Usage:     "compile a smart contract to a .nef file",
				UsageText: "neo-go contract compile -i path [-o nef] [-v] [-d] [-m manifest] [-c yaml] [--bindings file] [--no-standards] [--no-events] [--no-permissions] [--guess-eventtypes]",
				Description: `Compiles given smart contract to a .nef file and emits other associated
   information (manifest, bindings configuration, debug information files) if
   asked to. If none of --out, --manifest, --config, --bindings flags are specified,
   then the output filenames for these flags will be guessed using the contract
   name or path provided via --in option by trimming/adding corresponding suffixes
   to the common part of the path. In the latter case the configuration filepath
   will be guessed from the --in option using the same rule."`,
				Action: contractCompile,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input file for the smart contract to be compiled (*.go file or directory)",
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
					cli.BoolFlag{
						Name:  "guess-eventtypes",
						Usage: "guess event types for smart-contract bindings configuration from the code usages",
					},
					cli.StringFlag{
						Name:  "bindings",
						Usage: "output file for smart-contract bindings configuration",
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
			generateWrapperCmd,
			generateRPCWrapperCmd,
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
				UsageText: "neo-go contract testinvokefunction -r endpoint [--historic index/hash] scripthash [method] [arguments...] [--] [signers...]",
				Description: `Executes given (as a script hash) deployed script with the given method,
   arguments and signers (sender is not included by default). If no method is given
   "" is passed to the script, if no arguments are given, an empty array is 
   passed, if no signers are given no array is passed. If signers are specified,
   the first one of them is treated as a sender. All of the given arguments are 
   encapsulated into array before invoking the script. The script thus should 
   follow the regular convention of smart contract arguments (method string and 
   an array of other arguments).

` + cmdargs.ParamsParsingDoc + `

` + cmdargs.SignersParsingDoc + `
`,
				Action: testInvokeFunction,
				Flags:  testInvokeFunctionFlags,
			},
			{
				Name:      "testinvokescript",
				Usage:     "Invoke compiled AVM code in NEF format on the blockchain (test mode, not creating a transaction for it)",
				UsageText: "neo-go contract testinvokescript -r endpoint -i input.nef [--historic index/hash] [signers...]",
				Description: `Executes given compiled AVM instructions in NEF format with the given set of
   signers not included sender by default. See testinvokefunction documentation 
   for the details about parameters.
`,
				Action: testInvokeScript,
				Flags:  testInvokeScriptFlags,
			},
			{
				Name:      "init",
				Usage:     "initialize a new smart-contract in a directory with boiler plate code",
				UsageText: "neo-go contract init -n name [--skip-details]",
				Action:    initSmartContract,
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
				Name:      "inspect",
				Usage:     "creates a user readable dump of the program instructions",
				UsageText: "neo-go contract inspect -i file [-c]",
				Action:    inspect,
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
				Name:      "calc-hash",
				Usage:     "calculates hash of a contract after deployment",
				UsageText: "neo-go contract calc-hash -i nef -m manifest -s address",
				Action:    calcHash,
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
						Name:      "add-group",
						Usage:     "adds group to the manifest",
						UsageText: "neo-go contract manifest add-group -w wallet [--wallet-config path] -n nef -m manifest -a address -s address",
						Action:    manifestAddGroup,
						Flags: []cli.Flag{
							walletFlag,
							walletConfigFlag,
							cli.StringFlag{
								Name:  "sender, s",
								Usage: "deploy transaction sender",
							},
							flags.AddressFlag{
								Name:  addressFlagName, // use the same name for handler code unification.
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
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
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
		Events: []compiler.HybridEvent{
			{
				Name: "Hello world!",
				Parameters: []compiler.HybridParameter{
					{
						Parameter: manifest.Parameter{
							Name: "args",
							Type: smartcontract.ArrayType,
						},
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
	if err := os.WriteFile(filepath.Join(basePath, "neo-go.yml"), b, 0644); err != nil {
		return cli.NewExitError(err, 1)
	}

	ver := ModVersion
	if ver == "" {
		ver = "latest"
	}

	gm := []byte("module " + contractName + `
require (
	github.com/nspcc-dev/neo-go/pkg/interop ` + ver + `
)`)
	if err := os.WriteFile(filepath.Join(basePath, "go.mod"), gm, 0644); err != nil {
		return cli.NewExitError(err, 1)
	}

	data := []byte(fmt.Sprintf(smartContractTmpl, contractName))
	if err := os.WriteFile(filepath.Join(basePath, fileName), data, 0644); err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Fprintf(ctx.App.Writer, "Successfully initialized smart contract [%s]\n", contractName)

	return nil
}

func contractCompile(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	manifestFile := ctx.String("manifest")
	confFile := ctx.String("config")
	debugFile := ctx.String("debug")
	out := ctx.String("out")
	bindings := ctx.String("bindings")
	if len(confFile) == 0 && (len(manifestFile) != 0 || len(debugFile) != 0 || len(bindings) != 0) {
		return cli.NewExitError(errNoConfFile, 1)
	}
	autocomplete := len(manifestFile) == 0 &&
		len(confFile) == 0 &&
		len(out) == 0 &&
		len(bindings) == 0
	if autocomplete {
		var root string
		fileInfo, err := os.Stat(src)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to stat source file or directory: %w", err), 1)
		}
		if fileInfo.IsDir() {
			base := filepath.Base(fileInfo.Name())
			if base == string(filepath.Separator) {
				base = "contract"
			}
			root = filepath.Join(src, base)
		} else {
			root = strings.TrimSuffix(src, ".go")
		}
		manifestFile = root + ".manifest.json"
		confFile = root + ".yml"
		out = root + ".nef"
		bindings = root + ".bindings.yml"
	}

	o := &compiler.Options{
		Outfile: out,

		DebugInfo:    debugFile,
		ManifestFile: manifestFile,
		BindingsFile: bindings,

		NoStandardCheck:    ctx.Bool("no-standards"),
		NoEventsCheck:      ctx.Bool("no-events"),
		NoPermissionsCheck: ctx.Bool("no-permissions"),

		GuessEventTypes: ctx.Bool("guess-eventtypes"),
	}

	if len(confFile) != 0 {
		conf, err := ParseContractConfig(confFile)
		if err != nil {
			return err
		}
		o.Name = conf.Name
		o.SourceURL = conf.SourceURL
		o.ContractEvents = conf.Events
		o.DeclaredNamedTypes = conf.NamedTypes
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
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
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
	f, err := os.ReadFile(p)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't read .nef file: %w", err), 1)
	}
	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't unmarshal .nef file: %w", err), 1)
	}
	manifestBytes, err := os.ReadFile(mpath)
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
		params          []any
		paramsStart     = 1
		scParams        []smartcontract.Parameter
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
		cosignersOffset, scParams, err = cmdargs.ParseParams(args[paramsStart:], true)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		params = make([]any, len(scParams))
		for i := range scParams {
			params[i] = scParams[i]
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
		defer w.Close()
	}

	return invokeWithArgs(ctx, acc, w, script, operation, params, cosigners)
}

func invokeWithArgs(ctx *cli.Context, acc *wallet.Account, wall *wallet.Wallet, script util.Uint160, operation string, params []any, cosigners []transaction.Signer) error {
	var (
		err             error
		signersAccounts []actor.SignerAccount
		resp            *result.Invoke
		signAndPush     = acc != nil
		inv             *invoker.Invoker
		act             *actor.Actor
	)
	if signAndPush {
		signersAccounts, err = cmdargs.GetSignersAccounts(acc, wall, cosigners, transaction.None)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("invalid signers: %w", err), 1)
		}
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}
	if signAndPush {
		act, err = actor.New(c, signersAccounts)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to create RPC actor: %w", err), 1)
		}
		inv = &act.Invoker
	} else {
		inv, err = options.GetInvoker(c, ctx, cosigners)
		if err != nil {
			return err
		}
	}
	out := ctx.String("out")
	resp, err = inv.Call(script, operation, params...)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if resp.State != "HALT" {
		errText := fmt.Sprintf("Warning: %s VM state returned from the RPC node: %s", resp.State, resp.FaultException)
		if !signAndPush {
			return cli.NewExitError(errText, 1)
		}

		action := "send"
		process := "Sending"
		if out != "" {
			action = "save"
			process = "Saving"
		}
		if !ctx.Bool("force") {
			return cli.NewExitError(errText+".\nUse --force flag to "+action+" the transaction anyway.", 1)
		}
		fmt.Fprintln(ctx.App.Writer, errText+".\n"+process+" transaction...")
	}
	if !signAndPush {
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		fmt.Fprintln(ctx.App.Writer, string(b))
		return nil
	}
	if len(resp.Script) == 0 {
		return cli.NewExitError(errors.New("no script returned from the RPC node"), 1)
	}
	tx, err := act.MakeUnsignedUncheckedRun(resp.Script, resp.GasConsumed, nil)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create tx: %w", err), 1)
	}
	return txctx.SignAndSend(ctx, act, acc, tx)
}

func testInvokeScript(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}

	b, err := os.ReadFile(src)
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

	_, inv, err := options.GetRPCWithInvoker(gctx, ctx, signers)
	if err != nil {
		return err
	}

	resp, err := inv.Run(nefFile.Script)
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
	Events             []compiler.HybridEvent
	Permissions        []permission
	Overloads          map[string]string               `yaml:"overloads,omitempty"`
	NamedTypes         map[string]binding.ExtendedType `yaml:"namedtypes,omitempty"`
}

func inspect(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
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
		f, err := os.ReadFile(in)
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
	walletConfigPath := ctx.String("wallet-config")
	if len(wPath) != 0 && len(walletConfigPath) != 0 {
		return nil, nil, errConflictingWalletFlags
	}
	if len(wPath) == 0 && len(walletConfigPath) == 0 {
		return nil, nil, errNoWallet
	}
	var pass *string
	if len(walletConfigPath) != 0 {
		cfg, err := cliwallet.ReadWalletConfig(walletConfigPath)
		if err != nil {
			return nil, nil, err
		}
		wPath = cfg.Path
		pass = &cfg.Password
	}

	wall, err := wallet.NewWalletFromFile(wPath)
	if err != nil {
		return nil, nil, err
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		addr = addrFlag.Uint160()
	} else {
		addr = wall.GetChangeAddress()
	}

	acc, err := getUnlockedAccount(wall, addr, pass)
	return acc, wall, err
}

func getUnlockedAccount(wall *wallet.Wallet, addr util.Uint160, pass *string) (*wallet.Account, error) {
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, fmt.Errorf("wallet contains no account for '%s'", address.Uint160ToString(addr))
	}

	if acc.CanSign() {
		return acc, nil
	}

	if pass == nil {
		rawPass, err := input.ReadPassword(
			fmt.Sprintf("Enter account %s password > ", address.Uint160ToString(addr)))
		if err != nil {
			return nil, fmt.Errorf("Error reading password: %w", err)
		}
		trimmed := strings.TrimRight(string(rawPass), "\n")
		pass = &trimmed
	}
	err := acc.Decrypt(*pass, wall.Scrypt)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// contractDeploy deploys contract.
func contractDeploy(ctx *cli.Context) error {
	nefFile, f, err := readNEFFile(ctx.String("in"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	m, manifestBytes, err := readManifest(ctx.String("manifest"), util.Uint160{})
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}

	var appCallParams = []any{f, manifestBytes}

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

	acc, w, err := getAccFromContext(ctx)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't get sender address: %w", err), 1)
	}
	defer w.Close()
	sender := acc.ScriptHash()

	cosigners, sgnErr := cmdargs.GetSignersFromContext(ctx, signOffset)
	if sgnErr != nil {
		return err
	} else if len(cosigners) == 0 {
		cosigners = []transaction.Signer{{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}}
	}

	extErr := invokeWithArgs(ctx, acc, w, management.Hash, "deploy", appCallParams, cosigners)
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
	confBytes, err := os.ReadFile(confFile)
	if err != nil {
		return conf, cli.NewExitError(err, 1)
	}

	err = yaml.Unmarshal(confBytes, &conf)
	if err != nil {
		return conf, cli.NewExitError(fmt.Errorf("bad config: %w", err), 1)
	}
	return conf, nil
}
