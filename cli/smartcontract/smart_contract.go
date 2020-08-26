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
	"syscall"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
)

var (
	errNoInput             = errors.New("no input file was found, specify an input file with the '--in or -i' flag")
	errNoConfFile          = errors.New("no config file was found, specify a config file with the '--config' or '-c' flag")
	errNoManifestFile      = errors.New("no manifest file was found, specify a manifest file with the '--manifest' flag")
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
		Usage: "gas to add to the transaction",
	}
)

const (
	// smartContractTmpl is written to a file when used with `init` command.
	// %s is parsed to be the smartContractName
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
	// cosignersSeparator is a special value which is used to distinguish
	// parameters and cosigners for invoke* commands
	cosignersSeparator  = "--"
	arrayStartSeparator = "["
	arrayEndSeparator   = "]"
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
	deployFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "in, i",
			Usage: "Input file for the smart contract (*.nef)",
		},
		cli.StringFlag{
			Name:  "manifest",
			Usage: "Manifest input file (*.manifest.json)",
		},
		walletFlag,
		addressFlag,
		gasFlag,
	}
	deployFlags = append(deployFlags, options.RPC...)
	invokeFunctionFlags := []cli.Flag{
		walletFlag,
		addressFlag,
		gasFlag,
	}
	invokeFunctionFlags = append(invokeFunctionFlags, options.RPC...)
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
				},
			},
			{
				Name:  "deploy",
				Usage: "deploy a smart contract (.nef with description)",
				Description: `Deploys given contract into the chain. The gas parameter is for additional
   gas to be added as a network fee to prioritize the transaction. It may also
   be required to add that to satisfy chain's policy regarding transaction size
   and the minimum size fee (so if transaction send fails, try adding 0.001 GAS
   to it).
`,
				Action: contractDeploy,
				Flags:  deployFlags,
			},
			{
				Name:      "invokefunction",
				Usage:     "invoke deployed contract on the blockchain",
				UsageText: "neo-go contract invokefunction -r endpoint -w wallet [-a address] [-g gas] scripthash [method] [arguments...] [--] [signers...]",
				Description: `Executes given (as a script hash) deployed script with the given method,
   arguments and signers. Sender is included in the list of signers by default
   with FeeOnly witness scope. If you'd like to change default sender's scope, 
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
    * 'signer' is hex-encoded 160 bit (20 byte) LE value of signer's address,
                 which could have '0x' prefix.
    * 'scope' is a comma-separated set of cosigner's scopes, which could be:
        - 'FeeOnly' - marks transaction's sender and can be used only for the
                      sender. Signer with this scope can't be used during the
                      script execution and only pays fees for the transaction.
        - 'Global' - allows this witness in all contexts. This cannot be combined
                     with other flags.
        - 'CalledByEntry' - means that this condition must hold: EntryScriptHash 
                            == CallingScriptHash. The witness/permission/signature
                            given on first invocation will automatically expire if
                            entering deeper internal invokes. This can be default
                            safe choice for native NEO/GAS.
        - 'CustomContracts' - define valid custom contract hashes for witness check.
        - 'CustomGroups' - define custom pubkey for group members.

   If no scopes were specified, 'Global' used as default. If no signers were
   specified, no array is passed. Note that scopes are properly handled by 
   neo-go RPC server only. C# implementation does not support scopes capability.

   Examples:
    * '0000000009070e030d0f0e020d0c06050e030c02'
    * '0x0000000009070e030d0f0e020d0c06050e030c02'
    * '0x0000000009070e030d0f0e020d0c06050e030c02:Global'
    * '0000000009070e030d0f0e020d0c06050e030c02:CalledByEntry,CustomGroups'   
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
	fileName := "main.go"

	// create base directory
	if err := os.Mkdir(basePath, os.ModePerm); err != nil {
		return cli.NewExitError(err, 1)
	}

	m := ProjectConfig{
		SupportedStandards: []string{},
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

	fmt.Printf("Successfully initialized smart contract [%s]\n", contractName)

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
	}

	if len(confFile) != 0 {
		conf, err := parseContractConfig(confFile)
		if err != nil {
			return err
		}
		o.ContractFeatures = conf.GetFeatures()
		o.ContractEvents = conf.Events
		o.ContractSupportedStandards = conf.SupportedStandards
	}

	result, err := compiler.CompileAndSave(src, o)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if ctx.Bool("verbose") {
		fmt.Println(hex.EncodeToString(result))
	}

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
		gas             util.Fixed8
		operation       string
		params          = make([]smartcontract.Parameter, 0)
		paramsStart     = 1
		cosigners       []transaction.Signer
		cosignersOffset = 0
		resp            *result.Invoke
		acc             *wallet.Account
	)

	args := ctx.Args()
	if !args.Present() {
		return cli.NewExitError(errNoScriptHash, 1)
	}
	script, err := util.Uint160DecodeStringLE(args[0])
	if err != nil {
		return cli.NewExitError(fmt.Errorf("incorrect script hash: %w", err), 1)
	}
	if len(args) <= 1 {
		return cli.NewExitError(errNoMethod, 1)
	}
	operation = args[1]
	paramsStart++

	if len(args) > paramsStart {
		cosignersOffset, params, err = parseParams(args[paramsStart:], true)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	cosignersStart := paramsStart + cosignersOffset
	if len(args) > cosignersStart {
		for i, c := range args[cosignersStart:] {
			cosigner, err := parseCosigner(c)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("failed to parse cosigner #%d: %w", i+1, err), 1)
			}
			cosigners = append(cosigners, cosigner)
		}
	}

	if signAndPush {
		gas = flags.Fixed8FromContext(ctx, "gas")
		acc, err = getAccFromContext(ctx)
		if err != nil {
			return err
		}
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	resp, err = c.InvokeFunction(script, operation, params, cosigners)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if signAndPush {
		if len(resp.Script) == 0 {
			return cli.NewExitError(errors.New("no script returned from the RPC node"), 1)
		}
		script, err := hex.DecodeString(resp.Script)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("bad script returned from the RPC node: %w", err), 1)
		}
		txHash, err := c.SignAndPushInvocationTx(script, acc, resp.GasConsumed, gas, cosigners)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to push invocation tx: %w", err), 1)
		}
		fmt.Printf("Sent invocation transaction %s\n", txHash.StringLE())
	} else {
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return cli.NewExitError(err, 1)
		}

		fmt.Println(string(b))
	}

	return nil
}

// parseParams extracts array of smartcontract.Parameter from the given args and
// returns the number of handled words, the array itself and an error.
// `calledFromMain` denotes whether the method was called from the outside or
// recursively and used to check if cosignersSeparator and closing bracket are
// allowed to be in `args` sequence.
func parseParams(args []string, calledFromMain bool) (int, []smartcontract.Parameter, error) {
	res := []smartcontract.Parameter{}
	for k := 0; k < len(args); {
		s := args[k]
		switch s {
		case cosignersSeparator:
			if calledFromMain {
				return k + 1, res, nil // `1` to convert index to numWordsRead
			}
			return 0, []smartcontract.Parameter{}, errors.New("invalid array syntax: missing closing bracket")
		case arrayStartSeparator:
			numWordsRead, array, err := parseParams(args[k+1:], false)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to parse array: %w", err)
			}
			res = append(res, smartcontract.Parameter{
				Type:  smartcontract.ArrayType,
				Value: array,
			})
			k += 1 + numWordsRead // `1` for opening bracket
		case arrayEndSeparator:
			if calledFromMain {
				return 0, nil, errors.New("invalid array syntax: missing opening bracket")
			}
			return k + 1, res, nil // `1`to convert index to numWordsRead
		default:
			param, err := smartcontract.NewParameterFromString(s)
			if err != nil {
				return 0, nil, fmt.Errorf("failed to parse argument #%d: %w", k+1, err)
			}
			res = append(res, *param)
			k++
		}
	}
	if calledFromMain {
		return len(args), res, nil
	}
	return 0, []smartcontract.Parameter{}, errors.New("invalid array syntax: missing closing bracket")

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

	args := ctx.Args()
	var signers []transaction.Signer
	if args.Present() {
		for i, c := range args[:] {
			cosigner, err := parseCosigner(c)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("failed to parse signer #%d: %w", i+1, err), 1)
			}
			signers = append(signers, cosigner)
		}
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

	fmt.Println(string(b))

	return nil
}

// ProjectConfig contains project metadata.
type ProjectConfig struct {
	HasStorage         bool
	IsPayable          bool
	SupportedStandards []string
	Events             []manifest.Event
}

// GetFeatures returns smartcontract features from the config.
func (p *ProjectConfig) GetFeatures() smartcontract.PropertyState {
	var fs smartcontract.PropertyState
	if p.IsPayable {
		fs |= smartcontract.IsPayable
	}
	if p.HasStorage {
		fs |= smartcontract.HasStorage
	}
	return fs
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
	v.PrintOps()

	return nil
}

func getAccFromContext(ctx *cli.Context) (*wallet.Account, error) {
	var addr util.Uint160

	wPath := ctx.String("wallet")
	if len(wPath) == 0 {
		return nil, cli.NewExitError(errNoWallet, 1)
	}

	wall, err := wallet.NewWalletFromFile(wPath)
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	addrFlag := ctx.Generic("address").(*flags.Address)
	if addrFlag.IsSet {
		addr = addrFlag.Uint160()
	} else {
		addr = wall.GetChangeAddress()
	}
	acc := wall.GetAccount(addr)
	if acc == nil {
		return nil, cli.NewExitError(fmt.Errorf("wallet contains no account for '%s'", address.Uint160ToString(addr)), 1)
	}

	fmt.Printf("Enter account %s password > ", address.Uint160ToString(addr))
	rawPass, err := terminal.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	pass := strings.TrimRight(string(rawPass), "\n")
	err = acc.Decrypt(pass)
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	return acc, nil
}

// contractDeploy deploys contract.
func contractDeploy(ctx *cli.Context) error {
	in := ctx.String("in")
	if len(in) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	manifestFile := ctx.String("manifest")
	if len(manifestFile) == 0 {
		return cli.NewExitError(errNoManifestFile, 1)
	}
	gas := flags.Fixed8FromContext(ctx, "gas")

	acc, err := getAccFromContext(ctx)
	if err != nil {
		return err
	}
	f, err := ioutil.ReadFile(in)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	nefFile, err := nef.FileFromBytes(f)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to restore .nef file: %w", err), 1)
	}

	manifestBytes, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to read manifest file: %w", err), 1)
	}
	m := &manifest.Manifest{}
	err = json.Unmarshal(manifestBytes, m)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to restore manifest file: %w", err), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return err
	}

	txScript, err := request.CreateDeploymentScript(nefFile.Script, m)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create deployment script: %w", err), 1)
	}
	// It doesn't require any signers.
	invRes, err := c.InvokeScript(txScript, nil)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to test-invoke deployment script: %w", err), 1)
	}

	txHash, err := c.SignAndPushInvocationTx(txScript, acc, invRes.GasConsumed, gas, nil)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to push invocation tx: %w", err), 1)
	}
	fmt.Printf("Sent deployment transaction %s for contract %s\n", txHash.StringLE(), nefFile.Header.ScriptHash.StringLE())
	return nil
}

func parseContractConfig(confFile string) (ProjectConfig, error) {
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

func parseCosigner(c string) (transaction.Signer, error) {
	var (
		err error
		res = transaction.Signer{
			Scopes: transaction.Global,
		}
	)
	data := strings.SplitN(c, ":", 2)
	s := data[0]
	if len(s) == 2*util.Uint160Size+2 && s[0:2] == "0x" {
		s = s[2:]
	}
	res.Account, err = util.Uint160DecodeStringLE(s)
	if err != nil {
		return res, err
	}
	if len(data) > 1 {
		res.Scopes, err = transaction.ScopesFromString(data[1])
		if err != nil {
			return transaction.Signer{}, err
		}
	}
	return res, nil
}
