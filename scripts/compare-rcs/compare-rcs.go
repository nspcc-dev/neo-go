package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/urfave/cli/v2"
)

// Regexp to filter out System.Contract.CallNative SYSCALL and RET instruction.
var (
	regNativecall = regexp.MustCompile(`\[SYSCALL 1AF77B67 \d+\] `) // SYSCALL System.Contract.CallNative with RC value.
	regRet        = regexp.MustCompile(`\[RET  \d+] `)              // Just a RET opcode with RC value.
)

func main() {
	ctl := cli.NewApp()
	ctl.Name = "compare-rcs"
	ctl.Version = "1.0"
	ctl.Usage = "compare-rcs rcFileA rcFileB"
	ctl.Action = cliMain

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, ctl.Usage)
		os.Exit(1)
	}
}

func cliMain(c *cli.Context) error {
	a := c.Args().Get(0)
	b := c.Args().Get(1)
	if a == "" {
		return errors.New("no arguments given")
	}
	if b == "" {
		return errors.New("missing second argument")
	}
	fa, err := os.Open(a)
	if err != nil {
		return err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return err
	}
	defer fb.Close()

	return compare(fa, fb)
}

func compare(a, b *os.File) error {
	logFile, err := os.OpenFile("./diff.txt", os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	dumpA := bufio.NewReader(a)
	dumpB := bufio.NewReader(b)
	var (
		totalRecords    int
		mismatchRecords int
		mismatchHalted  int
		mismatchFaulted int
		mismatchCalls   int

		mismatchOther int
	)
	reportProgress := func() {
		mismatchPart := float32(mismatchRecords) / float32(totalRecords)
		haltedPart := float32(mismatchHalted) / float32(mismatchRecords)
		faultedPart := float32(mismatchFaulted) / float32(mismatchRecords)
		fmt.Printf("Total transactions count: %d\n", totalRecords)
		fmt.Printf("Mismatch records: %d (%.2f%%)\n", mismatchRecords, mismatchPart*100)
		fmt.Printf("\tHALTed: %d (%.2f%%)\n\tFAULTed: %d (%.2f%%)\n", mismatchHalted, haltedPart*100, mismatchFaulted, faultedPart*100)
		fmt.Printf("\tHALTed records with System.Contract.CallNative / opcode.RET mismatch: %d (%.2f%%)\n", mismatchCalls, float32(mismatchCalls)/float32(mismatchRecords)*100)
		fmt.Printf("\tOther: %d (%.2f%%)\n", mismatchOther, float32(mismatchOther)/float32(mismatchRecords)*100)
	}
	defer reportProgress()

parseLoop:
	for ; ; totalRecords++ {
		if totalRecords > 0 && totalRecords%500000 == 0 {
			reportProgress()
		}
		// Every transaction execution record contains two lines:
		//   - first line contains tx hash and VM state got after execution;
		//   - second line contains the set of instructions executed in form of: [OpCode Parameter RC] where:
		//  	- OpCode is the VM instruction,
		// 		- Parameter is upper-cased hex-encoded instruction parameter (optional, omitted if instruction doesn't have parameter),
		//		- RC is the reference counter value got after instruction execution.
		//
		// Example of a single execution record:
		// 	ff9ff8690f18bb12fe2704383b20bef9c2e1732dd71e355052f2f47905857417 HALT
		//	[PUSHNULL  1] [PUSHINT32 002F6859 2] [PUSHDATA1 7CA7001910CF59544929C3E6269B28E1C9AB6F12 3] [PUSHDATA1 6B123DD8BEC718648852BBC78595E3536A058F9F 4] [PUSH4  5] [PACK  5] [PUSH15  6] [PUSHDATA1 7472616E73666572 7] [PUSHDATA1 CF76E28BD0062C4A478EE35561011319F3CFA4D2 8] [SYSCALL 627D5B52 4] [PUSH0  5] [SYSCALL 1AF77B67 1] [RET  1] [ASSERT  0] [RET  0]

		// Parse txHash and VM state from file A.
		lA, err := dumpA.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading hash %s/%d: %w", a.Name(), totalRecords, err)
		}

		// Parse txHash and VM state from file B.
		lB, err := dumpB.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading hash %s/%d: %w", b.Name(), totalRecords, err)
		}

		// Retrieve txHash (possibly, 0x-prefixed) and VM state separated by a space.
		hA := strings.Split(strings.TrimPrefix(strings.TrimSuffix(lA, "\n"), "0x"), " ")
		hB := strings.Split(strings.TrimPrefix(strings.TrimSuffix(lB, "\n"), "0x"), " ")
		if len(hA) > 2 || len(hB) > 2 {
			return fmt.Errorf("unexpected output at %d: %s: %s vs %s: %s", totalRecords, a.Name(), string(lA), b.Name(), string(lB))
		}
		halt := (len(hA) == 2 && hA[1] == vmstate.Halt.String()) && (len(hB) == 2 && hB[1] == vmstate.Halt.String())
		if hA[0] != hB[0] {
			return fmt.Errorf("hash: %s vs %s", hA[0], hB[0])
		}

		// Parse instructions dump from file A.
		rcA, err := dumpA.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading RC %s/%s/%d: %w", a.Name(), hA[0], totalRecords, err)
		}

		// Parse instructions dump from file B.
		rcB, err := dumpB.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading RC %s/%s/%d: %w", b.Name(), hB[0], totalRecords, err)
		}

		// Compare raw instruction dumps.
		if string(rcA) != string(rcB) {
			mismatchRecords++
			if halt {
				mismatchHalted++
			} else {
				mismatchFaulted++
			}

			// Do not try to deal with faulted transactions, it's OK for RC to mismatch for them because
			// Go node has several optimisations of the way how refcounter is processed by some of the
			// instruction handlers.
			if !halt {
				continue parseLoop
			}

			// Filter out those executions that contain different RCs after System.Contract.CallNative SYSCALL and RET.
			// It's OK for them to mismatch due to a difference between System.Contract.CallNative processing in Go/C# nodes.
			rcAClear := regNativecall.ReplaceAllString(rcA, "")
			rcAClear = regRet.ReplaceAllString(rcAClear, "")
			rcBClear := regNativecall.ReplaceAllString(rcB, "")
			rcBClear = regRet.ReplaceAllString(rcBClear, "")
			if rcAClear == rcBClear {
				mismatchCalls++
				continue parseLoop
			}

			// If it's not the only difference, then log the transaction hash to investigate it more carefully.
			mismatchOther++
			_, _ = logFile.WriteString(fmt.Sprintf("%s\n", hA[0]))
		}
	}
}
