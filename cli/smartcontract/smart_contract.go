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

	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/go-yaml/yaml"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	errNoEndpoint          = errors.New("no RPC endpoint specified, use option '--endpoint' or '-e'")
	errNoInput             = errors.New("no input file was found, specify an input file with the '--in or -i' flag")
	errNoSmartContractName = errors.New("no name was provided, specify the '--name or -n' flag")
	errFileExist           = errors.New("A file with given smart-contract name already exists")
)

var (
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
				Name:   "testinvoke",
				Usage:  "Test an invocation of a smart contract on the blockchain",
				Action: testInvoke,
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
	Contract ContractDetails `yaml:"project"`
}

// ContractDetails contains contract metadata.
type ContractDetails struct {
	Author               string
	Email                string
	Version              string
	ProjectName          string `yaml:"name"`
	Description          string
	HasStorage           bool
	HasDynamicInvocation bool
	IsPayable            bool
	ReturnType           rpc.StackParamType
	Parameters           []rpc.StackParamType
}

func parseContractDetails() ContractDetails {
	details := ContractDetails{}
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
