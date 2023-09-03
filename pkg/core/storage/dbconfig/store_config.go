/*
Package dbconfig is a micropackage that contains storage DB configuration options.
*/
package dbconfig

type (
	// DBConfiguration describes configuration for DB. Supported types:
	// [LevelDB], [BoltDB] or [InMemoryDB] (not recommended for production usage).
	DBConfiguration struct {
		Type           string         `yaml:"Type"`
		LevelDBOptions LevelDBOptions `yaml:"LevelDBOptions"`
		BoltDBOptions  BoltDBOptions  `yaml:"BoltDBOptions"`
	}
	// LevelDBOptions configuration for LevelDB.
	LevelDBOptions struct {
		DataDirectoryPath string `yaml:"DataDirectoryPath"`
		ReadOnly          bool   `yaml:"ReadOnly"`
	}
	// BoltDBOptions configuration for BoltDB.
	BoltDBOptions struct {
		FilePath string `yaml:"FilePath"`
		ReadOnly bool   `yaml:"ReadOnly"`
	}
)
