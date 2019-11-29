package smartcontract

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	errNoEndpoint          = errors.New("no RPC endpoint specified, use option '--endpoint' or '-e'")
	errNoInput             = errors.New("no input file was found, specify an input file with the '--in or -i' flag")
	errNoConfFile          = errors.New("no config file was found, specify a config file with the '--config' or '-c' flag")
	errNoWIF               = errors.New("no WIF parameter found, specify it with the '--wif or -w' flag")
	errNoScriptHash        = errors.New("no smart contract hash was provided, specify one as the first argument")
	errNoSmartContractName = errors.New("no name was provided, specify the '--name or -n' flag")
	errFileExist           = errors.New("A file with given smart-contract name already exists")
)

const (
	// smartContractTmpl is written to a file when used with `init` command.
	// %s is parsed to be the smartContractName
	smartContractTmpl = `package %s

import "github.com/CityOfZion/neo-go/pkg/interop/runtime"

func Main(op string, args []interface{}) {
    runtime.Notify("Hello world!")
}`
)

// NewCommands returns 'contract' command.
func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:  "contract",
		Usage: "compile - debug - deploy smart contracts",
		Subcommands: []cli.Command{
			{
				Name:   "compile",
				Usage:  "compile a smart contract to a .avm file",
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
						Name:  "debug, d",
						Usage: "Debug mode will print out additional information after a compiling",
					},
				},
			},
			{
				Name:   "deploy",
				Usage:  "deploy a smart contract (.avm with description)",
				Action: contractDeploy,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input file for the smart contract (*.avm)",
					},
					cli.StringFlag{
						Name:  "config, c",
						Usage: "configuration input file (*.yml)",
					},
					cli.StringFlag{
						Name:  "endpoint, e",
						Usage: "RPC endpoint address (like 'http://seed4.ngd.network:20332')",
					},
					cli.StringFlag{
						Name:  "wif, w",
						Usage: "key to sign deployed transaction (in wif format)",
					},
					cli.IntFlag{
						Name:  "gas, g",
						Usage: "gas to pay for contract deployment",
					},
				},
			},
			{
				Name:      "testinvoke",
				Usage:     "invoke deployed contract on the blockchain (test mode)",
				UsageText: "neo-go contract testinvoke -e endpoint scripthash [arguments...]",
				Description: `Executes given (as a script hash) deployed script with the given arguments.
   It's very similar to the tesinvokefunction command, but differs in the way
   arguments are being passed. This invoker does not accept method parameter
   and it passes all given parameters as plain values to the contract, not
   wrapping them them into array like testinvokefunction does. For arguments
   syntax please refer to the testinvokefunction command help.

   Most of the time (if your contract follows the standard convention of
   method with array of values parameters) you want to use testinvokefunction
   command instead of testinvoke.
`,
				Action: testInvoke,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "endpoint, e",
						Usage: "RPC endpoint address (like 'http://seed4.ngd.network:20332')",
					},
				},
			},
			{
				Name:      "testinvokefunction",
				Usage:     "invoke deployed contract on the blockchain (test mode)",
				UsageText: "neo-go contract testinvokefunction -e endpoint scripthash [method] [arguments...]",
				Description: `Executes given (as a script hash) deployed script with the given method and
   arguments. If no method is given "" is passed to the script, if no arguments
   are given, an empty array is passed. All of the given arguments are
   encapsulated into array before invoking the script. The script thus should
   follow the regular convention of smart contract arguments (method string and
   an array of other arguments).

   Arguments always do have regular Neo smart contract parameter types, either
   specified explicitly or being inferred from the value. To specify the type
   manually use "type:value" syntax where the type is one of the following:
   'signature', 'bool', 'int', 'hash160', 'hash256', 'bytes', 'key' or 'string'.
   Array types are not currently supported.

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
`,
				Action: testInvokeFunction,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "endpoint, e",
						Usage: "RPC endpoint address (like 'http://seed4.ngd.network:20332')",
					},
				},
			},
			{
				Name:   "testinvokescript",
				Usage:  "Invoke compiled AVM code on the blockchain (test mode, not creating a transaction for it)",
				Action: testInvokeScript,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "endpoint, e",
						Usage: "RPC endpoint address (like 'http://seed4.ngd.network:20332')",
					},
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input location of the avm file that needs to be invoked",
					},
				},
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
						Usage: "input file of the program",
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

	// Ask contract information and write a neo-go.yml file unless the -skip-details flag is set.
	// TODO: Fix the missing neo-go.yml file with the `init` command when the package manager is in place.
	if !ctx.Bool("skip-details") {
		details := parseContractDetails()
		details.ReturnType = rpc.ByteArray
		details.Parameters = make([]rpc.StackParamType, 2)
		details.Parameters[0] = rpc.String
		details.Parameters[1] = rpc.Array

		project := &ProjectConfig{Contract: details}
		b, err := yaml.Marshal(project)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		if err := ioutil.WriteFile(filepath.Join(basePath, "neo-go.yml"), b, 0644); err != nil {
			return cli.NewExitError(err, 1)
		}
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

	o := &compiler.Options{
		Outfile: ctx.String("out"),
		Debug:   ctx.Bool("debug"),
	}

	if err := compiler.CompileAndSave(src, o); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func testInvoke(ctx *cli.Context) error {
	return testInvokeInternal(ctx, false)
}

func testInvokeFunction(ctx *cli.Context) error {
	return testInvokeInternal(ctx, true)
}

func testInvokeInternal(ctx *cli.Context, withMethod bool) error {
	var resp *rpc.InvokeScriptResponse
	var operation string
	var paramsStart = 1
	var params = make([]smartcontract.Parameter, 0)

	endpoint := ctx.String("endpoint")
	if len(endpoint) == 0 {
		return cli.NewExitError(errNoEndpoint, 1)
	}

	args := ctx.Args()
	if !args.Present() {
		return cli.NewExitError(errNoScriptHash, 1)
	}
	script := args[0]
	if withMethod && len(args) > 1 {
		operation = args[1]
		paramsStart++
	}
	if len(args) > paramsStart {
		for k, s := range args[paramsStart:] {
			param, err := smartcontract.NewParameterFromString(s)
			if err != nil {
				return cli.NewExitError(fmt.Errorf("failed to parse argument #%d: %v", k+paramsStart+1, err), 1)
			}
			params = append(params, *param)
		}
	}

	client, err := rpc.NewClient(context.TODO(), endpoint, rpc.ClientOptions{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if withMethod {
		resp, err = client.InvokeFunction(script, operation, params)
	} else {
		resp, err = client.Invoke(script, params)
	}
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	b, err := json.MarshalIndent(resp.Result, "", "  ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println(string(b))

	return nil
}

func testInvokeScript(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	endpoint := ctx.String("endpoint")
	if len(endpoint) == 0 {
		return cli.NewExitError(errNoEndpoint, 1)
	}

	b, err := ioutil.ReadFile(src)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	client, err := rpc.NewClient(context.TODO(), endpoint, rpc.ClientOptions{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	scriptHex := hex.EncodeToString(b)
	resp, err := client.InvokeScript(scriptHex)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	b, err = json.MarshalIndent(resp.Result, "", "  ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println(string(b))

	return nil
}

// ProjectConfig contains project metadata.
type ProjectConfig struct {
	Version  uint
	Contract rpc.ContractDetails `yaml:"project"`
}

func parseContractDetails() rpc.ContractDetails {
	details := rpc.ContractDetails{}
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Author: ")
	details.Author, _ = reader.ReadString('\n')

	fmt.Print("Email: ")
	details.Email, _ = reader.ReadString('\n')

	fmt.Print("Version: ")
	details.Version, _ = reader.ReadString('\n')

	fmt.Print("Project name: ")
	details.ProjectName, _ = reader.ReadString('\n')

	fmt.Print("Description: ")
	details.Description, _ = reader.ReadString('\n')

	return details
}

func inspect(ctx *cli.Context) error {
	in := ctx.String("in")
	compile := ctx.Bool("compile")
	if len(in) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	b, err := ioutil.ReadFile(in)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if compile {
		b, err = compiler.Compile(bytes.NewReader(b))
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "failed to compile"), 1)
		}
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps()

	return nil
}

// contractDeploy deploys contract.
func contractDeploy(ctx *cli.Context) error {
	in := ctx.String("in")
	if len(in) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	confFile := ctx.String("config")
	if len(confFile) == 0 {
		return cli.NewExitError(errNoConfFile, 1)
	}
	endpoint := ctx.String("endpoint")
	if len(endpoint) == 0 {
		return cli.NewExitError(errNoEndpoint, 1)
	}
	wifStr := ctx.String("wif")
	if len(wifStr) == 0 {
		return cli.NewExitError(errNoWIF, 1)
	}
	gas := util.Fixed8FromInt64(int64(ctx.Int("gas")))

	wif, err := keys.WIFDecode(wifStr, 0)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("bad wif: %v", err), 1)
	}
	avm, err := ioutil.ReadFile(in)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	confBytes, err := ioutil.ReadFile(confFile)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	conf := ProjectConfig{}
	err = yaml.Unmarshal(confBytes, &conf)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("bad config: %v", err), 1)
	}

	client, err := rpc.NewClient(context.TODO(), endpoint, rpc.ClientOptions{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	txScript, err := rpc.CreateDeploymentScript(avm, &conf.Contract)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create deployment script: %v", err), 1)
	}

	txHash, err := client.SignAndPushInvocationTx(txScript, wif, gas)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to push invocation tx: %v", err), 1)
	}
	fmt.Printf("Sent deployment transaction %s for contract %s\n", txHash.ReverseString(), hash.Hash160(avm).ReverseString())
	return nil
}
