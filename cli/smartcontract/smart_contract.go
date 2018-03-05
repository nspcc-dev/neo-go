package smartcontract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/urfave/cli"
)

const (
	errNoInput = "Input file is mandatory and should be passed using -i flag."
)

// NewCommand returns a new contract command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:  "contract",
		Usage: "compile - debug - deploy smart contracts",
		Subcommands: []cli.Command{

			// Compile a smart contract.
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
				},
			},

			// Testinvoke a smart contract.
			{
				Name:   "invoke",
				Usage:  "Test an invocation of a smart contract on the blockchain",
				Action: testInvoke,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input location of the avm file that needs to be invoked",
					},
					cli.StringFlag{
						Name:  "operation, o",
						Usage: "Operation (method) that needs to be invoked",
					},
					cli.StringSliceFlag{
						Name:  "param, p",
						Usage: "Parameter used when invoking the contract",
					},
				},
			},

			// Dump opcode of a smart contract.
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
		},
	}
}

func contractCompile(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(errNoInput, 1)
	}

	o := &compiler.Options{
		Outfile: ctx.String("out"),
		Debug:   true,
	}

	if err := compiler.CompileAndSave(src, o); err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}

func testInvoke(ctx *cli.Context) error {
	var (
		src       = ctx.String("in")
		operation = ctx.String("operation")
		params    = ctx.StringSlice("param")
	)

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

	var (
		scriptHex = hex.EncodeToString(b)
		result    interface{}
	)

	if len(operation) == 0 {
		resp, err := client.InvokeScript(scriptHex)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		result = resp.Result
	} else if len(operation) > 0 && len(params) > 0 {
		parameters, err := parseParamsToParameters(params)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		resp, err := client.InvokeFunction(scriptHex, operation, parameters)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		if resp.Result == nil {
			return cli.NewExitError(errors.New("invoke failed, thats all we know :("), 1)
		}
	}

	b, err = json.MarshalIndent(result, "", "  ")
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println(string(b))

	return nil
}

func parseParamsToParameters(params []string) ([]smartcontract.Parameter, error) {
	parameters := make([]smartcontract.Parameter, len(params))
	for i, param := range params {
		parts := strings.Split(param, ":")
		if len(parts) != 2 {
			return nil, errors.New("failed to parse parameter")
		}

		var (
			t   = parts[0]
			v   = parts[1]
			p   smartcontract.Parameter
			err error
		)

		switch t {
		case "string":
			p.Type = smartcontract.StringType
			p.Value = v
		case "int":
			p.Type = smartcontract.IntegerType
			p.Value, err = strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
		}
		parameters[i] = p
	}
	return parameters, nil
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
