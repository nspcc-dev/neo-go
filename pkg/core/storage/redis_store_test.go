package storage

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

type mockedRedisStore struct {
	RedisStore
	mini *miniredis.Miniredis
}

func prepareRedisMock(t *testing.T) (*miniredis.Miniredis, *RedisStore) {
	miniRedis, err := miniredis.Run()
	require.Nil(t, err, "MiniRedis mock creation error")

	_ = miniRedis.Set("foo", "bar")

	dbConfig := DBConfiguration{
		Type: "redisDB",
		RedisDBOptions: RedisDBOptions{
			Addr:     miniRedis.Addr(),
			Password: "",
			DB:       0,
		},
	}
	newRedisStore, err := NewRedisStore(dbConfig.RedisDBOptions)
	require.Nil(t, err, "NewRedisStore() error")
	return miniRedis, newRedisStore
}

func (mrs *mockedRedisStore) Close() error {
	err := mrs.RedisStore.Close()
	mrs.mini.Close()
	return err
}

func newRedisStoreForTesting(t *testing.T) Store {
	mock, rs := prepareRedisMock(t)
	mrs := &mockedRedisStore{RedisStore: *rs, mini: mock}
	return mrs
}
