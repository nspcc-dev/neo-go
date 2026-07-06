package neorpc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventIDs_Compat(t *testing.T) {
	// Ensure there's no unknown event ID in EventIDs list.
	for _, id := range EventIDs {
		_, err := GetEventIDFromString(id.String())
		require.NoError(t, err)
	}

	// Ensure all necessary IDs are included in EventIDs list.
	m := make(map[EventID]struct{})
	for _, id := range EventIDs {
		_, ok := m[id]
		require.False(t, ok)
		m[id] = struct{}{}
	}
	for i := lastEventID; i < MissedEventID; i++ {
		_, ok := m[i]
		require.False(t, ok)
	}
	_, ok := m[InvalidEventID]
	require.False(t, ok)
}
