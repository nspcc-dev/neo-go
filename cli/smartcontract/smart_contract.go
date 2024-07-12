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
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// addressFlagName and addressFlagAlias are a flag name and its alias
// used for address-related operations. It should be the same within
// the smartcontract package, thus, use this constant.
const (
	addressFlagName  = "address"
	addressFlagAlias = "a"
)

var (
	errNoConfFile   = errors.New("no config file was found, specify a config file with the '--config' or '-c' flag")
	errNoMethod     = errors.New("no method specified for function invocation command")
	errNoScriptHash = errors.New("no smart contract hash was provided, specify one as the first argument")
	errFileExist    = errors.New("A file with given smart-contract name already exists")
	addressFlag     = &flags.AddressFlag{
		Name:    addressFlagName,
		Aliases: []string{addressFlagAlias},
		Usage:   "Address to use as transaction signee (and gas source)",
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
func NewCommands() []*cli.Command {
	testInvokeScriptFlags := []cli.Flag{
		&cli.StringFlag{
			Name:     "in",
			Aliases:  []string{"i"},
			Required: true,
			Usage:    "Input location of the .nef file that needs to be invoked",
			Action:   cmdargs.EnsureNotEmpty("in"),
		},
		options.Historic,
	}
	testInvokeScriptFlags = append(testInvokeScriptFlags, options.RPC...)
	testInvokeFunctionFlags := []cli.Flag{options.Historic}
	testInvokeFunctionFlags = append(testInvokeFunctionFlags, options.RPC...)
	invokeFunctionFlags := []cli.Flag{
		addressFlag,
		txctx.GasFlag,
		txctx.SysGasFlag,
		txctx.OutFlag,
		txctx.ForceFlag,
		txctx.AwaitFlag,
	}
	invokeFunctionFlags = append(invokeFunctionFlags, options.Wallet...)
	invokeFunctionFlags = append(invokeFunctionFlags, options.RPC...)
	deployFlags := append(invokeFunctionFlags, []cli.Flag{
		&cli.StringFlag{
			Name:     "in",
			Aliases:  []string{"i"},
			Required: true,
			Usage:    "Input file for the smart contract (*.nef)",
			Action:   cmdargs.EnsureNotEmpty("in"),
		},
		&cli.StringFlag{
			Name:     "manifest",
			Aliases:  []string{"m"},
			Required: true,
			Usage:    "Manifest input file (*.manifest.json)",
			Action:   cmdargs.EnsureNotEmpty("manifest"),
		},
	}...)
	manifestAddGroupFlags := append([]cli.Flag{
		&flags.AddressFlag{
			Name:     "sender",
			Aliases:  []string{"s"},
			Required: true,
			Usage:    "Deploy transaction sender",
		},
		&flags.AddressFlag{
			Name:     addressFlagName, // use the same name for handler code unification.
			Aliases:  []string{addressFlagAlias},
			Required: true,
			Usage:    "Account to sign group with",
		},
		&cli.StringFlag{
			Name:     "nef",
			Aliases:  []string{"n"},
			Required: true,
			Usage:    "Path to the NEF file",
			Action:   cmdargs.EnsureNotEmpty("nef"),
		},
		&cli.StringFlag{
			Name:     "manifest",
			Aliases:  []string{"m"},
			Required: true,
			Usage:    "Path to the manifest",
			Action:   cmdargs.EnsureNotEmpty("manifest"),
		},
	}, options.Wallet...)
	return []*cli.Command{{
		Name:  "contract",
		Usage: "Compile - debug - deploy smart contracts",
		Subcommands: []*cli.Command{
			{
				Name:      "compile",
				Usage:     "Compile a smart contract to a .nef file",
				UsageText: "neo-go contract compile -i path [-o nef] [-v] [-d] [-m manifest] [-c yaml] [--bindings file] [--no-standards] [--no-events] [--no-permissions] [--guess-eventtypes]",
				Description: `Compiles given smart contract to a .nef file and emits other associated
   information (manifest, bindings configuration, debug information files) if
   asked to. If none of --out, --manifest, --config, --bindings flags are specified,
   then the output filenames for these flags will be guessed using the contract
   name or path provided via --in option by trimming/adding corresponding suffixes
   to the common part of the path. In the latter case the configuration filepath
   will be guessed from the --in option using the same rule.
`,
				Action: contractCompile,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "in",
						Aliases:  []string{"i"},
						Required: true,
						Usage:    "Input file for the smart contract to be compiled (*.go file or directory)",
						Action:   cmdargs.EnsureNotEmpty("in"),
					},
					&cli.StringFlag{
						Name:    "out",
						Aliases: []string{"o"},
						Usage:   "Output of the compiled contract",
					},
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Print out additional information after a compiling",
					},
					&cli.StringFlag{
						Name:    "debug",
						Aliases: []string{"d"},
						Usage:   "Emit debug info in a separate file",
					},
					&cli.StringFlag{
						Name:    "manifest",
						Aliases: []string{"m"},
						Usage:   "Emit contract manifest (*.manifest.json) file into separate file using configuration input file (*.yml)",
					},
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "Configuration input file (*.yml)",
					},
					&cli.BoolFlag{
						Name:  "no-standards",
						Usage: "Do not check compliance with supported standards",
					},
					&cli.BoolFlag{
						Name:  "no-events",
						Usage: "Do not check emitted events with the manifest",
					},
					&cli.BoolFlag{
						Name:  "no-permissions",
						Usage: "Do not check if invoked contracts are allowed in manifest",
					},
					&cli.BoolFlag{
						Name:  "guess-eventtypes",
						Usage: "Guess event types for smart-contract bindings configuration from the code usages",
					},
					&cli.StringFlag{
						Name:  "bindings",
						Usage: "Output file for smart-contract bindings configuration",
					},
				},
			},
			{
				Name:      "deploy",
				Usage:     "Deploy a smart contract (.nef with description)",
				UsageText: "neo-go contract deploy -r endpoint -w wallet [-a address] [-g gas] [-e sysgas] --in contract.nef --manifest contract.manifest.json [--out file] [--force] [--await] [data]",
				Description: `Deploys given contract into the chain. The gas parameter is for additional
   gas to be added as a network fee to prioritize the transaction. The data 
   parameter is an optional parameter to be passed to '_deploy' method. When
   --await flag is specified, it waits for the transaction to be included 
   in a block.
`,
				Action: contractDeploy,
				Flags:  deployFlags,
			},
			generateWrapperCmd,
			generateRPCWrapperCmd,
			{
				Name:      "invokefunction",
				Usage:     "Invoke deployed contract on the blockchain",
				UsageText: "neo-go contract invokefunction -r endpoint -w wallet [-a address] [-g gas] [-e sysgas] [--out file] [--force] [--await] scripthash [method] [arguments...] [--] [signers...]",
				Description: `Executes given (as a script hash) deployed script with the given method,
   arguments and signers. Sender is included in the list of signers by default
   with None witness scope. If you'd like to change default sender's scope, 
   specify it via signers parameter. See testinvokefunction documentation for 
   the details about parameters. It differs from testinvokefunction in that this
   command sends an invocation transaction to the network. When --await flag is
   specified, it waits for the transaction to be included in a block.
`,
				Action: invokeFunction,
				Flags:  invokeFunctionFlags,
			},
			{
				Name:      "testinvokefunction",
				Usage:     "Invoke deployed contract on the blockchain (test mode)",
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
				Usage:     "Initialize a new smart-contract in a directory with boiler plate code",
				UsageText: "neo-go contract init -n name [--skip-details]",
				Action:    initSmartContract,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Name of the smart-contract to be initialized",
						Action:   cmdargs.EnsureNotEmpty("name"),
					},
					&cli.BoolFlag{
						Name:    "skip-details",
						Aliases: []string{"skip"},
						Usage:   "Skip filling in the projects and contract details",
					},
				},
			},
			{
				Name:      "inspect",
				Usage:     "Creates a user readable dump of the program instructions",
				UsageText: "neo-go contract inspect -i file [-c]",
				Action:    inspect,
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "compile",
						Aliases: []string{"c"},
						Usage:   "Compile input file (it should be go code then)",
					},
					&cli.StringFlag{
						Name:     "in",
						Aliases:  []string{"i"},
						Required: true,
						Usage:    "Input file of the program (either .go or .nef)",
						Action:   cmdargs.EnsureNotEmpty("in"),
					},
				},
			},
			{
				Name:      "calc-hash",
				Usage:     "Calculates hash of a contract after deployment",
				UsageText: "neo-go contract calc-hash -i nef -m manifest -s address",
				Action:    calcHash,
				Flags: []cli.Flag{
					&flags.AddressFlag{
						Name:     "sender",
						Aliases:  []string{"s"},
						Required: true,
						Usage:    "Sender script hash or address",
					},
					&cli.StringFlag{
						Name:     "in",
						Required: true,
						Usage:    "Path to NEF file",
						Action:   cmdargs.EnsureNotEmpty("in"),
					},
					&cli.StringFlag{
						Name:     "manifest",
						Aliases:  []string{"m"},
						Required: true,
						Usage:    "Path to manifest file",
						Action:   cmdargs.EnsureNotEmpty("manifest"),
					},
				},
			},
			{
				Name:  "manifest",
				Usage: "Manifest-related commands",
				Subcommands: []*cli.Command{
					{
						Name:      "add-group",
						Usage:     "Adds group to the manifest",
						UsageText: "neo-go contract manifest add-group -w wallet [--wallet-config path] -n nef -m manifest -a address -s address",
						Action:    manifestAddGroup,
						Flags:     manifestAddGroupFlags,
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

	// Check if the file already exists, if yes, exit
	if _, err := os.Stat(contractName); err == nil {
		return cli.Exit(errFileExist, 1)
	}

	basePath := contractName
	contractName = filepath.Base(contractName)
	fileName := "main.go"

	// create base directory
	if err := os.Mkdir(basePath, os.ModePerm); err != nil {
		return cli.Exit(err, 1)
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
		return cli.Exit(err, 1)
	}
	if err := os.WriteFile(filepath.Join(basePath, "neo-go.yml"), b, 0644); err != nil {
		return cli.Exit(err, 1)
	}

	ver := ModVersion
	if ver == "" {
		ver = "latest"
	}

	gm := []byte("module " + contractName + `

go 1.20

require (
	github.com/nspcc-dev/neo-go/pkg/interop ` + ver + `
)`)
	if err := os.WriteFile(filepath.Join(basePath, "go.mod"), gm, 0644); err != nil {
		return cli.Exit(err, 1)
	}

	data := []byte(fmt.Sprintf(smartContractTmpl, contractName))
	if err := os.WriteFile(filepath.Join(basePath, fileName), data, 0644); err != nil {
		return cli.Exit(err, 1)
	}

	fmt.Fprintf(ctx.App.Writer, "Successfully initialized smart contract [%s]\n", contractName)

	return nil
}

func contractCompile(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	src := ctx.String("in")
	manifestFile := ctx.String("manifest")
	confFile := ctx.String("config")
	debugFile := ctx.String("debug")
	out := ctx.String("out")
	bindings := ctx.String("bindings")
	if len(confFile) == 0 && (len(manifestFile) != 0 || len(debugFile) != 0 || len(bindings) != 0) {
		return cli.Exit(errNoConfFile, 1)
	}
	autocomplete := len(manifestFile) == 0 &&
		len(confFile) == 0 &&
		len(out) == 0 &&
		len(bindings) == 0
	if autocomplete {
		var root string
		fileInfo, err := os.Stat(src)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to stat source file or directory: %w", err), 1)
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
		return cli.Exit(err, 1)
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
	p := ctx.String("in")
	mpath := ctx.String("manifest")

	f, err := os.ReadFile(p)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't read .nef file: %w", err), 1)
	}
	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't unmarshal .nef file: %w", err), 1)
	}
	manifestBytes, err := os.ReadFile(mpath)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}
	m := &manifest.Manifest{}
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to restore manifest file: %w", err), 1)
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
		exitErr         cli.ExitCoder
		operation       string
		params          []any
		paramsStart     = 1
		scParams        []smartcontract.Parameter
		cosigners       []transaction.Signer
		cosignersOffset = 0
	)

	args := ctx.Args()
	if !args.Present() {
		return cli.Exit(errNoScriptHash, 1)
	}
	argsSlice := args.Slice()
	script, err := flags.ParseAddress(argsSlice[0])
	if err != nil {
		return cli.Exit(fmt.Errorf("incorrect script hash: %w", err), 1)
	}
	if len(argsSlice) <= 1 {
		return cli.Exit(errNoMethod, 1)
	}
	operation = argsSlice[1]
	paramsStart++

	if len(argsSlice) > paramsStart {
		cosignersOffset, scParams, err = cmdargs.ParseParams(argsSlice[paramsStart:], true)
		if err != nil {
			return cli.Exit(err, 1)
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
		acc, w, err = options.GetAccFromContext(ctx)
		if err != nil {
			return cli.Exit(err, 1)
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
			return cli.Exit(fmt.Errorf("invalid signers: %w", err), 1)
		}
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	if signAndPush {
		_, act, err = options.GetRPCWithActor(gctx, ctx, signersAccounts)
		if err != nil {
			return err
		}
		inv = &act.Invoker
	} else {
		_, inv, err = options.GetRPCWithInvoker(gctx, ctx, cosigners)
		if err != nil {
			return err
		}
	}
	out := ctx.String("out")
	resp, err = inv.Call(script, operation, params...)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if resp.State != "HALT" {
		errText := fmt.Sprintf("Warning: %s VM state returned from the RPC node: %s", resp.State, resp.FaultException)
		if !signAndPush {
			return cli.Exit(errText, 1)
		}

		action := "send"
		process := "Sending"
		if out != "" {
			action = "save"
			process = "Saving"
		}
		if !ctx.Bool("force") {
			return cli.Exit(errText+".\nUse --force flag to "+action+" the transaction anyway.", 1)
		}
		fmt.Fprintln(ctx.App.Writer, errText+".\n"+process+" transaction...")
	}
	if !signAndPush {
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return cli.Exit(err, 1)
		}

		fmt.Fprintln(ctx.App.Writer, string(b))
		return nil
	}
	if len(resp.Script) == 0 {
		return cli.Exit(errors.New("no script returned from the RPC node"), 1)
	}
	tx, err := act.MakeUnsignedUncheckedRun(resp.Script, resp.GasConsumed, nil)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create tx: %w", err), 1)
	}
	return txctx.SignAndSend(ctx, act, acc, tx)
}

func testInvokeScript(ctx *cli.Context) error {
	src := ctx.String("in")
	b, err := os.ReadFile(src)
	if err != nil {
		return cli.Exit(err, 1)
	}
	nefFile, err := nef.FileFromBytes(b)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to restore .nef file: %w", err), 1)
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
		return cli.Exit(err, 1)
	}

	b, err = json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return cli.Exit(err, 1)
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
	var (
		b   []byte
		err error
	)
	if compile {
		b, err = compiler.Compile(in, nil)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to compile: %w", err), 1)
		}
	} else {
		f, err := os.ReadFile(in)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to read .nef file: %w", err), 1)
		}
		nefFile, err := nef.FileFromBytes(f)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to restore .nef file: %w", err), 1)
		}
		b = nefFile.Script
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps(ctx.App.Writer)

	return nil
}

// contractDeploy deploys contract.
func contractDeploy(ctx *cli.Context) error {
	nefFile, f, err := readNEFFile(ctx.String("in"))
	if err != nil {
		return cli.Exit(err, 1)
	}

	m, manifestBytes, err := readManifest(ctx.String("manifest"), util.Uint160{})
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}

	var appCallParams = []any{f, manifestBytes}

	signOffset, data, err := cmdargs.ParseParams(ctx.Args().Slice(), true)
	if err != nil {
		return cli.Exit(fmt.Errorf("unable to parse 'data' parameter: %w", err), 1)
	}
	if len(data) > 1 {
		return cli.Exit("'data' should be represented as a single parameter", 1)
	}
	if len(data) != 0 {
		appCallParams = append(appCallParams, data[0])
	}

	acc, w, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't get sender address: %w", err), 1)
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
		return conf, cli.Exit(err, 1)
	}

	err = yaml.Unmarshal(confBytes, &conf)
	if err != nil {
		return conf, cli.Exit(fmt.Errorf("bad config: %w", err), 1)
	}
	return conf, nil
}
