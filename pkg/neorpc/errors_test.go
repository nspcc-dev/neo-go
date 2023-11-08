package neorpc

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_ErrorsAs(t *testing.T) {
	err := NewInternalServerError("some error")
	wrapped := fmt.Errorf("some meaningful error: %w", err)

	// Check that Error can be used as a target for errors.As:
	var actual *Error
	require.True(t, errors.As(wrapped, &actual))
	require.Equal(t, "Internal error (-32603) - some error", actual.Error())

	var bad *fs.PathError
	require.False(t, errors.As(wrapped, &bad))
}

func TestError_ErrorsIs(t *testing.T) {
	err := NewInternalServerError("some error")
	wrapped := fmt.Errorf("some meaningful error: %w", err)

	// Check that Error can be recognized via errors.Is:
	ref := NewInternalServerError("another server error")
	require.True(t, errors.Is(wrapped, ref))

	// Invalid target type:
	require.False(t, errors.Is(wrapped, NewInvalidParamsError("invalid params")))
}
