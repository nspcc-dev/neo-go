package storage

import (
	"testing"

	"github.com/alicebob/miniredis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisStore(t *testing.T) {
	redisMock, redisStore := prepareRedisMock(t)
	key := []byte("testKey")
	value := []byte("testValue")
	err := redisStore.Put(key, value)
	assert.Nil(t, err, "NewRedisStore Put error")

	result, err := redisStore.Get(key)
	assert.Nil(t, err, "NewRedisStore Get error")

	assert.Equal(t, value, result)
	require.NoError(t, redisStore.Close())
	redisMock.Close()
}

func TestRedisBatch_Len(t *testing.T) {
	want := len(map[string]string{})
	b := &MemoryBatch{
		m: map[*[]byte][]byte{},
	}
	assert.Equal(t, len(b.m), want)
}

func TestRedisStore_GetAndPut(t *testing.T) {
	prepareRedisMock(t)
	type args struct {
		k       []byte
		v       []byte
		kToLook []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{"TestRedisStore_Get_Strings",
			args{
				k:       []byte("foo"),
				v:       []byte("bar"),
				kToLook: []byte("foo"),
			},
			[]byte("bar"),
			false,
		},
		{"TestRedisStore_Get_Negative_Strings",
			args{
				k:       []byte("foo"),
				v:       []byte("bar"),
				kToLook: []byte("wrong"),
			},
			[]byte(nil),
			true,
		},
	}
	redisMock, redisStore := prepareRedisMock(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := redisStore.Put(tt.args.k, tt.args.v)
			assert.Nil(t, err, "Got error while Put operation processing")
			got, err := redisStore.Get(tt.args.kToLook)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
			redisMock.FlushDB()
		})
	}
	require.NoError(t, redisStore.Close())
	redisMock.Close()
}

func TestRedisStore_PutBatch(t *testing.T) {
	batch := &MemoryBatch{m: map[*[]byte][]byte{&[]byte{'f', 'o', 'o', '1'}: []byte("bar1")}}
	mock, redisStore := prepareRedisMock(t)
	err := redisStore.PutBatch(batch)
	assert.Nil(t, err, "Error while PutBatch")
	result, err := redisStore.Get([]byte("foo1"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("bar1"), result)
	require.NoError(t, redisStore.Close())
	mock.Close()
}

func TestRedisStore_Seek(t *testing.T) {
	mock, redisStore := prepareRedisMock(t)
	redisStore.Seek([]byte("foo"), func(k, v []byte) {
		assert.Equal(t, []byte("bar"), v)
	})
	require.NoError(t, redisStore.Close())
	mock.Close()
}

func prepareRedisMock(t *testing.T) (*miniredis.Miniredis, *RedisStore) {
	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Errorf("MiniRedis mock creation error = %v", err)
	}
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
	if err != nil {
		t.Errorf("NewRedisStore() error = %v", err)
		return nil, nil
	}
	return miniRedis, newRedisStore
}
