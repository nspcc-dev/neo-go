package storage

type (
	// DBConfiguration describes configuration for DB. Supported: 'levelDB', 'boltDB', 'badgerDB'.
	DBConfiguration struct {
		Type            string          `yaml:"Type"`
		LevelDBOptions  LevelDBOptions  `yaml:"LevelDBOptions"`
		BoltDBOptions   BoltDBOptions   `yaml:"BoltDBOptions"`
		BadgerDBOptions BadgerDBOptions `yaml:"BadgerDBOptions"`
	}
)
