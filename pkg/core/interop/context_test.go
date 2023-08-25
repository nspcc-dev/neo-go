package interop

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/stretchr/testify/require"
)

func TestIsHardforkEnabled(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		ic := &Context{Hardforks: map[string]uint32{config.HFAspidochelone.String(): 0, config.HFBasilisk.String(): 0}, Block: &block.Block{Header: block.Header{Index: 10}}}
		require.True(t, ic.IsHardforkEnabled(config.HFAspidochelone))
		require.True(t, ic.IsHardforkEnabled(config.HFBasilisk))
	})
	t.Run("new disabled", func(t *testing.T) {
		ic := &Context{Hardforks: map[string]uint32{config.HFAspidochelone.String(): 5}, Block: &block.Block{Header: block.Header{Index: 10}}}
		require.True(t, ic.IsHardforkEnabled(config.HFAspidochelone))
		require.False(t, ic.IsHardforkEnabled(config.HFBasilisk))
	})
	t.Run("old enabled", func(t *testing.T) {
		ic := &Context{Hardforks: map[string]uint32{config.HFAspidochelone.String(): 0, config.HFBasilisk.String(): 10}, Block: &block.Block{Header: block.Header{Index: 5}}}
		require.True(t, ic.IsHardforkEnabled(config.HFAspidochelone))
		require.False(t, ic.IsHardforkEnabled(config.HFBasilisk))
	})
	t.Run("not yet enabled", func(t *testing.T) {
		ic := &Context{Hardforks: map[string]uint32{config.HFAspidochelone.String(): 10}, Block: &block.Block{Header: block.Header{Index: 5}}}
		require.False(t, ic.IsHardforkEnabled(config.HFAspidochelone))
	})
	t.Run("already enabled", func(t *testing.T) {
		ic := &Context{Hardforks: map[string]uint32{config.HFAspidochelone.String(): 10}, Block: &block.Block{Header: block.Header{Index: 15}}}
		require.True(t, ic.IsHardforkEnabled(config.HFAspidochelone))
	})
}
