package storage

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/stretchr/testify/require"
)

func TestStorageNames(t *testing.T) {
	tmp := t.TempDir()
	cfg := dbconfig.DBConfiguration{
		LevelDBOptions: dbconfig.LevelDBOptions{
			DataDirectoryPath: filepath.Join(tmp, "level"),
		},
		BoltDBOptions: dbconfig.BoltDBOptions{
			FilePath: filepath.Join(tmp, "bolt"),
		},
	}
	for _, name := range []string{dbconfig.BoltDB, dbconfig.LevelDB, dbconfig.InMemoryDB} {
		t.Run(name, func(t *testing.T) {
			cfg.Type = name
			s, err := NewStore(cfg)
			require.NoError(t, err)
			require.NoError(t, s.Close())
		})
	}
}
