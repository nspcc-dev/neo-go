package neorpc_test

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestFaultException_ErrorsAs(t *testing.T) {
	err := neorpc.GASLimitExceededException
	wrapped := fmt.Errorf("%w executing RET", err)

	// Check that FaultException can be used as a target for errors.As:
	var actual *neorpc.FaultException
	require.True(t, errors.As(wrapped, &actual))
	require.Equal(t, "GAS limit exceeded", actual.Error())

	var bad *fs.PathError
	require.False(t, errors.As(wrapped, &bad))
}

func TestFaultException_ErrorsIs(t *testing.T) {
	err := &neorpc.FaultException{Message: "GAS limit exceeded executing System.Contract.Call"}

	// Check that a specific FaultException can be recognized via errors.Is:
	ref := neorpc.GASLimitExceededException
	require.True(t, errors.Is(err, ref))

	// Target exception message mismatch.
	require.False(t, errors.Is(err, &neorpc.FaultException{Message: "some error"}))
}

func TestGASLimitExceededException_MessageCompat(t *testing.T) {
	require.Equal(t, vm.ErrGASLimitExceeded.Error(), neorpc.GASLimitExceededException.Error())
}
