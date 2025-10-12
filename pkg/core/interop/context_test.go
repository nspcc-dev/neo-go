package interop

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
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

func TestContext_GetFunction(t *testing.T) {
	ic := &Context{
		Hardforks: map[string]uint32{config.HFFaun.String(): 42},
		Functions: []Function{
			{ID: interopnames.ToID([]byte(interopnames.SystemStorageLocalGet)), ActiveFrom: config.HFFaun},
		},
	}
	t.Run("GetLocal disabled", func(t *testing.T) {
		ic.Block = &block.Block{Header: block.Header{Index: 0}}
		require.Nil(t, ic.GetFunction(interopnames.ToID([]byte(interopnames.SystemStorageLocalGet))))
	})
	t.Run("GetLocal enabled", func(t *testing.T) {
		ic.Block = &block.Block{Header: block.Header{Index: 42}}
		require.NotNil(t, ic.GetFunction(interopnames.ToID([]byte(interopnames.SystemStorageLocalGet))))
	})
}
