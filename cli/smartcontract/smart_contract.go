package smartcontract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/urfave/cli"
)

var (
	errNoInput             = errors.New("No input file was found, specify an input file with the '--in or -i' flag")
	errNoSmartContractName = errors.New("No name was provided, specify the '--name or -n' flag")
	errFileExist           = errors.New("A file with given smart-contract name already exists")
)

var (
	// smartContractTmpl is written to a file when used with `init` command.
	// %s is parsed to be the smartContractName
	smartContractTmpl = `package %s

import "github.com/CityOfZion/neo-storm/interop/runtime"

func Main(op string, args []interface{}) {
    runtime.Notify("Hello world!")
}`
)

// NewCommand returns a new contract command.
func NewCommand() cli.Command {
	return cli.Command{
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
						Name:  "in, i",
						Usage: "Input location of the avm file that needs to be invoked",
					},
				},
			},
			{
				Name:   "opdump",
				Usage:  "dump the opcode of a .go file",
				Action: contractDumpOpcode,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input file for the smart contract",
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
				},
			},
		},
	}
}

// initSmartContract initializes a given directory with some boiler plate code.
func initSmartContract(ctx *cli.Context) error {
	scName := ctx.String("name")
	if scName == "" {
		return cli.NewExitError(errNoSmartContractName, 1)
	}

	// Check if the file already exists, if yes, exit
	if _, err := os.Stat(scName); err == nil {
		return cli.NewExitError(errFileExist, 1)
	}

	basePath := scName
	fileName := "main.go"

	// create base directory
	err := os.Mkdir(basePath, os.ModePerm)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	data := []byte(fmt.Sprintf(smartContractTmpl, scName))
	err = ioutil.WriteFile(filepath.Join(basePath, fileName), data, 0644)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Printf("Successfully initialized smart contract [%s]\n", scName)
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

	b, err := ioutil.ReadFile(src)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	// For now we will hardcode the endpoint.
	// On the long term the internal VM will run the script.
	// TODO: remove RPC dependency, hardcoded node.
	endpoint := "http://seed5.bridgeprotocol.io:10332"
	opts := rpc.ClientOptions{}
	client, err := rpc.NewClient(context.TODO(), endpoint, opts)
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

func contractDumpOpcode(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}
	if err := compiler.DumpOpcode(src); err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}
