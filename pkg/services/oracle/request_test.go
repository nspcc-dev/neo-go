package oracle

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckContentType(t *testing.T) {
	allowedTypes := []string{"application/json", "text/plain"}
	require.True(t, checkMediaType("application/json", allowedTypes))
	require.True(t, checkMediaType("application/json; param=value", allowedTypes))
	require.True(t, checkMediaType("text/plain; filename=file.txt", allowedTypes))

	require.False(t, checkMediaType("image/gif", allowedTypes))
	require.True(t, checkMediaType("image/gif", nil))

	require.False(t, checkMediaType("invalid format", allowedTypes))
}
